package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/time/rate"
)

// stubNominatim returns a handler backed by a test server serving the given
// raw JSON body, with no rate limit so tests run fast.
func stubNominatim(t *testing.T, status int, body string) *GeocodeHandler {
	t.Helper()
	h, _ := countingNominatim(t, status, body, rate.NewLimiter(rate.Inf, 1))
	return h
}

// countingNominatim is stubNominatim with an injectable limiter and a counter
// of upstream hits.
func countingNominatim(t *testing.T, status int, body string, limiter *rate.Limiter) (*GeocodeHandler, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &GeocodeHandler{
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		limiter:    limiter,
		maxWait:    time.Minute,
		cache:      newGeocodeCache(time.Hour, 10, time.Now),
	}, &hits
}

const splitBody = `[{"name":"Split","display_name":"Split, Croatia","lat":"43.5081","lon":"16.4402","category":"place"}]`

func TestGeocode_Search(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[
		{"name":"Split","display_name":"Split, Croatia","lat":"43.5081","lon":"16.4402","category":"place"},
		{"name":"","display_name":"Hvar, Croatia","lat":"43.1729","lon":"16.4412","category":"boundary"},
		{"name":"Bad","display_name":"Bad","lat":"not-a-number","lon":"0","category":"place"}
	]`)
	out, err := h.search(context.Background(), &geocodeInput{Query: "Split"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// The third entry has an unparseable latitude and is dropped.
	if len(out.Body) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(out.Body), out.Body)
	}
	if out.Body[0].Name != "Split" || out.Body[0].Latitude != 43.5081 {
		t.Fatalf("unexpected first result: %+v", out.Body[0])
	}
	if out.Body[0].Label != "Split, Croatia" {
		t.Fatalf("label not populated: %+v", out.Body[0])
	}
	// Empty name falls back to display_name.
	if out.Body[1].Name != "Hvar, Croatia" {
		t.Fatalf("name fallback failed: %+v", out.Body[1])
	}
}

// TestGeocode_FiltersAndDedups checks that non-place categories (streets, POIs)
// are dropped and that a town listed as both a place node and an admin
// boundary collapses to one result.
func TestGeocode_FiltersAndDedups(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[
		{"name":"Gdańsk","display_name":"Gdańsk, Poland","lat":"54.3520","lon":"18.6466","category":"place"},
		{"name":"Gdańsk","display_name":"Gdańsk, Pomerania, Poland","lat":"54.3521","lon":"18.6467","category":"boundary"},
		{"name":"Gdańska","display_name":"Gdańska street","lat":"52.2300","lon":"21.0100","category":"highway"},
		{"name":"Gdansk","display_name":"Gdansk, North Dakota, USA","lat":"48.4100","lon":"-98.3700","category":"place"}
	]`)
	out, err := h.search(context.Background(), &geocodeInput{Query: "Gdansk"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// The street is filtered out; the duplicate Gdańsk coordinates collapse.
	if len(out.Body) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(out.Body), out.Body)
	}
	if out.Body[1].Label != "Gdansk, North Dakota, USA" {
		t.Fatalf("unexpected second result: %+v", out.Body[1])
	}
}

// TestGeocode_ReadsJSONv2Category feeds full Nominatim jsonv2 records, which
// carry the feature class as "category" and no "class" key, and checks the
// handler asks for jsonv2 and keeps places and boundaries but drops streets.
func TestGeocode_ReadsJSONv2Category(t *testing.T) {
	var format string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		format = r.URL.Query().Get("format")
		_, _ = w.Write([]byte(`[
			{"place_id":1,"licence":"ODbL","osm_type":"relation","osm_id":1597935,"lat":"54.3482907","lon":"18.6540233","category":"boundary","type":"administrative","place_rank":16,"importance":0.71,"addresstype":"city","name":"Gdańsk","display_name":"Gdańsk, Pomeranian Voivodeship, Poland","boundingbox":["54.27","54.45","18.43","18.95"]},
			{"place_id":2,"licence":"ODbL","osm_type":"node","osm_id":31560326,"lat":"44.1194","lon":"15.2314","category":"place","type":"city","place_rank":16,"importance":0.6,"addresstype":"city","name":"Zadar","display_name":"Zadar, Croatia","boundingbox":["43.9","44.3","15.0","15.4"]},
			{"place_id":3,"licence":"ODbL","osm_type":"way","osm_id":4229281,"lat":"52.2300","lon":"21.0100","category":"highway","type":"residential","place_rank":26,"importance":0.1,"addresstype":"road","name":"Gdańska","display_name":"Gdańska, Warsaw, Poland","boundingbox":["52.22","52.24","21.00","21.02"]}
		]`))
	}))
	t.Cleanup(srv.Close)
	h := &GeocodeHandler{
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		limiter:    rate.NewLimiter(rate.Inf, 1),
		maxWait:    time.Minute,
		cache:      newGeocodeCache(time.Hour, 10, time.Now),
	}

	out, err := h.search(context.Background(), &geocodeInput{Query: "Gdańsk"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if format != "jsonv2" {
		t.Fatalf("upstream format = %q, want jsonv2", format)
	}
	var names []string
	for _, r := range out.Body {
		names = append(names, r.Name)
	}
	if len(names) != 2 || names[0] != "Gdańsk" || names[1] != "Zadar" {
		t.Fatalf("got %v, want [Gdańsk Zadar]", names)
	}
}

func TestGeocode_UpstreamError(t *testing.T) {
	h := stubNominatim(t, http.StatusInternalServerError, "boom")
	if _, err := h.search(context.Background(), &geocodeInput{Query: "Split"}); err == nil {
		t.Fatal("expected error on upstream 500")
	}
}

func TestGeocode_NoResults(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[]`)
	out, err := h.search(context.Background(), &geocodeInput{Query: "zzzzz"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out.Body) != 0 {
		t.Fatalf("got %d results, want 0", len(out.Body))
	}
}

// TestGeocode_CachesNormalizedQuery checks that repeat lookups differing only
// in case and surrounding whitespace reach the upstream once.
func TestGeocode_CachesNormalizedQuery(t *testing.T) {
	h, hits := countingNominatim(t, http.StatusOK, splitBody, rate.NewLimiter(rate.Inf, 1))
	for _, q := range []string{"Split", "  split ", "SPLIT"} {
		out, err := h.search(context.Background(), &geocodeInput{Query: q})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if len(out.Body) != 1 || out.Body[0].Name != "Split" {
			t.Fatalf("search %q: unexpected results %+v", q, out.Body)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

// TestGeocode_CoalescesConcurrentIdenticalQueries checks that identical
// lookups arriving while the first is still upstream share its result even
// when the upstream call outlasts the limiter interval.
func TestGeocode_CoalescesConcurrentIdenticalQueries(t *testing.T) {
	var hits atomic.Int64
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		<-release
		_, _ = w.Write([]byte(splitBody))
	}))
	t.Cleanup(srv.Close)
	h := &GeocodeHandler{
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		limiter:    rate.NewLimiter(rate.Every(10*time.Millisecond), 1),
		maxWait:    time.Minute,
		cache:      newGeocodeCache(time.Hour, 10, time.Now),
	}

	queries := []string{"Split", "split ", " SPLIT"}
	errs := make(chan error, len(queries))
	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Go(func() {
			out, err := h.search(context.Background(), &geocodeInput{Query: q})
			if err == nil && (len(out.Body) != 1 || out.Body[0].Name != "Split") {
				err = errors.New("unexpected results")
			}
			errs <- err
		})
	}
	// Hold the first upstream call well past the limiter interval so a
	// follower that only re-checked the cache would fetch again.
	time.Sleep(200 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("search: %v", err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

// TestGeocode_LookupSurvivesCallerCancel checks that a cancelled request
// does not fail other callers sharing its lookup.
func TestGeocode_LookupSurvivesCallerCancel(t *testing.T) {
	h, hits := countingNominatim(t, http.StatusOK, splitBody, rate.NewLimiter(rate.Inf, 1))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.search(ctx, &geocodeInput{Query: "Split"}); err != nil {
		t.Fatalf("search with cancelled caller: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

// TestGeocode_CachesEmptyResults checks that a query with no matches is not
// re-sent upstream either.
func TestGeocode_CachesEmptyResults(t *testing.T) {
	h, hits := countingNominatim(t, http.StatusOK, `[]`, rate.NewLimiter(rate.Inf, 1))
	for range 2 {
		if _, err := h.search(context.Background(), &geocodeInput{Query: "zzzzz"}); err != nil {
			t.Fatalf("search: %v", err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

func TestGeocode_DoesNotCacheErrors(t *testing.T) {
	h, hits := countingNominatim(t, http.StatusInternalServerError, "boom", rate.NewLimiter(rate.Inf, 1))
	for range 2 {
		if _, err := h.search(context.Background(), &geocodeInput{Query: "Split"}); err == nil {
			t.Fatal("expected error on upstream 500")
		}
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("upstream hits = %d, want 2", got)
	}
}

// TestGeocode_RateLimitsUpstream fires distinct queries at once and checks
// the upstream sees them no faster than the limiter interval.
func TestGeocode_RateLimitsUpstream(t *testing.T) {
	const interval = 100 * time.Millisecond
	var mu sync.Mutex
	var times []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	h := &GeocodeHandler{
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		limiter:    rate.NewLimiter(rate.Every(interval), 1),
		maxWait:    time.Minute,
		cache:      newGeocodeCache(time.Hour, 10, time.Now),
	}

	queries := []string{"Split", "Hvar", "Vis", "Korcula"}
	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Go(func() {
			if _, err := h.search(context.Background(), &geocodeInput{Query: q}); err != nil {
				t.Errorf("search %q: %v", q, err)
			}
		})
	}
	wg.Wait()

	if len(times) != len(queries) {
		t.Fatalf("upstream hits = %d, want %d", len(times), len(queries))
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	// Allow scheduler jitter; without the limiter the gaps are ~0.
	const minGap = interval * 8 / 10
	for i := 1; i < len(times); i++ {
		if gap := times[i].Sub(times[i-1]); gap < minGap {
			t.Fatalf("hits %d and %d were %v apart, want >= %v", i-1, i, gap, minGap)
		}
	}
}

// TestGeocode_BusyWhenLimiterQueueTooLong checks that a request that would
// wait longer than maxWait fails with 503 and never reaches the upstream.
func TestGeocode_BusyWhenLimiterQueueTooLong(t *testing.T) {
	h, hits := countingNominatim(t, http.StatusOK, splitBody, rate.NewLimiter(rate.Every(time.Hour), 1))
	h.maxWait = 10 * time.Millisecond
	if _, err := h.search(context.Background(), &geocodeInput{Query: "Split"}); err != nil {
		t.Fatalf("first search: %v", err)
	}
	_, err := h.search(context.Background(), &geocodeInput{Query: "Hvar"})
	var se huma.StatusError
	if !errors.As(err, &se) || se.GetStatus() != http.StatusServiceUnavailable {
		t.Fatalf("got %v, want 503", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
	// A cached query still answers while the limiter is exhausted.
	if _, err := h.search(context.Background(), &geocodeInput{Query: "split"}); err != nil {
		t.Fatalf("cached search: %v", err)
	}
}

func TestGeocodeCache_ExpiresAfterTTL(t *testing.T) {
	now := time.Unix(0, 0)
	c := newGeocodeCache(time.Hour, 10, func() time.Time { return now })
	c.put("split", nil)
	now = now.Add(59 * time.Minute)
	if _, ok := c.get("split"); !ok {
		t.Fatal("entry expired before TTL")
	}
	now = now.Add(time.Minute)
	if _, ok := c.get("split"); ok {
		t.Fatal("entry served after TTL")
	}
}

func TestGeocodeCache_EvictsAtLimit(t *testing.T) {
	now := time.Unix(0, 0)
	c := newGeocodeCache(time.Hour, 2, func() time.Time { return now })
	c.put("a", nil)
	now = now.Add(time.Second)
	c.put("b", nil)
	now = now.Add(time.Second)
	c.put("c", nil)
	if _, ok := c.get("a"); ok {
		t.Fatal("oldest entry kept past the limit")
	}
	for _, k := range []string{"b", "c"} {
		if _, ok := c.get(k); !ok {
			t.Fatalf("entry %q evicted, want kept", k)
		}
	}
	if n := len(c.entries); n != 2 {
		t.Fatalf("cache size = %d, want 2", n)
	}
}

func TestGeocodeCache_EvictsExpiredBeforeLive(t *testing.T) {
	now := time.Unix(0, 0)
	c := newGeocodeCache(time.Hour, 2, func() time.Time { return now })
	c.put("old", nil)
	now = now.Add(30 * time.Minute)
	c.put("live", nil)
	now = now.Add(45 * time.Minute) // "old" is now expired, "live" is not
	c.put("new", nil)
	if _, ok := c.get("live"); !ok {
		t.Fatal("live entry evicted while an expired one existed")
	}
	if _, ok := c.entries["old"]; ok {
		t.Fatal("expired entry kept")
	}
}

func TestNewGeocodeHandler_PolicyLimits(t *testing.T) {
	h := NewGeocodeHandler()
	if h.limiter.Limit() != rate.Every(time.Second) || h.limiter.Burst() != 1 {
		t.Fatalf("limiter = %v/%d, want 1 req/s burst 1", h.limiter.Limit(), h.limiter.Burst())
	}
	if h.cache.ttl != 24*time.Hour || h.cache.limit != 1000 {
		t.Fatalf("cache = ttl %v limit %d, want 24h/1000", h.cache.ttl, h.cache.limit)
	}
}
