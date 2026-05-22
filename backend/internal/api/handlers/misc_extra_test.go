package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestQueryError(t *testing.T) {
	inner := errors.New("connection reset")
	qe := &QueryError{Op: "GetTrip", Err: inner}

	if qe.Error() != "query GetTrip failed: connection reset" {
		t.Fatalf("unexpected message: %q", qe.Error())
	}
	if !errors.Is(qe, inner) {
		t.Fatalf("Unwrap must expose the inner error")
	}
}

func TestValOrZero(t *testing.T) {
	f := 3.5
	if valOrZeroFloat(&f) != 3.5 {
		t.Fatalf("valOrZeroFloat(&3.5) = %v", valOrZeroFloat(&f))
	}
	if valOrZeroFloat(nil) != 0 {
		t.Fatalf("valOrZeroFloat(nil) must be 0")
	}

	i := int64(7)
	if valOrZeroInt(&i) != 7 {
		t.Fatalf("valOrZeroInt(&7) = %v", valOrZeroInt(&i))
	}
	if valOrZeroInt(nil) != 0 {
		t.Fatalf("valOrZeroInt(nil) must be 0")
	}
}

func TestRegisterGeocodeRoutes(t *testing.T) {
	_, api := humatest.New(t)
	RegisterGeocodeRoutes(api)
	// A short query fails minLength validation, proving the route is wired.
	resp := api.GetCtx(userCtx(context.Background()), "/geocode?q=a")
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", resp.Code)
	}
}

func TestNewGeocodeHandler(t *testing.T) {
	h := NewGeocodeHandler()
	if h.baseURL != nominatimBaseURL || h.httpClient == nil {
		t.Fatalf("unexpected handler: %+v", h)
	}
}

func TestTripHandler_Complete_MemberForbidden(t *testing.T) {
	_, api := humatest.New(t)
	RegisterTripRoutes(api, &mockQuerier{}, nil)
	resp := api.PostCtx(userCtxRole(context.Background(), "member"), "/trips/1/complete", map[string]any{})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", resp.Code, resp.Body)
	}
}
