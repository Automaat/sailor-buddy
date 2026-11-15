package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"golang.org/x/time/rate"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
)

// stubUpstream is an httptest server standing in for Nominatim or Photon. It
// answers every request with status and body and records each request's
// query and User-Agent.
type stubUpstream struct {
	srv    *httptest.Server
	hits   atomic.Int64
	mu     sync.Mutex
	params []url.Values
	agents []string
}

func newStubUpstream(t *testing.T, status int, body string) *stubUpstream {
	t.Helper()
	s := &stubUpstream{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		s.mu.Lock()
		s.params = append(s.params, r.URL.Query())
		s.agents = append(s.agents, r.UserAgent())
		s.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stubUpstream) lastParams(t *testing.T) url.Values {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.params) == 0 {
		t.Fatal("upstream not called")
	}
	return s.params[len(s.params)-1]
}

// testGeocoder wires a handler to the stub upstreams with no rate limit. A
// nil photon turns the fallback off.
func testGeocoder(nominatim, photon *stubUpstream) *GeocodeHandler {
	h := &GeocodeHandler{
		nominatim:  geocodeUpstream{name: geocodeSourceNominatim, baseURL: nominatim.srv.URL, limiter: rate.NewLimiter(rate.Inf, 1)},
		photon:     geocodeUpstream{name: geocodeSourcePhoton, limiter: rate.NewLimiter(rate.Inf, 1)},
		httpClient: nominatim.srv.Client(),
		maxWait:    time.Minute,
		cache:      newGeocodeCache(time.Hour, 10, time.Now),
	}
	if photon != nil {
		h.photon.baseURL = photon.srv.URL
	}
	return h
}

// stubNominatim returns a handler without the Photon fallback, backed by a
// Nominatim stub serving body.
func stubNominatim(t *testing.T, status int, body string) *GeocodeHandler {
	t.Helper()
	return testGeocoder(newStubUpstream(t, status, body), nil)
}

const splitBody = `[{"name":"Split","display_name":"Split, Croatia","lat":"43.5081","lon":"16.4402","category":"place","type":"city",
	"address":{"county":"Split-Dalmatia County","country":"Chorwacja"}}]`

const photonSplitBody = `{"type":"FeatureCollection","features":[
	{"type":"Feature","geometry":{"type":"Point","coordinates":[16.4402,43.5081]},
	 "properties":{"name":"Split","osm_key":"place","osm_value":"city","county":"Split-Dalmatia County","country":"Croatia"}}]}`

func names(rs []dto.GeocodeResult) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

func wantStatus(t *testing.T, err error, status int) {
	t.Helper()
	var se huma.StatusError
	if !errors.As(err, &se) || se.GetStatus() != status {
		t.Fatalf("got %v, want %d", err, status)
	}
}

func TestGeocode_Search(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[
		{"name":"Lefkada","display_name":"Lefkada, Lefkada Regional Unit, Wyspy Jońskie, Grecja","lat":"38.83","lon":"20.70","category":"place","type":"town",
		 "address":{"town":"Lefkada","county":"Lefkada Regional Unit","state":"Wyspy Jońskie","country":"Grecja"}},
		{"name":"Gdynia","display_name":"Gdynia, województwo pomorskie, Polska","lat":"54.51","lon":"18.53","category":"boundary","type":"administrative","addresstype":"city",
		 "address":{"city":"Gdynia","county":"Gdynia","state":"województwo pomorskie","country":"Polska"}},
		{"name":"","display_name":"Hvar, Croatia","lat":"43.17","lon":"16.44","category":"place","type":"town"},
		{"name":"Bad","display_name":"Bad","lat":"not-a-number","lon":"0","category":"place","type":"town"}
	]`)
	out, err := h.search(context.Background(), &geocodeInput{Query: "Lefkada"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	want := []dto.GeocodeResult{
		{Name: "Lefkada", Label: "Lefkada, Lefkada Regional Unit, Wyspy Jońskie, Grecja", Latitude: 38.83, Longitude: 20.70, Source: "nominatim"},
		{Name: "Gdynia", Label: "Gdynia, województwo pomorskie, Polska", Latitude: 54.51, Longitude: 18.53, Source: "nominatim"},
		{Name: "Hvar, Croatia", Label: "Hvar, Croatia", Latitude: 43.17, Longitude: 16.44, Source: "nominatim"},
	}
	if !reflect.DeepEqual(out.Body, want) {
		t.Fatalf("results = %+v, want %+v", out.Body, want)
	}
}

// TestGeocode_KeepsOnlySettlements checks the town filter: place records of
// settlement types and administrative boundaries around a settlement stay;
// everything else, from streets to regions and bays, goes.
func TestGeocode_KeepsOnlySettlements(t *testing.T) {
	tests := []struct {
		category, typ, addressType string
		keep                       bool
	}{
		{"place", "city", "city", true},
		{"place", "town", "town", true},
		{"place", "village", "village", true},
		{"place", "hamlet", "hamlet", true},
		{"place", "suburb", "suburb", true},
		{"place", "island", "island", true},
		{"place", "islet", "islet", true},
		{"place", "locality", "locality", true},
		{"place", "municipality", "municipality", true},
		{"boundary", "administrative", "city", true},
		{"boundary", "administrative", "town", true},
		{"boundary", "administrative", "village", true},
		{"boundary", "administrative", "municipality", true},
		{"boundary", "administrative", "state", false},
		{"boundary", "administrative", "county", false},
		{"boundary", "administrative", "country", false},
		{"boundary", "national_park", "city", false},
		{"place", "state", "state", false},
		{"place", "region", "region", false},
		{"place", "", "", false},
		{"natural", "bay", "bay", false},
		{"leisure", "marina", "leisure", false},
		{"highway", "residential", "road", false},
		{"amenity", "restaurant", "amenity", false},
		{"tourism", "hotel", "tourism", false},
	}
	for _, tt := range tests {
		t.Run(tt.category+"/"+tt.typ+"/"+tt.addressType, func(t *testing.T) {
			body, err := json.Marshal([]map[string]any{{
				"name": "Hel", "display_name": "Hel, Polska", "lat": "54.6", "lon": "18.8",
				"category": tt.category, "type": tt.typ, "addresstype": tt.addressType,
			}})
			if err != nil {
				t.Fatal(err)
			}
			h := stubNominatim(t, http.StatusOK, string(body))
			out, err := h.search(context.Background(), &geocodeInput{Query: "Hel"})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if kept := len(out.Body) == 1; kept != tt.keep {
				t.Fatalf("kept = %v, want %v", kept, tt.keep)
			}
		})
	}
}

// TestGeocode_Dedups checks that one town listed several times collapses to
// its best-ranked record, while distinct towns of one name stay.
func TestGeocode_Dedups(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "identical labels far apart",
			body: `[
				{"name":"Szczecin","display_name":"Szczecin, województwo zachodniopomorskie, Polska","lat":"53.4300","lon":"14.5500","category":"boundary","type":"administrative","addresstype":"city","address":{"state":"województwo zachodniopomorskie","country":"Polska"}},
				{"name":"Szczecin","display_name":"Szczecin, województwo zachodniopomorskie, Polska","lat":"53.3800","lon":"14.6600","category":"place","type":"city","address":{"state":"województwo zachodniopomorskie","country":"Polska"}}
			]`,
			want: []string{"Szczecin, województwo zachodniopomorskie, Polska"},
		},
		{
			name: "labels differing in case and accents",
			body: `[
				{"name":"Świnoujście","display_name":"Świnoujście, Polska","lat":"53.91","lon":"14.25","category":"place","type":"town","address":{"country":"Polska"}},
				{"name":"SWINOUJSCIE","display_name":"SWINOUJSCIE, POLSKA","lat":"53.70","lon":"14.60","category":"place","type":"town","address":{"country":"POLSKA"}}
			]`,
			want: []string{"Świnoujście, Polska"},
		},
		{
			name: "same name within 5 km",
			body: `[
				{"name":"Gdańsk","display_name":"Gdańsk, województwo pomorskie, Polska","lat":"54.3480","lon":"18.6540","category":"boundary","type":"administrative","addresstype":"city","address":{"state":"województwo pomorskie","country":"Polska"}},
				{"name":"Gdańsk","display_name":"Gdańsk, powiat gdański, Polska","lat":"54.3720","lon":"18.6380","category":"place","type":"city","address":{"county":"powiat gdański","country":"Polska"}}
			]`,
			want: []string{"Gdańsk, województwo pomorskie, Polska"},
		},
		{
			name: "same name far apart",
			body: `[
				{"name":"Gdańsk","display_name":"Gdańsk, Polska","lat":"54.35","lon":"18.65","category":"place","type":"city","address":{"country":"Polska"}},
				{"name":"Gdansk","display_name":"Gdansk, North Dakota, USA","lat":"48.41","lon":"-98.37","category":"place","type":"village","address":{"state":"North Dakota","country":"USA"}}
			]`,
			want: []string{"Gdańsk, Polska", "Gdansk, North Dakota, USA"},
		},
		{
			name: "different names close together",
			body: `[
				{"name":"Hel","display_name":"Hel, Polska","lat":"54.6080","lon":"18.8010","category":"place","type":"town","address":{"country":"Polska"}},
				{"name":"Jurata","display_name":"Jurata, Polska","lat":"54.6300","lon":"18.7400","category":"place","type":"village","address":{"country":"Polska"}}
			]`,
			want: []string{"Hel, Polska", "Jurata, Polska"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := stubNominatim(t, http.StatusOK, tt.body)
			out, err := h.search(context.Background(), &geocodeInput{Query: "x" + tt.name})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			var labels []string
			for _, r := range out.Body {
				labels = append(labels, r.Label)
			}
			if !reflect.DeepEqual(labels, tt.want) {
				t.Fatalf("labels = %q, want %q", labels, tt.want)
			}
		})
	}
}

func TestGeocode_CapsResults(t *testing.T) {
	var records []map[string]any
	for i := range 8 {
		records = append(records, map[string]any{
			"name": "Town", "display_name": "Town " + strconv.Itoa(i), "lat": "10", "lon": strconv.Itoa(i * 10),
			"category": "place", "type": "town",
		})
	}
	body, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	out, err := stubNominatim(t, http.StatusOK, string(body)).search(context.Background(), &geocodeInput{Query: "Town"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out.Body) != geocodeMaxResults {
		t.Fatalf("got %d results, want %d", len(out.Body), geocodeMaxResults)
	}
}

// TestGeocode_NominatimRequest checks the request shape: jsonv2 with address
// details, names in Polish then English, a descriptive User-Agent, and a
// viewbox around the rounded map centre only when the caller sends one.
func TestGeocode_NominatimRequest(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantViewbox string
	}{
		{"no bias", "/geocode?q=Hel", ""},
		{"bias rounded to the grid", "/geocode?q=Hel&lat=54.37&lon=18.62", "15.5,56.5,21.5,52.5"},
		{"viewbox clamped", "/geocode?q=Hel&lat=89.9&lon=179.9", "177,90,180,88"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nominatim := newStubUpstream(t, http.StatusOK, splitBody)
			_, api := humatest.New(t)
			registerGeocode(api, testGeocoder(nominatim, nil))
			if resp := api.Get(tt.path); resp.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", resp.Code, resp.Body.String())
			}
			p := nominatim.lastParams(t)
			want := map[string]string{
				"q": "Hel", "format": "jsonv2", "addressdetails": "1", "limit": "15",
				"accept-language": "pl,en", "viewbox": tt.wantViewbox, "bounded": "",
			}
			if tt.wantViewbox != "" {
				want["bounded"] = "0"
			}
			for k, v := range want {
				if got := p.Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
			if ua := nominatim.agents[0]; ua != geocodeUserAgent {
				t.Errorf("User-Agent = %q, want %q", ua, geocodeUserAgent)
			}
		})
	}
}

// TestGeocode_BiasIsPartOfCacheKey checks that a search near another map
// centre asks upstream again, while a centre on the same grid cell and a
// repeat of the unbiased search are served from the cache.
func TestGeocode_BiasIsPartOfCacheKey(t *testing.T) {
	nominatim := newStubUpstream(t, http.StatusOK, splitBody)
	_, api := humatest.New(t)
	registerGeocode(api, testGeocoder(nominatim, nil))
	steps := []struct {
		path     string
		wantHits int64
	}{
		{"/geocode?q=Split", 1},
		{"/geocode?q=Split&lat=43.5&lon=16.4", 2},
		{"/geocode?q=split&lat=43.6&lon=16.3", 2},
		{"/geocode?q=Split&lat=54.4&lon=18.6", 3},
		{"/geocode?q=SPLIT", 3},
	}
	for _, s := range steps {
		if resp := api.Get(s.path); resp.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", s.path, resp.Code, resp.Body.String())
		}
		if got := nominatim.hits.Load(); got != s.wantHits {
			t.Fatalf("after %s: upstream hits = %d, want %d", s.path, got, s.wantHits)
		}
	}
}

// TestGeocode_PhotonFallback covers when the Photon fallback runs and what
// the caller gets: Photon answers when Nominatim finds no town or fails, but
// a throttled Nominatim answers 503 without touching Photon.
func TestGeocode_PhotonFallback(t *testing.T) {
	const onlyStreets = `[{"name":"Jastarnia","display_name":"Jastarnia street","lat":"52.2","lon":"21.0","category":"highway","type":"residential"}]`
	tests := []struct {
		name            string
		nominatimStatus int
		nominatimBody   string
		photonStatus    int
		photonBody      string
		wantStatus      int
		wantNames       []string
		wantSource      string
		wantPhotonHits  int64
	}{
		{"nominatim finds a town", http.StatusOK, splitBody, http.StatusOK, photonSplitBody, 0, []string{"Split"}, "nominatim", 0},
		{"nominatim finds nothing", http.StatusOK, `[]`, http.StatusOK, photonSplitBody, 0, []string{"Split"}, "photon", 1},
		{"nominatim finds no town", http.StatusOK, onlyStreets, http.StatusOK, photonSplitBody, 0, []string{"Split"}, "photon", 1},
		{"nominatim fails", http.StatusInternalServerError, "boom", http.StatusOK, photonSplitBody, 0, []string{"Split"}, "photon", 1},
		{"nominatim returns junk", http.StatusOK, "<html>", http.StatusOK, photonSplitBody, 0, []string{"Split"}, "photon", 1},
		{"nominatim throttled", http.StatusTooManyRequests, "slow down", http.StatusOK, photonSplitBody, http.StatusServiceUnavailable, nil, "", 0},
		{"both fail", http.StatusBadGateway, "boom", http.StatusInternalServerError, "boom", http.StatusBadGateway, nil, "", 1},
		{"nominatim fails, photon throttled", http.StatusInternalServerError, "boom", http.StatusTooManyRequests, "", http.StatusServiceUnavailable, nil, "", 1},
		{"both find nothing", http.StatusOK, `[]`, http.StatusOK, `{"features":[]}`, 0, []string{}, "", 1},
		{"nominatim finds nothing, photon fails", http.StatusOK, `[]`, http.StatusInternalServerError, "boom", 0, []string{}, "", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			photon := newStubUpstream(t, tt.photonStatus, tt.photonBody)
			h := testGeocoder(newStubUpstream(t, tt.nominatimStatus, tt.nominatimBody), photon)
			out, err := h.search(context.Background(), &geocodeInput{Query: "Jastarn"})
			if got := photon.hits.Load(); got != tt.wantPhotonHits {
				t.Fatalf("photon hits = %d, want %d", got, tt.wantPhotonHits)
			}
			if tt.wantStatus != 0 {
				wantStatus(t, err, tt.wantStatus)
				return
			}
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if got := names(out.Body); !reflect.DeepEqual(got, tt.wantNames) {
				t.Fatalf("names = %q, want %q", got, tt.wantNames)
			}
			for _, r := range out.Body {
				if r.Source != tt.wantSource {
					t.Fatalf("source = %q, want %q", r.Source, tt.wantSource)
				}
			}
		})
	}
}

// TestGeocode_NominatimUnreachableFallsBack checks that a connection
// failure, not only an error status, sends the lookup to Photon.
func TestGeocode_NominatimUnreachableFallsBack(t *testing.T) {
	nominatim := newStubUpstream(t, http.StatusOK, splitBody)
	photon := newStubUpstream(t, http.StatusOK, photonSplitBody)
	h := testGeocoder(nominatim, photon)
	nominatim.srv.Close()
	h.httpClient = http.DefaultClient
	out, err := h.search(context.Background(), &geocodeInput{Query: "Split"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out.Body) != 1 || out.Body[0].Source != "photon" {
		t.Fatalf("results = %+v, want one Photon result", out.Body)
	}
}

// TestGeocode_FallbackCaching checks which answers are cached: a Photon
// answer, even an empty one, is; an empty Nominatim answer whose fallback
// failed is not, so the next try reaches Photon again.
func TestGeocode_FallbackCaching(t *testing.T) {
	tests := []struct {
		name             string
		photonStatus     int
		photonBody       string
		wantSecondHits   int64
		wantSecondPhoton int64
	}{
		{"photon answer cached", http.StatusOK, photonSplitBody, 1, 1},
		{"empty photon answer cached", http.StatusOK, `{"features":[]}`, 1, 1},
		{"failed fallback not cached", http.StatusInternalServerError, "boom", 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nominatim := newStubUpstream(t, http.StatusOK, `[]`)
			photon := newStubUpstream(t, tt.photonStatus, tt.photonBody)
			h := testGeocoder(nominatim, photon)
			for range 2 {
				if _, err := h.search(context.Background(), &geocodeInput{Query: "Jastarn"}); err != nil {
					t.Fatalf("search: %v", err)
				}
			}
			if got := nominatim.hits.Load(); got != tt.wantSecondHits {
				t.Fatalf("nominatim hits = %d, want %d", got, tt.wantSecondHits)
			}
			if got := photon.hits.Load(); got != tt.wantSecondPhoton {
				t.Fatalf("photon hits = %d, want %d", got, tt.wantSecondPhoton)
			}
		})
	}
}

// TestGeocode_PhotonResults checks how Photon features map to results: only
// place features of settlement types with valid coordinates, longitude first
// in GeoJSON, a "Town, County, Region, Country" label and the same dedup as
// Nominatim.
func TestGeocode_PhotonResults(t *testing.T) {
	photon := newStubUpstream(t, http.StatusOK, `{"type":"FeatureCollection","features":[
		{"geometry":{"coordinates":[18.6788,54.6961]},"properties":{"name":"Jastarnia","osm_key":"place","osm_value":"town","county":"Puck County","state":"Pomeranian Voivodeship","country":"Poland"}},
		{"geometry":{"coordinates":[18.6790,54.6970]},"properties":{"name":"Jastarnia","osm_key":"place","osm_value":"village","county":"Puck County","state":"Pomeranian Voivodeship","country":"Poland"}},
		{"geometry":{"coordinates":[14.5680,68.2340]},"properties":{"name":"Svolvær","osm_key":"place","osm_value":"town","state":"Nordland","country":"Norway"}},
		{"geometry":{"coordinates":[18.0,54.0]},"properties":{"name":"Pomeranian Voivodeship","osm_key":"place","osm_value":"state","country":"Poland"}},
		{"geometry":{"coordinates":[18.0,54.0]},"properties":{"name":"Jastarnia Marina","osm_key":"leisure","osm_value":"marina","country":"Poland"}},
		{"geometry":{"coordinates":[18.0]},"properties":{"name":"Broken","osm_key":"place","osm_value":"town"}},
		{"geometry":{"coordinates":[200,54.0]},"properties":{"name":"Far","osm_key":"place","osm_value":"town"}},
		{"geometry":{"coordinates":[18.0,54.0]},"properties":{"name":"","osm_key":"place","osm_value":"town"}},
		{"geometry":{"coordinates":[12.5683,55.6761]},"properties":{"name":"Copenhagen","osm_key":"place","osm_value":"city","county":"Copenhagen","state":"Capital Region of Denmark","country":"Denmark"}}
	]}`)
	h := testGeocoder(newStubUpstream(t, http.StatusOK, `[]`), photon)
	in := &geocodeInput{Query: " Jastarn ", bias: &geocodeBias{lat: 54.5, lon: 18.5}}
	out, err := h.search(context.Background(), in)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	want := []dto.GeocodeResult{
		{Name: "Jastarnia", Label: "Jastarnia, Puck County, Pomeranian Voivodeship, Poland", Latitude: 54.6961, Longitude: 18.6788, Source: "photon"},
		{Name: "Svolvær", Label: "Svolvær, Nordland, Norway", Latitude: 68.2340, Longitude: 14.5680, Source: "photon"},
		{Name: "Copenhagen", Label: "Copenhagen, Capital Region of Denmark, Denmark", Latitude: 55.6761, Longitude: 12.5683, Source: "photon"},
	}
	if !reflect.DeepEqual(out.Body, want) {
		t.Fatalf("results = %+v, want %+v", out.Body, want)
	}

	p := photon.lastParams(t)
	wantParams := map[string]string{
		"q": "Jastarn", "limit": "10", "lang": "en", "osm_tag": "place",
		"lat": "54.5", "lon": "18.5", "zoom": photonBiasZoom,
	}
	for k, v := range wantParams {
		if got := p.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if ua := photon.agents[0]; ua != geocodeUserAgent {
		t.Errorf("User-Agent = %q, want %q", ua, geocodeUserAgent)
	}
}

func TestGeocode_PhotonRequestWithoutBias(t *testing.T) {
	photon := newStubUpstream(t, http.StatusOK, photonSplitBody)
	h := testGeocoder(newStubUpstream(t, http.StatusOK, `[]`), photon)
	if _, err := h.search(context.Background(), &geocodeInput{Query: "Split"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	p := photon.lastParams(t)
	for _, k := range []string{"lat", "lon", "zoom"} {
		if p.Has(k) {
			t.Errorf("%s sent without a bias: %q", k, p.Get(k))
		}
	}
}

func TestGeocode_UpstreamError(t *testing.T) {
	h := stubNominatim(t, http.StatusInternalServerError, "boom")
	_, err := h.search(context.Background(), &geocodeInput{Query: "Split"})
	wantStatus(t, err, http.StatusBadGateway)
}

func TestGeocode_NoResults(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[]`)
	out, err := h.search(context.Background(), &geocodeInput{Query: "zzzzz"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if out.Body == nil || len(out.Body) != 0 {
		t.Fatalf("got %#v, want an empty slice", out.Body)
	}
}

// TestGeocode_CachesNormalizedQuery checks that repeat lookups differing only
// in case and surrounding whitespace or invisible characters reach the
// upstream once.
func TestGeocode_CachesNormalizedQuery(t *testing.T) {
	nominatim := newStubUpstream(t, http.StatusOK, splitBody)
	h := testGeocoder(nominatim, nil)
	for _, q := range []string{"Split", "  split ", "SPLIT", "\u200bsplit\ufeff"} {
		out, err := h.search(context.Background(), &geocodeInput{Query: q})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if len(out.Body) != 1 || out.Body[0].Name != "Split" {
			t.Fatalf("search %q: unexpected results %+v", q, out.Body)
		}
	}
	if got := nominatim.hits.Load(); got != 1 {
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
		nominatim:  geocodeUpstream{baseURL: srv.URL, limiter: rate.NewLimiter(rate.Every(10*time.Millisecond), 1)},
		httpClient: srv.Client(),
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
	nominatim := newStubUpstream(t, http.StatusOK, splitBody)
	h := testGeocoder(nominatim, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.search(ctx, &geocodeInput{Query: "Split"}); err != nil {
		t.Fatalf("search with cancelled caller: %v", err)
	}
	if got := nominatim.hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

// TestGeocode_CachesEmptyResults checks that a query with no matches is not
// re-sent upstream either.
func TestGeocode_CachesEmptyResults(t *testing.T) {
	nominatim := newStubUpstream(t, http.StatusOK, `[]`)
	h := testGeocoder(nominatim, nil)
	for range 2 {
		if _, err := h.search(context.Background(), &geocodeInput{Query: "zzzzz"}); err != nil {
			t.Fatalf("search: %v", err)
		}
	}
	if got := nominatim.hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

func TestGeocode_DoesNotCacheErrors(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			nominatim := newStubUpstream(t, status, "boom")
			h := testGeocoder(nominatim, nil)
			for range 2 {
				if _, err := h.search(context.Background(), &geocodeInput{Query: "Split"}); err == nil {
					t.Fatal("expected an error")
				}
			}
			if got := nominatim.hits.Load(); got != 2 {
				t.Fatalf("upstream hits = %d, want 2", got)
			}
		})
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
		nominatim:  geocodeUpstream{baseURL: srv.URL, limiter: rate.NewLimiter(rate.Every(interval), 1)},
		httpClient: srv.Client(),
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
	const minGap = interval * 8 / 10
	for i := 1; i < len(times); i++ {
		if gap := times[i].Sub(times[i-1]); gap < minGap {
			t.Fatalf("hits %d and %d were %v apart, want >= %v", i-1, i, gap, minGap)
		}
	}
}

// TestGeocode_BusyWhenLimiterQueueTooLong checks that a request that would
// wait longer than maxWait fails with 503 and reaches neither upstream.
func TestGeocode_BusyWhenLimiterQueueTooLong(t *testing.T) {
	nominatim := newStubUpstream(t, http.StatusOK, splitBody)
	photon := newStubUpstream(t, http.StatusOK, photonSplitBody)
	h := testGeocoder(nominatim, photon)
	h.nominatim.limiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	h.maxWait = 10 * time.Millisecond
	if _, err := h.search(context.Background(), &geocodeInput{Query: "Split"}); err != nil {
		t.Fatalf("first search: %v", err)
	}
	_, err := h.search(context.Background(), &geocodeInput{Query: "Hvar"})
	wantStatus(t, err, http.StatusServiceUnavailable)
	if got := nominatim.hits.Load(); got != 1 {
		t.Fatalf("nominatim hits = %d, want 1", got)
	}
	if got := photon.hits.Load(); got != 0 {
		t.Fatalf("photon hits = %d, want 0", got)
	}
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
	now = now.Add(45 * time.Minute)
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
	if h.nominatim.limiter.Limit() != rate.Every(time.Second) || h.nominatim.limiter.Burst() != 1 {
		t.Fatalf("nominatim limiter = %v/%d, want 1 req/s burst 1", h.nominatim.limiter.Limit(), h.nominatim.limiter.Burst())
	}
	if h.photon.limiter.Limit() != rate.Every(500*time.Millisecond) || h.photon.limiter.Burst() != 1 {
		t.Fatalf("photon limiter = %v/%d, want 2 req/s burst 1", h.photon.limiter.Limit(), h.photon.limiter.Burst())
	}
	if h.nominatim.baseURL != nominatimBaseURL || h.photon.baseURL != photonBaseURL {
		t.Fatalf("base URLs = %q, %q", h.nominatim.baseURL, h.photon.baseURL)
	}
	if h.cache.ttl != 24*time.Hour || h.cache.limit != 1000 {
		t.Fatalf("cache = ttl %v limit %d, want 24h/1000", h.cache.ttl, h.cache.limit)
	}
}

// TestGeocode_QueryValidation drives the operation through huma: a missing,
// bare, empty or too-short query, counting only base characters (no blanks,
// blank lookalikes or combining marks), or a half or invalid map bias is
// rejected with 422 before it reaches the cache, the rate limiter or the
// upstream service. An accepted query reaches the upstream trimmed, with the
// marks of decomposed letters intact.
func TestGeocode_QueryValidation(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantUpQuery string
	}{
		{"missing", "/geocode", http.StatusUnprocessableEntity, ""},
		{"bare", "/geocode?q", http.StatusUnprocessableEntity, ""},
		{"bare before value", "/geocode?q&q=ab", http.StatusUnprocessableEntity, ""},
		{"bare escaped name", "/geocode?%71", http.StatusUnprocessableEntity, ""},
		{"empty", "/geocode?q=", http.StatusUnprocessableEntity, ""},
		{"one character", "/geocode?q=a", http.StatusUnprocessableEntity, ""},
		{"whitespace only", "/geocode?q=%20%20%20", http.StatusUnprocessableEntity, ""},
		{"one character padded", "/geocode?q=%20a%20", http.StatusUnprocessableEntity, ""},
		{"zero-width spaces", "/geocode?q=%E2%80%8B%E2%80%8B", http.StatusUnprocessableEntity, ""},
		{"byte order marks", "/geocode?q=%EF%BB%BF%EF%BB%BF", http.StatusUnprocessableEntity, ""},
		{"NUL bytes", "/geocode?q=%00%00", http.StatusUnprocessableEntity, ""},
		{"invalid UTF-8", "/geocode?q=%FF%FF", http.StatusUnprocessableEntity, ""},
		{"one character among invisibles", "/geocode?q=%E2%80%8Ba%00%EF%BB%BF", http.StatusUnprocessableEntity, ""},
		{"Hangul fillers", "/geocode?q=%E3%85%A4%E3%85%A4", http.StatusUnprocessableEntity, ""},
		{"Hangul choseong and halfwidth fillers", "/geocode?q=%E1%85%9F%EF%BE%A0", http.StatusUnprocessableEntity, ""},
		{"Hangul jungseong fillers", "/geocode?q=%E1%85%A0%E1%85%A0", http.StatusUnprocessableEntity, ""},
		{"braille blanks", "/geocode?q=%E2%A0%80%E2%A0%80", http.StatusUnprocessableEntity, ""},
		{"combining grapheme joiners", "/geocode?q=%CD%8F%CD%8F", http.StatusUnprocessableEntity, ""},
		{"variation selectors", "/geocode?q=%EF%B8%8F%EF%B8%8F", http.StatusUnprocessableEntity, ""},
		{"supplementary variation selectors", "/geocode?q=%F3%A0%84%80%F3%A0%84%80", http.StatusUnprocessableEntity, ""},
		{"lone combining marks", "/geocode?q=%CC%81%CC%81", http.StatusUnprocessableEntity, ""},
		{"one decomposed letter", "/geocode?q=a%CC%81", http.StatusUnprocessableEntity, ""},
		{"one character among blank lookalikes", "/geocode?q=%E3%85%A4a%E2%A0%80%EF%B8%8F", http.StatusUnprocessableEntity, ""},
		{"one character among inner blanks", "/geocode?q=a%E2%A0%80%E3%85%A4%CD%8F", http.StatusUnprocessableEntity, ""},
		{"lat without lon", "/geocode?q=ab&lat=54", http.StatusUnprocessableEntity, ""},
		{"lon without lat", "/geocode?q=ab&lon=18", http.StatusUnprocessableEntity, ""},
		{"lat out of range", "/geocode?q=ab&lat=91&lon=18", http.StatusUnprocessableEntity, ""},
		{"lon out of range", "/geocode?q=ab&lat=54&lon=-181", http.StatusUnprocessableEntity, ""},
		{"NaN lat", "/geocode?q=ab&lat=NaN&lon=18", http.StatusUnprocessableEntity, ""},
		{"infinite lon", "/geocode?q=ab&lat=54&lon=Inf", http.StatusUnprocessableEntity, ""},
		{"text lat", "/geocode?q=ab&lat=north&lon=18", http.StatusUnprocessableEntity, ""},
		{"two characters", "/geocode?q=ab", http.StatusOK, "ab"},
		{"with bias", "/geocode?q=ab&lat=54.4&lon=18.6", http.StatusOK, "ab"},
		{"two characters padded", "/geocode?q=%20ab%20", http.StatusOK, "ab"},
		{"two multibyte characters", "/geocode?q=%C5%81%C3%B3", http.StatusOK, "Łó"},
		{"two characters among invisibles", "/geocode?q=%E2%80%8Bab%00%EF%BB%BF", http.StatusOK, "ab"},
		{"inner invisible kept", "/geocode?q=a%E2%80%8Bb", http.StatusOK, "a\u200bb"},
		{"two characters among blank lookalikes", "/geocode?q=%E3%85%A4ab%EF%B8%8F%E2%A0%80", http.StatusOK, "ab"},
		{"inner blank lookalike kept", "/geocode?q=a%E2%A0%80b", http.StatusOK, "a\u2800b"},
		{"NFD name keeps its marks", "/geocode?q=%C5%81o%CC%81dz%CC%81", http.StatusOK, "\u0141o\u0301dz\u0301"},
		{"two decomposed letters", "/geocode?q=a%CC%81e%CC%81", http.StatusOK, "a\u0301e\u0301"},
		{"literal true", "/geocode?q=true", http.StatusOK, "true"},
		{"value before bare", "/geocode?q=ab&q", http.StatusOK, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nominatim := newStubUpstream(t, http.StatusOK, splitBody)
			h := testGeocoder(nominatim, nil)
			limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
			h.nominatim.limiter = limiter
			_, api := humatest.New(t)
			registerGeocode(api, h)

			resp := api.Get(tt.path)
			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", resp.Code, tt.wantStatus, resp.Body.String())
			}
			nominatim.mu.Lock()
			var got []string
			for _, p := range nominatim.params {
				got = append(got, p.Get("q"))
			}
			nominatim.mu.Unlock()
			if tt.wantUpQuery != "" {
				if len(got) != 1 || got[0] != tt.wantUpQuery {
					t.Fatalf("upstream queries = %q, want [%q]", got, tt.wantUpQuery)
				}
				return
			}
			if len(got) != 0 {
				t.Fatalf("upstream queries = %q, want none", got)
			}
			if n := len(h.cache.entries); n != 0 {
				t.Fatalf("cache entries = %d, want 0", n)
			}
			if tokens := limiter.Tokens(); tokens < 1 {
				t.Fatalf("limiter tokens = %v, want 1 (unspent)", tokens)
			}
		})
	}
}

// TestGeocode_DropsInvalidCoordinates drives the operation through huma with
// upstream records whose coordinates are NaN, infinite or out of range. Each
// such record is dropped, the valid records are kept, the response encodes as
// JSON, and the repeat request is served from a cache that holds only the
// valid records.
func TestGeocode_DropsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name     string
		lat, lon string
	}{
		{"NaN latitude", "NaN", "16.44"},
		{"NaN longitude", "43.50", "nan"},
		{"Inf latitude", "Inf", "16.44"},
		{"negative Inf latitude", "-Inf", "16.44"},
		{"Infinity longitude", "43.50", "+Infinity"},
		{"overflowing latitude", "1e400", "16.44"},
		{"latitude above 90", "90.5", "16.44"},
		{"latitude below -90", "-91", "16.44"},
		{"longitude above 180", "43.50", "180.01"},
		{"longitude below -180", "43.50", "-200"},
	}
	want := []dto.GeocodeResult{
		{Name: "Split", Label: "Split, Croatia", Latitude: 43.5081, Longitude: 16.4402, Source: "nominatim"},
		{Name: "Pole", Label: "North Pole", Latitude: 90, Longitude: -180, Source: "nominatim"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `[
				{"name":"Broken","display_name":"Broken","lat":"` + tt.lat + `","lon":"` + tt.lon + `","category":"place","type":"town"},
				{"name":"Split","display_name":"Split, Croatia","lat":"43.5081","lon":"16.4402","category":"place","type":"city"},
				{"name":"Pole","display_name":"North Pole","lat":"90","lon":"-180","category":"place","type":"locality"}
			]`
			nominatim := newStubUpstream(t, http.StatusOK, body)
			_, api := humatest.New(t)
			registerGeocode(api, testGeocoder(nominatim, nil))

			for range 2 {
				resp := api.Get("/geocode?q=Split")
				if resp.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200: %s", resp.Code, resp.Body.String())
				}
				var got []dto.GeocodeResult
				if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode %q: %v", resp.Body.String(), err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("results = %+v, want %+v", got, want)
				}
			}
			if n := nominatim.hits.Load(); n != 1 {
				t.Fatalf("upstream hits = %d, want 1", n)
			}
		})
	}
}

func TestFoldPlaceName(t *testing.T) {
	tests := map[string]string{
		"Świnoujście":        "swinoujscie",
		"  ŁEBA ":            "leba",
		"Svolvær":            "svolvaer",
		"København":          "kobenhavn",
		"Gdan\u0301sk":       "gdansk",
		"Nowa\u00a0  Wieś":   "nowa wies",
		"Hel\u200b":          "hel",
		"Λευκάδα":            "λευκαδα",
		"Szczecin, Polska":   "szczecin, polska",
		"Straße am Hafen":    "strasse am hafen",
		"Đurđevac":           "durdevac",
		"Œuilly":             "oeuilly",
		"Þórshöfn":           "thorshofn",
		"Ísafjörður":         "isafjordur",
		"Kızkalesi":          "kizkalesi",
		"Kraków  i  okolice": "krakow i okolice",
	}
	for in, want := range tests {
		if got := foldPlaceName(in); got != want {
			t.Errorf("foldPlaceName(%q) = %q, want %q", in, got, want)
		}
	}
}
