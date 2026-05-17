package handlers

import (
	"context"
	"encoding/json"
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
}

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
	q.Set("limit", "5")
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

	results := make([]dto.GeocodeResult, 0, len(raw))
	for _, r := range raw {
		lat, latErr := strconv.ParseFloat(r.Lat, 64)
		lon, lonErr := strconv.ParseFloat(r.Lon, 64)
		if latErr != nil || lonErr != nil {
			continue
		}
		name := r.Name
		if name == "" {
			name = r.DisplayName
		}
		results = append(results, dto.GeocodeResult{Name: name, Latitude: lat, Longitude: lon})
	}
	return &geocodeOutput{Body: results}, nil
}
