package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/sync/singleflight"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/time/rate"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
)

const (
	nominatimBaseURL = "https://nominatim.openstreetmap.org/search"
	photonBaseURL    = "https://photon.komoot.io/api/"
)

// geocodeUserAgent identifies the application and its operator, as both the
// Nominatim and the Photon usage policies require.
const geocodeUserAgent = "sailor-buddy/1.0 (+https://github.com/Automaat/sailor-buddy)"

// geocodeInterval follows the Nominatim usage policy: at most 1 request per
// second, and proxies should cache. Ports rarely move, so a day-long cache
// TTL is safe.
const geocodeInterval = time.Second

// photonInterval keeps the Photon fallback polite: komoot sets no numeric
// limit, only fair use.
const photonInterval = 500 * time.Millisecond

const (
	geocodeCacheTTL   = 24 * time.Hour
	geocodeCacheLimit = 1000
)

// geocodeMaxWait bounds how long a request queues for a limiter, so a burst
// of lookups fails fast instead of piling up.
const geocodeMaxWait = 5 * time.Second

// geocodeFetchLimit over-fetches from Nominatim: the settlement filter and
// the dedup trim the list, so asking for more keeps ~geocodeMaxResults.
const (
	geocodeFetchLimit = 15
	photonFetchLimit  = 10
	geocodeMaxResults = 5
)

// geocodeDupRadiusKm is how close two equally named results must lie to count
// as one place, such as a town's node and the centroid of its boundary.
const geocodeDupRadiusKm = 5.0

// The map-centre bias is rounded to a geocodeBiasStep grid, so panning the
// map a little reuses the cached answer. The Nominatim viewbox spans a
// sailing region around the rounded centre; Photon's zoom gives a similar
// radius.
const (
	geocodeBiasStep        = 0.5
	geocodeViewboxHalfLat  = 2.0
	geocodeViewboxHalfLon  = 3.0
	photonBiasZoom         = "7"
	geocodeSourceNominatim = "nominatim"
	geocodeSourcePhoton    = "photon"
)

// geocodeUpstream is one search service and the limiter that keeps the proxy
// within its usage policy. An empty baseURL turns the service off.
type geocodeUpstream struct {
	name    string
	baseURL string
	limiter *rate.Limiter
}

// GeocodeHandler proxies town-name lookups to OpenStreetMap Nominatim and,
// when Nominatim finds no town or fails, to Photon. Each upstream has its own
// rate limiter, answers are cached by normalized query and map bias, and
// concurrent identical lookups share one upstream call. Base URLs, HTTP
// client, limiters and cache are injectable so tests can stub both services.
type GeocodeHandler struct {
	nominatim  geocodeUpstream
	photon     geocodeUpstream
	httpClient *http.Client
	maxWait    time.Duration
	cache      *geocodeCache
	inflight   singleflight.Group
}

func NewGeocodeHandler() *GeocodeHandler {
	return &GeocodeHandler{
		nominatim: geocodeUpstream{
			name: geocodeSourceNominatim, baseURL: nominatimBaseURL,
			limiter: rate.NewLimiter(rate.Every(geocodeInterval), 1),
		},
		photon: geocodeUpstream{
			name: geocodeSourcePhoton, baseURL: photonBaseURL,
			limiter: rate.NewLimiter(rate.Every(photonInterval), 1),
		},
		httpClient: &http.Client{Timeout: 8 * time.Second},
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

// geocodeKey normalizes a query so case and surrounding invisible characters
// do not defeat the cache; Nominatim matching ignores both.
func geocodeKey(q string) string {
	return strings.ToLower(trimGeocodeQuery(q))
}

// isBlankGeocodeRune reports whether r carries no searchable text:
//   - whitespace;
//   - format (Cf, such as zero-width spaces and the byte order mark) and
//     control (Cc, such as NUL) characters;
//   - the other default-ignorable code points: Hangul fillers (U+115F,
//     U+1160, U+3164, U+FFA0), the combining grapheme joiner (U+034F) and the
//     variation selectors (U+FE00..U+FE0F and others);
//   - the braille blank (U+2800), which renders as an empty cell;
//   - bytes that are not valid UTF-8.
//
// It matches explicit code points, not whole categories: the combining
// marks of decomposed (NFD) letters, such as the acute in "ź", stay.
func isBlankGeocodeRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cc, r) ||
		unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r) ||
		unicode.Is(unicode.Variation_Selector, r) ||
		r == '⠀' || r == utf8.RuneError
}

// trimGeocodeQuery strips leading and trailing runes that isBlankGeocodeRune
// matches. Blank runes between other characters stay. The result may still
// start with a combining mark or hold only marks, so the length check counts
// with geocodeBaseCharCount instead of the rune count.
func trimGeocodeQuery(q string) string {
	return strings.TrimFunc(q, isBlankGeocodeRune)
}

// geocodeBaseCharCount counts the runes in q that are neither blank nor
// combining marks (M). A letter and its decomposed accents count once, so NFC
// and NFD spellings of a name have the same count.
func geocodeBaseCharCount(q string) int {
	n := 0
	for _, r := range q {
		if !isBlankGeocodeRune(r) && !unicode.Is(unicode.M, r) {
			n++
		}
	}
	return n
}

// geocodeMinQueryLen is the fewest base characters, as geocodeBaseCharCount
// counts them, that a query worth sending upstream holds.
const geocodeMinQueryLen = 2

type geocodeInput struct {
	Query string  `query:"q" required:"true" minLength:"2" doc:"Town or place name to search for"`
	Lat   float64 `query:"lat" minimum:"-90" maximum:"90" doc:"Latitude of the map centre; nearby places rank higher. Give together with lon"`
	Lon   float64 `query:"lon" minimum:"-180" maximum:"180" doc:"Longitude of the map centre; nearby places rank higher. Give together with lat"`

	bias *geocodeBias
}

// Resolve rejects a query that holds fewer than two base characters: the
// upstream call sends the trimmed text, so "  ", " a", two zero-width spaces,
// two Hangul fillers or two lone combining marks would spend a request of the
// Nominatim rate budget on an empty or one-letter search. It also rejects a
// bare "q" with no "=", which huma reads as the value "true", and a lat
// without lon or the other way round. huma runs Resolve before the handler,
// so a rejected query touches neither the cache nor the limiters.
func (i *geocodeInput) Resolve(ctx huma.Context) []error {
	u := ctx.URL()
	if isBareQueryParam(u.RawQuery, "q") {
		return []error{&huma.ErrorDetail{
			Message:  "expected a value for q",
			Location: "query.q",
		}}
	}
	if geocodeBaseCharCount(i.Query) < geocodeMinQueryLen {
		return []error{&huma.ErrorDetail{
			Message:  fmt.Sprintf("expected at least %d visible characters", geocodeMinQueryLen),
			Location: "query.q",
			Value:    i.Query,
		}}
	}
	params := u.Query()
	hasLat, hasLon := params.Has("lat"), params.Has("lon")
	if hasLat != hasLon {
		return []error{&huma.ErrorDetail{
			Message:  "lat and lon must be given together",
			Location: "query.lat",
		}}
	}
	if hasLat {
		if math.IsNaN(i.Lat) || math.IsNaN(i.Lon) {
			return []error{&huma.ErrorDetail{Message: "expected a number", Location: "query.lat"}}
		}
		b := roundGeocodeBias(i.Lat, i.Lon)
		i.bias = &b
	}
	return nil
}

// isBareQueryParam reports whether the first occurrence of name in rawQuery
// has no "=". It walks the query the way huma's queryparam.Get does, which
// returns "true" for such a flag-style parameter.
func isBareQueryParam(rawQuery, name string) bool {
	for part := range strings.SplitSeq(rawQuery, "&") {
		key, _, hasValue := strings.Cut(part, "=")
		if k, err := url.QueryUnescape(key); err == nil && k == name {
			return !hasValue
		}
	}
	return false
}

// geocodeBias is a map centre rounded to the geocodeBiasStep grid. The
// rounded value is both sent upstream and part of the cache key, so a cached
// answer always matches the request that produced it.
type geocodeBias struct {
	lat, lon float64
}

func roundGeocodeBias(lat, lon float64) geocodeBias {
	round := func(v float64) float64 { return math.Round(v/geocodeBiasStep) * geocodeBiasStep }
	return geocodeBias{lat: round(lat), lon: round(lon)}
}

// viewbox returns the Nominatim viewbox (left,top,right,bottom) around the
// bias, clamped to valid coordinates.
func (b geocodeBias) viewbox() string {
	clamp := func(v, limit float64) float64 { return math.Max(-limit, math.Min(limit, v)) }
	return strings.Join([]string{
		formatCoord(clamp(b.lon-geocodeViewboxHalfLon, 180)),
		formatCoord(clamp(b.lat+geocodeViewboxHalfLat, 90)),
		formatCoord(clamp(b.lon+geocodeViewboxHalfLon, 180)),
		formatCoord(clamp(b.lat-geocodeViewboxHalfLat, 90)),
	}, ",")
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// geocodeCacheKey adds the rounded bias to the normalized query: a biased
// search ranks other towns first, so it must not share an unbiased answer.
func geocodeCacheKey(q string, bias *geocodeBias) string {
	key := geocodeKey(q)
	if bias == nil {
		return key + "|"
	}
	return key + "|" + formatCoord(bias.lat) + "," + formatCoord(bias.lon)
}

type geocodeOutput struct {
	Body []dto.GeocodeResult
}

// nominatimResult is the subset of a Nominatim jsonv2 record we consume.
// jsonv2 names the feature class "category"; only the older json format
// calls it "class". With addressdetails=1 the address holds the county,
// state and country used for the label.
type nominatimResult struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Category    string            `json:"category"`
	Type        string            `json:"type"`
	AddressType string            `json:"addresstype"`
	Address     map[string]string `json:"address"`
}

// photonResponse is the subset of a Photon GeoJSON answer we consume.
// GeoJSON orders coordinates longitude first.
type photonResponse struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Name     string `json:"name"`
			OSMKey   string `json:"osm_key"`
			OSMValue string `json:"osm_value"`
			County   string `json:"county"`
			State    string `json:"state"`
			Country  string `json:"country"`
		} `json:"properties"`
	} `json:"features"`
}

// settlementTypes are the OpenStreetMap place values a port town can carry.
// Nominatim and Photon both report them: as the type of a "place" record, or
// as the address type of an administrative boundary drawn around one.
var settlementTypes = map[string]bool{
	"city": true, "town": true, "village": true, "hamlet": true, "suburb": true,
	"island": true, "islet": true, "locality": true, "municipality": true,
}

// isNominatimSettlement keeps towns and the administrative boundaries of
// towns. Streets, POIs, marinas, natural features and regions are dropped.
func isNominatimSettlement(r nominatimResult) bool {
	switch r.Category {
	case "place":
		return settlementTypes[r.Type]
	case "boundary":
		return r.Type == "administrative" && settlementTypes[r.AddressType]
	}
	return false
}

// RegisterGeocodeRoutes wires the geocoding proxy onto the API.
func RegisterGeocodeRoutes(api huma.API) {
	registerGeocode(api, NewGeocodeHandler())
}

// registerGeocode registers the geocode operation backed by h, so tests can
// exercise the full huma request path against stub upstreams.
func registerGeocode(api huma.API, h *GeocodeHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "geocode", Method: http.MethodGet, Path: "/geocode",
		Summary: "Search towns/places by name", Tags: []string{"Geocode"},
	}, h.search)
}

// search answers from the cache or runs one shared lookup. Followers share
// the leader's lookup, so it runs without the leader's cancellation; maxWait
// and the client timeout still bound it.
func (h *GeocodeHandler) search(ctx context.Context, in *geocodeInput) (*geocodeOutput, error) {
	key := geocodeCacheKey(in.Query, in.bias)
	if cached, ok := h.cache.get(key); ok {
		return &geocodeOutput{Body: cached}, nil
	}
	lookupCtx := context.WithoutCancel(ctx)
	v, err, _ := h.inflight.Do(key, func() (any, error) {
		return h.lookup(lookupCtx, key, trimGeocodeQuery(in.Query), in.bias)
	})
	if err != nil {
		return nil, err
	}
	results, _ := v.([]dto.GeocodeResult)
	return &geocodeOutput{Body: results}, nil
}

// lookup asks Nominatim, then Photon when Nominatim finds no town or fails.
// A busy answer (a limiter queue past maxWait or an upstream 429) is returned
// as 503 at once: falling back would only move the load to Photon. A lookup
// that finished just before this one joined the flight has already filled
// the cache. When the Photon fallback fails, its error is returned (503 when
// it is busy, else 502) rather than an empty list the user would read as
// "no such town"; nothing is cached, so the next try reaches Photon again.
func (h *GeocodeHandler) lookup(ctx context.Context, key, query string, bias *geocodeBias) ([]dto.GeocodeResult, error) {
	if cached, ok := h.cache.get(key); ok {
		return cached, nil
	}
	results, err := h.searchNominatim(ctx, query, bias)
	if err == nil && len(results) > 0 {
		h.cache.put(key, results)
		return results, nil
	}
	if isGeocodeBusy(err) {
		return nil, err
	}
	if h.photon.baseURL == "" {
		if err != nil {
			return nil, err
		}
		h.cache.put(key, results)
		return results, nil
	}
	fallback, perr := h.searchPhoton(ctx, query, bias)
	if perr != nil {
		return nil, perr
	}
	h.cache.put(key, fallback)
	return fallback, nil
}

func geocodeBusy() error {
	return huma.Error503ServiceUnavailable("geocoding service busy, try again")
}

func isGeocodeBusy(err error) bool {
	var se huma.StatusError
	return errors.As(err, &se) && se.GetStatus() == http.StatusServiceUnavailable
}

// fetch waits for the upstream's limiter, then GETs its base URL with params
// and decodes the JSON answer into out. It maps a 429 to 503 busy and any
// other failure to 502.
func (h *GeocodeHandler) fetch(ctx context.Context, up geocodeUpstream, params url.Values, out any) error {
	waitCtx, cancel := context.WithTimeout(ctx, h.maxWait)
	err := up.limiter.Wait(waitCtx)
	cancel()
	if err != nil {
		slog.Warn("geocode rate limited", "service", up.name, "query", params.Get("q"), "err", err)
		return geocodeBusy()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, up.baseURL+"?"+params.Encode(), http.NoBody)
	if err != nil {
		slog.Error("geocode build request", "service", up.name, "err", err)
		return huma.Error500InternalServerError("failed to search places")
	}
	req.Header.Set("User-Agent", geocodeUserAgent)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Error("geocode request", "service", up.name, "query", params.Get("q"), "err", err)
		return huma.Error502BadGateway("geocoding service unavailable")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		slog.Warn("geocode upstream throttled", "service", up.name, "query", params.Get("q"))
		return geocodeBusy()
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("geocode upstream status", "service", up.name, "query", params.Get("q"), "status", resp.StatusCode)
		return huma.Error502BadGateway("geocoding service unavailable")
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		slog.Error("geocode decode", "service", up.name, "query", params.Get("q"), "err", err)
		return huma.Error502BadGateway("geocoding service returned invalid data")
	}
	return nil
}

// searchNominatim asks for names in Polish, else English, else the local
// name, so "Lefkada" is not stored as "Λευκάδα". A bias becomes a viewbox
// that ranks nearby towns first without excluding the rest (bounded=0).
// Nominatim returns matches in descending importance; the filter keeps that
// order, so the best match stays first.
func (h *GeocodeHandler) searchNominatim(ctx context.Context, query string, bias *geocodeBias) ([]dto.GeocodeResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "jsonv2")
	params.Set("limit", strconv.Itoa(geocodeFetchLimit))
	params.Set("addressdetails", "1")
	params.Set("accept-language", "pl,en")
	if bias != nil {
		params.Set("viewbox", bias.viewbox())
		params.Set("bounded", "0")
	}
	var raw []nominatimResult
	if err := h.fetch(ctx, h.nominatim, params, &raw); err != nil {
		return nil, err
	}

	var found geocodeCandidates
	for _, r := range raw {
		if !isNominatimSettlement(r) {
			continue
		}
		lat, lon, ok := parseCoordinates(r.Lat, r.Lon)
		if !ok {
			continue
		}
		name := r.Name
		if name == "" {
			name = r.DisplayName
		}
		label := r.DisplayName
		if r.Address != nil || label == "" {
			label = geocodeLabel(name, r.Address["county"], r.Address["state"], r.Address["country"])
		}
		fullPath := r.DisplayName
		if fullPath == "" {
			fullPath = label
		}
		found.add(dto.GeocodeResult{
			Name: name, Label: label, Latitude: lat, Longitude: lon, Source: geocodeSourceNominatim,
		}, fullPath)
		if found.full() {
			break
		}
	}
	return found.results(), nil
}

// searchPhoton is the fallback for prefixes and typos ("Jastarn",
// "Svolvar") that Nominatim's whole-word matching misses. Photon has no
// Polish names, so it asks for English ones, which fall back to the local
// name.
func (h *GeocodeHandler) searchPhoton(ctx context.Context, query string, bias *geocodeBias) ([]dto.GeocodeResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(photonFetchLimit))
	params.Set("lang", "en")
	params.Set("osm_tag", "place")
	if bias != nil {
		params.Set("lat", formatCoord(bias.lat))
		params.Set("lon", formatCoord(bias.lon))
		params.Set("zoom", photonBiasZoom)
	}
	var raw photonResponse
	if err := h.fetch(ctx, h.photon, params, &raw); err != nil {
		return nil, err
	}

	var found geocodeCandidates
	for _, f := range raw.Features {
		p := f.Properties
		if p.OSMKey != "place" || !settlementTypes[p.OSMValue] || p.Name == "" || len(f.Geometry.Coordinates) < 2 {
			continue
		}
		lat, lon := f.Geometry.Coordinates[1], f.Geometry.Coordinates[0]
		if !validCoordinates(lat, lon) {
			continue
		}
		label := geocodeLabel(p.Name, p.County, p.State, p.Country)
		found.add(dto.GeocodeResult{
			Name: p.Name, Label: label, Latitude: lat, Longitude: lon, Source: geocodeSourcePhoton,
		}, label)
		if found.full() {
			break
		}
	}
	return found.results(), nil
}

// geocodeLabel joins a town name with its county, region and country,
// skipping empty parts and parts that repeat the one before (a city that is
// its own county).
func geocodeLabel(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || (len(out) > 0 && foldPlaceName(out[len(out)-1]) == foldPlaceName(p)) {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

// geocodeCandidates collects results in rank order and drops duplicates:
// both upstreams list one town several times, as a place node and as an
// administrative boundary, whose coordinates differ. A later result is a
// duplicate of a kept one when their folded name and full path match, or
// when their folded names match and they lie within geocodeDupRadiusKm.
type geocodeCandidates struct {
	kept []geocodeCandidate
}

type geocodeCandidate struct {
	result dto.GeocodeResult
	name   string
	path   string
}

func (c *geocodeCandidates) add(r dto.GeocodeResult, fullPath string) {
	cand := geocodeCandidate{result: r, name: foldPlaceName(r.Name), path: foldPlaceName(fullPath)}
	for _, k := range c.kept {
		if k.name != cand.name {
			continue
		}
		if k.path == cand.path || distanceKm(k.result.Latitude, k.result.Longitude, r.Latitude, r.Longitude) <= geocodeDupRadiusKm {
			return
		}
	}
	c.kept = append(c.kept, cand)
}

func (c *geocodeCandidates) full() bool { return len(c.kept) >= geocodeMaxResults }

// results returns a non-nil slice so an empty answer encodes as [].
func (c *geocodeCandidates) results() []dto.GeocodeResult {
	out := make([]dto.GeocodeResult, len(c.kept))
	for i, k := range c.kept {
		out[i] = k.result
	}
	return out
}

// placeFolds spells out letters that Unicode does not decompose into a base
// letter and a mark, so "Łeba" folds like "Leba" and "Svolvær" like
// "Svolvaer", as PostgreSQL's unaccent does.
var placeFolds = strings.NewReplacer(
	"ł", "l", "đ", "d", "ø", "o", "æ", "ae", "œ", "oe", "ß", "ss", "þ", "th", "ð", "d", "ı", "i",
)

// foldPlaceName lowercases a name, strips its accents and collapses its
// blanks, so spellings that differ only in case or diacritics compare equal.
func foldPlaceName(s string) string {
	s = placeFolds.Replace(strings.ToLower(s))
	stripped, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	if err == nil {
		s = stripped
	}
	return strings.Join(strings.FieldsFunc(s, isBlankGeocodeRune), " ")
}

// distanceKm is the great-circle distance between two points.
func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

// parseCoordinates parses a Nominatim lat/lon pair and reports whether it is
// a usable position.
func parseCoordinates(rawLat, rawLon string) (lat, lon float64, ok bool) {
	lat, latErr := strconv.ParseFloat(rawLat, 64)
	lon, lonErr := strconv.ParseFloat(rawLon, 64)
	if latErr != nil || lonErr != nil || !validCoordinates(lat, lon) {
		return 0, 0, false
	}
	return lat, lon, true
}

// validCoordinates rejects NaN, which the JSON encoder cannot write, so a
// record carrying one would break the whole response. ±Inf also fails the
// range check, which drops out-of-range positions from a broken upstream.
func validCoordinates(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180
}
