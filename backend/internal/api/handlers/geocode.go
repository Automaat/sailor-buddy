package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
)

const (
	nominatimBaseURL = "https://nominatim.openstreetmap.org/search"
	// The Nominatim usage policy allows at most 1 request per second and asks
	// proxies to cache. Ports rarely move, so a day-long TTL is safe.
	geocodeInterval   = time.Second
	geocodeCacheTTL   = 24 * time.Hour
	geocodeCacheLimit = 1000
	// geocodeMaxWait bounds how long a request queues for the limiter, so a
	// burst of lookups fails fast instead of piling up.
	geocodeMaxWait = 5 * time.Second
)

// GeocodeHandler proxies town-name lookups to an OpenStreetMap Nominatim
// instance. Upstream calls share one rate limiter, responses are cached by
// normalized query and concurrent identical lookups share one upstream call.
// The base URL, HTTP client, limiter and cache are injectable so tests can
// stub the upstream service.
type GeocodeHandler struct {
	baseURL    string
	httpClient *http.Client
	limiter    *rate.Limiter
	maxWait    time.Duration
	cache      *geocodeCache
	inflight   singleflight.Group
}

func NewGeocodeHandler() *GeocodeHandler {
	return &GeocodeHandler{
		baseURL:    nominatimBaseURL,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		limiter:    rate.NewLimiter(rate.Every(geocodeInterval), 1),
		maxWait:    geocodeMaxWait,
		cache:      newGeocodeCache(geocodeCacheTTL, geocodeCacheLimit, time.Now),
	}
}

type geocodeEntry struct {
	results []dto.GeocodeResult
	stored  time.Time
}

// geocodeCache is a mutex-guarded TTL cache with a fixed entry cap. At the
// cap it drops expired entries, then the oldest one.
type geocodeCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	limit   int
	now     func() time.Time
	entries map[string]geocodeEntry
}

func newGeocodeCache(ttl time.Duration, limit int, now func() time.Time) *geocodeCache {
	return &geocodeCache{ttl: ttl, limit: limit, now: now, entries: make(map[string]geocodeEntry)}
}

func (c *geocodeCache) get(key string) ([]dto.GeocodeResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.now().Sub(e.stored) >= c.ttl {
		delete(c.entries, key)
		return nil, false
	}
	return e.results, true
}

func (c *geocodeCache) put(key string, results []dto.GeocodeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.limit {
		c.evictLocked(now)
	}
	c.entries[key] = geocodeEntry{results: results, stored: now}
}

func (c *geocodeCache) evictLocked(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for k, e := range c.entries {
		if now.Sub(e.stored) >= c.ttl {
			delete(c.entries, k)
			continue
		}
		if oldestKey == "" || e.stored.Before(oldest) {
			oldestKey, oldest = k, e.stored
		}
	}
	if len(c.entries) >= c.limit {
		delete(c.entries, oldestKey)
	}
}

// geocodeKey normalizes a query so case and surrounding whitespace do not
// defeat the cache; Nominatim matching ignores both.
func geocodeKey(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

// geocodeMinQueryLen is the shortest query, in characters after trimming
// surrounding whitespace, worth sending upstream.
const geocodeMinQueryLen = 2

type geocodeInput struct {
	Query string `query:"q" required:"true" minLength:"2" doc:"Town or place name to search for"`
}

// Resolve rejects a query that is too short once surrounding whitespace is
// trimmed: the upstream call sends the trimmed text, so "  " or " a" would
// spend a request of the Nominatim rate budget on an empty or one-letter
// search. huma runs it before the handler, so a rejected query touches
// neither the cache nor the limiter.
func (i *geocodeInput) Resolve(_ huma.Context) []error {
	if utf8.RuneCountInString(strings.TrimSpace(i.Query)) < geocodeMinQueryLen {
		return []error{&huma.ErrorDetail{
			Message:  fmt.Sprintf("expected at least %d non-whitespace characters", geocodeMinQueryLen),
			Location: "query.q",
			Value:    i.Query,
		}}
	}
	return nil
}

type geocodeOutput struct {
	Body []dto.GeocodeResult
}

// nominatimResult is the subset of a Nominatim jsonv2 record we consume.
// jsonv2 names the feature class "category"; only the older json format
// calls it "class".
type nominatimResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Category    string `json:"category"`
}

// placeCategories is the set of Nominatim feature categories a sailing
// destination can plausibly belong to: populated places and the
// administrative areas that share their name. Streets, buildings and POIs are
// dropped as noise.
var placeCategories = map[string]bool{"place": true, "boundary": true, "natural": true}

// RegisterGeocodeRoutes wires the geocoding proxy onto the API.
func RegisterGeocodeRoutes(api huma.API) {
	registerGeocode(api, NewGeocodeHandler())
}

// registerGeocode registers the geocode operation backed by h, so tests can
// exercise the full huma request path against a stub upstream.
func registerGeocode(api huma.API, h *GeocodeHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "geocode", Method: http.MethodGet, Path: "/geocode",
		Summary: "Search towns/places by name", Tags: []string{"Geocode"},
	}, h.search)
}

func (h *GeocodeHandler) search(ctx context.Context, in *geocodeInput) (*geocodeOutput, error) {
	key := geocodeKey(in.Query)
	if cached, ok := h.cache.get(key); ok {
		return &geocodeOutput{Body: cached}, nil
	}

	// Followers share the leader's lookup, so it must not die with the
	// leader's request; maxWait and the client timeout still bound it.
	lookupCtx := context.WithoutCancel(ctx)
	v, err, _ := h.inflight.Do(key, func() (any, error) {
		return h.lookup(lookupCtx, key, in.Query)
	})
	if err != nil {
		return nil, err
	}
	results, _ := v.([]dto.GeocodeResult)
	return &geocodeOutput{Body: results}, nil
}

// lookup waits for the rate limiter, then fetches and caches one query.
func (h *GeocodeHandler) lookup(ctx context.Context, key, query string) ([]dto.GeocodeResult, error) {
	// A lookup that finished just before this one joined the flight has
	// already filled the cache.
	if cached, ok := h.cache.get(key); ok {
		return cached, nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, h.maxWait)
	err := h.limiter.Wait(waitCtx)
	cancel()
	if err != nil {
		slog.Warn("geocode rate limited", "query", query, "err", err)
		return nil, huma.Error503ServiceUnavailable("geocoding service busy, try again")
	}
	results, err := h.fetch(ctx, query)
	if err != nil {
		return nil, err
	}
	h.cache.put(key, results)
	return results, nil
}

func (h *GeocodeHandler) fetch(ctx context.Context, query string) ([]dto.GeocodeResult, error) {
	q := url.Values{}
	q.Set("q", strings.TrimSpace(query))
	q.Set("format", "jsonv2")
	// Over-fetch: category filtering and coordinate dedup below trim the list,
	// so a higher limit keeps ~5 usable matches.
	q.Set("limit", "10")
	q.Set("addressdetails", "0")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"?"+q.Encode(), http.NoBody)
	if err != nil {
		slog.Error("geocode build request", "query", query, "err", err)
		return nil, huma.Error500InternalServerError("failed to search places")
	}
	// Nominatim's usage policy requires a descriptive User-Agent that
	// identifies the application and a way to reach its operator.
	req.Header.Set("User-Agent", "sailor-buddy/1.0 (+https://github.com/Automaat/sailor-buddy)")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Error("geocode request", "query", query, "err", err)
		return nil, huma.Error502BadGateway("geocoding service unavailable")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Error("geocode upstream status", "query", query, "status", resp.StatusCode)
		return nil, huma.Error502BadGateway("geocoding service unavailable")
	}

	var raw []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		slog.Error("geocode decode", "query", query, "err", err)
		return nil, huma.Error502BadGateway("geocoding service returned invalid data")
	}

	// Nominatim returns matches in descending importance order; the filtering
	// loop preserves that, so the best match stays first.
	results := make([]dto.GeocodeResult, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, r := range raw {
		// Drop streets, buildings and POIs — only towns and the admin areas
		// that share their name make sense as a sailing destination.
		if !placeCategories[r.Category] {
			continue
		}
		lat, lon, ok := parseCoordinates(r.Lat, r.Lon)
		if !ok {
			continue
		}
		// Nominatim often lists the same town twice (a place node and its
		// administrative boundary); collapse near-identical coordinates.
		key := fmt.Sprintf("%.2f,%.2f", lat, lon)
		if seen[key] {
			continue
		}
		seen[key] = true
		name := r.Name
		if name == "" {
			name = r.DisplayName
		}
		label := r.DisplayName
		if label == "" {
			label = name
		}
		results = append(results, dto.GeocodeResult{
			Name: name, Label: label, Latitude: lat, Longitude: lon,
		})
		if len(results) == 5 {
			break
		}
	}
	return results, nil
}

// parseCoordinates parses a Nominatim lat/lon pair and reports whether it is
// a usable position. strconv.ParseFloat accepts "NaN" and "Inf", which the
// JSON encoder cannot write, so a record carrying one would break the whole
// response. ±Inf also fails the range check, which drops out-of-range
// positions from a broken upstream.
func parseCoordinates(rawLat, rawLon string) (lat, lon float64, ok bool) {
	lat, latErr := strconv.ParseFloat(rawLat, 64)
	lon, lonErr := strconv.ParseFloat(rawLon, 64)
	if latErr != nil || lonErr != nil {
		return 0, 0, false
	}
	if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}
