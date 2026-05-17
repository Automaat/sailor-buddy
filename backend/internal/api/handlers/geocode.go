package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
)

const nominatimBaseURL = "https://nominatim.openstreetmap.org/search"

// GeocodeHandler proxies town-name lookups to an OpenStreetMap Nominatim
// instance. The base URL and HTTP client are injectable so tests can stub the
// upstream service.
type GeocodeHandler struct {
	baseURL    string
	httpClient *http.Client
}

func NewGeocodeHandler() *GeocodeHandler {
	return &GeocodeHandler{
		baseURL:    nominatimBaseURL,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

type geocodeInput struct {
	Query string `query:"q" minLength:"2" doc:"Town or place name to search for"`
}

type geocodeOutput struct {
	Body []dto.GeocodeResult
}

// nominatimResult is the subset of a Nominatim jsonv2 record we consume.
type nominatimResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Class       string `json:"class"`
}

// placeClasses is the set of Nominatim feature classes a sailing destination
// can plausibly belong to: populated places and the administrative areas that
// share their name. Streets, buildings and POIs are dropped as noise.
var placeClasses = map[string]bool{"place": true, "boundary": true, "natural": true}

// RegisterGeocodeRoutes wires the geocoding proxy onto the API.
func RegisterGeocodeRoutes(api huma.API) {
	h := NewGeocodeHandler()
	huma.Register(api, huma.Operation{
		OperationID: "geocode", Method: http.MethodGet, Path: "/geocode",
		Summary: "Search towns/places by name", Tags: []string{"Geocode"},
	}, h.search)
}

func (h *GeocodeHandler) search(ctx context.Context, in *geocodeInput) (*geocodeOutput, error) {
	q := url.Values{}
	q.Set("q", in.Query)
	q.Set("format", "jsonv2")
	// Over-fetch: class filtering and coordinate dedup below trim the list,
	// so a higher limit keeps ~5 usable matches.
	q.Set("limit", "10")
	q.Set("addressdetails", "0")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"?"+q.Encode(), http.NoBody)
	if err != nil {
		slog.Error("geocode build request", "query", in.Query, "err", err)
		return nil, huma.Error500InternalServerError("failed to search places")
	}
	// Nominatim's usage policy requires a descriptive User-Agent that
	// identifies the application and a way to reach its operator.
	req.Header.Set("User-Agent", "sailor-buddy/1.0 (+https://github.com/Automaat/sailor-buddy)")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Error("geocode request", "query", in.Query, "err", err)
		return nil, huma.Error502BadGateway("geocoding service unavailable")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Error("geocode upstream status", "query", in.Query, "status", resp.StatusCode)
		return nil, huma.Error502BadGateway("geocoding service unavailable")
	}

	var raw []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		slog.Error("geocode decode", "query", in.Query, "err", err)
		return nil, huma.Error502BadGateway("geocoding service returned invalid data")
	}

	// Nominatim returns matches in descending importance order; the filtering
	// loop preserves that, so the best match stays first.
	results := make([]dto.GeocodeResult, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, r := range raw {
		// Drop streets, buildings and POIs — only towns and the admin areas
		// that share their name make sense as a sailing destination.
		if !placeClasses[r.Class] {
			continue
		}
		lat, latErr := strconv.ParseFloat(r.Lat, 64)
		lon, lonErr := strconv.ParseFloat(r.Lon, 64)
		if latErr != nil || lonErr != nil {
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
	return &geocodeOutput{Body: results}, nil
}
