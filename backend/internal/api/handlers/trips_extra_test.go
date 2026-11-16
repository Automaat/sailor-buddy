package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestTripHandler_List_Errors(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		m := &mockQuerier{
			listTripsFn: func(context.Context, sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) {
				return nil, errors.New("fail")
			},
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("count error", func(t *testing.T) {
		m := &mockQuerier{
			listTripsFn:  func(context.Context, sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) { return nil, nil },
			countTripsFn: func(context.Context) (int64, error) { return 0, errors.New("fail") },
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTripHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{ID: id, Name: "Adriatic", Status: sqlcdb.TripStatusPlanned}, nil
			},
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, errors.New("fail")
			},
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTripGet_YachtSummary(t *testing.T) {
	yachtType := types.NullString{String: "Bavaria 46", Valid: true}
	yachtID := types.NullInt64{Int64: 3, Valid: true}
	tests := []struct {
		name      string
		yachtID   types.NullInt64
		yachtType types.NullString
		yacht     error
		want      int
		summary   *dto.YachtSummary
	}{
		{name: "no yacht", want: http.StatusOK},
		{
			name: "yacht with type", yachtID: yachtID, yachtType: yachtType, want: http.StatusOK,
			summary: &dto.YachtSummary{ID: 3, Name: "Zawisza", YachtType: &yachtType.String},
		},
		{
			name: "yacht without type", yachtID: yachtID, want: http.StatusOK,
			summary: &dto.YachtSummary{ID: 3, Name: "Zawisza"},
		},
		{name: "yacht deleted meanwhile", yachtID: yachtID, yacht: sql.ErrNoRows, want: http.StatusOK},
		{name: "yacht read fails", yachtID: yachtID, yacht: errors.New("fail"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{
						ID: id, Name: "Adriatic", Status: sqlcdb.TripStatusPlanned,
						YachtID: tt.yachtID, PublishedAt: published,
					}, nil
				},
				getYachtFn: func(_ context.Context, id int64) (sqlcdb.Yacht, error) {
					if tt.yacht != nil {
						return sqlcdb.Yacht{}, tt.yacht
					}
					return sqlcdb.Yacht{
						ID: id, Name: "Zawisza", YachtType: tt.yachtType,
						OwnerName: types.NullString{String: "Armator", Valid: true},
					}, nil
				},
			}
			resp := tripTestAPI(t, m).GetCtx(callerCtx(8, "member"), "/trips/5")
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusOK {
				return
			}
			var got dto.TripDetail
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.ID != 5 || got.Name != "Adriatic" {
				t.Fatalf("unexpected trip: %+v", got.Trip)
			}
			if !reflect.DeepEqual(got.Yacht, tt.summary) {
				t.Fatalf("yacht = %+v, want %+v", got.Yacht, tt.summary)
			}
			if tt.summary == nil && strings.Contains(resp.Body.String(), `"yacht":`) {
				t.Fatalf("yacht present without a summary: %s", resp.Body)
			}
			if strings.Contains(resp.Body.String(), "Armator") {
				t.Fatalf("summary leaks yacht details: %s", resp.Body)
			}
		})
	}
}

func TestTripHandler_Create_DBError(t *testing.T) {
	m := &mockQuerier{
		createTripFn: func(context.Context, sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{}, errors.New("fail")
		},
	}
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", map[string]any{"cruise_id": 1, "name": "X"})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestTripHandler_Update_DBError(t *testing.T) {
	m := &mockQuerier{
		getTripFn:    func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id, CruiseID: 1}, nil },
		updateTripFn: func(context.Context, sqlcdb.UpdateTripParams) error { return errors.New("fail") },
	}
	resp := tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/1", map[string]any{"cruise_id": 1, "name": "X"})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestTripHandler_Update_MemberForbidden(t *testing.T) {
	resp := tripTestAPI(t, &mockQuerier{}).PutCtx(
		userCtxRole(context.Background(), "member"), "/trips/1", map[string]any{"cruise_id": 1, "name": "X"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", resp.Code)
	}
}

func TestTripHandler_Delete_DBError(t *testing.T) {
	m := &mockQuerier{deleteTripFn: func(context.Context, int64) error { return errors.New("fail") }}
	resp := tripTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/1")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestTripHandler_Cancel_DBError(t *testing.T) {
	m := &mockQuerier{
		cancelTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{}, errors.New("fail")
		},
	}
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/1/cancel")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestNewTripHandler(t *testing.T) {
	if NewTripHandler(&mockQuerier{}, nil) == nil {
		t.Fatal("want non-nil handler")
	}
}
