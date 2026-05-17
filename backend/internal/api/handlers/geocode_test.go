package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubNominatim returns a test server serving the given raw JSON body.
func stubNominatim(t *testing.T, status int, body string) *GeocodeHandler {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &GeocodeHandler{baseURL: srv.URL, httpClient: srv.Client()}
}

func TestGeocode_Search(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[
		{"name":"Split","display_name":"Split, Croatia","lat":"43.5081","lon":"16.4402","class":"place"},
		{"name":"","display_name":"Hvar, Croatia","lat":"43.1729","lon":"16.4412","class":"boundary"},
		{"name":"Bad","display_name":"Bad","lat":"not-a-number","lon":"0","class":"place"}
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

// TestGeocode_FiltersAndDedups checks that non-place classes (streets, POIs)
// are dropped and that a town listed as both a place node and an admin
// boundary collapses to one result.
func TestGeocode_FiltersAndDedups(t *testing.T) {
	h := stubNominatim(t, http.StatusOK, `[
		{"name":"Gdańsk","display_name":"Gdańsk, Poland","lat":"54.3520","lon":"18.6466","class":"place"},
		{"name":"Gdańsk","display_name":"Gdańsk, Pomerania, Poland","lat":"54.3521","lon":"18.6467","class":"boundary"},
		{"name":"Gdańska","display_name":"Gdańska street","lat":"52.2300","lon":"21.0100","class":"highway"},
		{"name":"Gdansk","display_name":"Gdansk, North Dakota, USA","lat":"48.4100","lon":"-98.3700","class":"place"}
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
