package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCruiseHandler_Update_InvalidJSON(t *testing.T) {
	h := NewCruiseHandler(&mockQuerier{})
	req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestYachtHandler_Update_InvalidJSON(t *testing.T) {
	h := NewYachtHandler(&mockQuerier{})
	req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTrainingHandler_Update_InvalidJSON(t *testing.T) {
	h := NewTrainingHandler(&mockQuerier{})
	req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCrewHandler_Update_InvalidJSON(t *testing.T) {
	h := NewCrewHandler(&mockQuerier{})
	req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

