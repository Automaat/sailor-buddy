package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

const recordCaptainID = 7

func captainVoyage(_ context.Context, id int64) (sqlcdb.Voyage, error) {
	return sqlcdb.Voyage{ID: id, CaptainUserID: types.NullInt64{Int64: recordCaptainID, Valid: true}}, nil
}

func TestVoyageLog_RecordFields(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want int
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), want: http.StatusNoContent},
		{name: "assigned captain", ctx: callerCtx(recordCaptainID, "member"), want: http.StatusNoContent},
		{name: "other member", ctx: callerCtx(8, "member"), want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *sqlcdb.UpdateVoyageLogParams
			m := &mockQuerier{
				getVoyageFn: captainVoyage,
				updateVoyageLogFn: func(_ context.Context, arg sqlcdb.UpdateVoyageLogParams) error {
					got = &arg
					return nil
				},
			}
			resp := voyageTestAPI(t, m).PutCtx(tt.ctx, "/voyages/5/log", map[string]any{
				"yacht_id":    3,
				"track_url":   "https://www.navionics.com/track/123",
				"voyage_card": "u kpt. Nowaka",
			})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusNoContent {
				if got != nil {
					t.Fatal("log written despite rejection")
				}
				return
			}
			if got == nil {
				t.Fatal("no write recorded")
			}
			if got.YachtID != (types.NullInt64{Int64: 3, Valid: true}) ||
				got.TrackUrl.String != "https://www.navionics.com/track/123" ||
				got.VoyageCard.String != "u kpt. Nowaka" {
				t.Fatalf("unexpected log params: %+v", got)
			}
		})
	}
}

func TestVoyageLog_ClearsEmptyRecordFields(t *testing.T) {
	var got *sqlcdb.UpdateVoyageLogParams
	m := &mockQuerier{
		getVoyageFn: captainVoyage,
		updateVoyageLogFn: func(_ context.Context, arg sqlcdb.UpdateVoyageLogParams) error {
			got = &arg
			return nil
		},
	}
	resp := voyageTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/voyages/5/log", map[string]any{
		"track_url": "  ", "voyage_card": "",
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d; body=%s", resp.Code, resp.Body)
	}
	if got == nil {
		t.Fatal("no write recorded")
	}
	if got.TrackUrl.Valid || got.VoyageCard.Valid || got.YachtID.Valid {
		t.Fatalf("want NULL fields, got %+v", got)
	}
}

func TestVoyageTrackURL_Validation(t *testing.T) {
	tests := []struct {
		url  string
		want int
	}{
		{url: "https://example.com/track", want: http.StatusNoContent},
		{url: "http://example.com/t?id=1", want: http.StatusNoContent},
		{url: "javascript:alert(1)", want: http.StatusUnprocessableEntity},
		{url: "ftp://example.com/track.gpx", want: http.StatusUnprocessableEntity},
		{url: "example.com/track", want: http.StatusUnprocessableEntity},
		{url: "/relative/path", want: http.StatusUnprocessableEntity},
		{url: "https://", want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			written := false
			m := &mockQuerier{
				getVoyageFn: captainVoyage,
				updateVoyageLogFn: func(context.Context, sqlcdb.UpdateVoyageLogParams) error {
					written = true
					return nil
				},
				getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
					return sqlcdb.Cruise{ID: id}, nil
				},
				updateVoyageFn: func(context.Context, sqlcdb.UpdateVoyageParams) error {
					written = true
					return nil
				},
			}
			api := voyageTestAPI(t, m)
			resp := api.PutCtx(callerCtx(recordCaptainID, "member"), "/voyages/5/log", map[string]any{"track_url": tt.url})
			if resp.Code != tt.want {
				t.Fatalf("log: status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			resp = api.PutCtx(callerCtx(1, "admin"), "/voyages/5", map[string]any{
				"name": "Bałtyk", "cruise_id": 2, "track_url": tt.url,
			})
			if resp.Code != tt.want {
				t.Fatalf("update: status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusNoContent && written {
				t.Fatal("voyage written despite an invalid track link")
			}
		})
	}
}

func TestVoyageCreate_StoresRecordFields(t *testing.T) {
	var got *sqlcdb.CreateVoyageParams
	m := &mockQuerier{
		getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
			return sqlcdb.Cruise{ID: id}, nil
		},
		createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
			got = &arg
			return sqlcdb.Voyage{ID: 9, CruiseID: arg.CruiseID, Name: arg.Name, TrackUrl: arg.TrackUrl, VoyageCard: arg.VoyageCard}, nil
		},
	}
	resp := voyageTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/voyages", map[string]any{
		"name": "Bałtyk", "cruise_id": 2, "track_url": "https://example.com/t", "voyage_card": "w klubie",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", resp.Code, resp.Body)
	}
	if got == nil {
		t.Fatal("no write recorded")
	}
	if got.TrackUrl.String != "https://example.com/t" || got.VoyageCard.String != "w klubie" {
		t.Fatalf("unexpected params: %+v", got)
	}
	var v dto.Voyage
	if err := json.Unmarshal(resp.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v.TrackUrl == nil || *v.TrackUrl != "https://example.com/t" || v.VoyageCard == nil || *v.VoyageCard != "w klubie" {
		t.Fatalf("unexpected voyage: %+v", v)
	}
}

func TestVoyageLog_UnknownYacht(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: captainVoyage,
		updateVoyageLogFn: func(context.Context, sqlcdb.UpdateVoyageLogParams) error {
			return &pgconn.PgError{Code: "23503", ConstraintName: "voyages_yacht_id_fkey"}
		},
	}
	resp := voyageTestAPI(t, m).PutCtx(callerCtx(recordCaptainID, "member"), "/voyages/5/log", map[string]any{"yacht_id": 999})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", resp.Code, resp.Body)
	}
}

func TestVoyageAddYacht_Authorization(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		voyage error
		want   int
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), want: http.StatusCreated},
		{name: "assigned captain", ctx: callerCtx(recordCaptainID, "member"), want: http.StatusCreated},
		{name: "other member", ctx: callerCtx(8, "member"), want: http.StatusForbidden},
		{name: "missing voyage", ctx: callerCtx(1, "admin"), voyage: sql.ErrNoRows, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *sqlcdb.CreateYachtParams
			m := &mockQuerier{
				getVoyageFn: func(ctx context.Context, id int64) (sqlcdb.Voyage, error) {
					if tt.voyage != nil {
						return sqlcdb.Voyage{}, tt.voyage
					}
					return captainVoyage(ctx, id)
				},
				createYachtFn: func(_ context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
					got = &arg
					return sqlcdb.Yacht{ID: 11, Name: arg.Name, YachtType: arg.YachtType, RegistrationNo: arg.RegistrationNo}, nil
				},
			}
			resp := voyageTestAPI(t, m).PostCtx(tt.ctx, "/voyages/5/yachts", map[string]any{
				"name": "Zawisza", "yacht_type": "Bavaria 46",
			})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusCreated {
				if got != nil {
					t.Fatal("yacht created despite rejection")
				}
				return
			}
			if got == nil {
				t.Fatal("no write recorded")
			}
			if got.Name != "Zawisza" || got.YachtType.String != "Bavaria 46" || got.RegistrationNo.Valid {
				t.Fatalf("unexpected yacht params: %+v", got)
			}
			var y dto.Yacht
			if err := json.Unmarshal(resp.Body.Bytes(), &y); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if y.ID != 11 || y.Name != "Zawisza" {
				t.Fatalf("unexpected yacht: %+v", y)
			}
		})
	}
}

func TestVoyagePorts_AddNameOnlyAnchorage(t *testing.T) {
	var got *sqlcdb.CreateVoyagePortParams
	m := &mockQuerier{
		getVoyageFn: captainVoyage,
		createVoyagePortFn: func(_ context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
			got = &arg
			return sqlcdb.VoyagePort{
				ID: 9, VoyageID: arg.VoyageID, Name: arg.Name, Latitude: arg.Latitude,
				Longitude: arg.Longitude, Anchorage: arg.Anchorage, Position: arg.Position,
			}, nil
		},
	}
	resp := voyagePortTestAPI(t, m).PostCtx(callerCtx(recordCaptainID, "member"), "/voyages/5/ports",
		map[string]any{"name": "Zatoka Puck", "anchorage": true, "position": 2})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	if got == nil {
		t.Fatal("no write recorded")
	}
	if got.Latitude.Valid || got.Longitude.Valid || !got.Anchorage || got.Position != 2 {
		t.Fatalf("unexpected port params: %+v", got)
	}
	var raw map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["latitude"]; ok {
		t.Fatalf("name-only port must omit latitude: %s", resp.Body)
	}
	if raw["anchorage"] != true {
		t.Fatalf("anchorage not returned: %s", resp.Body)
	}
}

func TestVoyagePorts_RejectsHalfCoordinates(t *testing.T) {
	bodies := []map[string]any{
		{"name": "Hel", "latitude": 54.6},
		{"name": "Hel", "longitude": 18.8},
	}
	for _, body := range bodies {
		created := false
		m := &mockQuerier{
			getVoyageFn: captainVoyage,
			createVoyagePortFn: func(context.Context, sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
				created = true
				return sqlcdb.VoyagePort{}, nil
			},
		}
		resp := voyagePortTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/voyages/5/ports", body)
		if resp.Code != http.StatusUnprocessableEntity || created {
			t.Fatalf("body %v: status = %d, created = %v; body=%s", body, resp.Code, created, resp.Body)
		}
	}
}

func TestTripComplete_RejectsHalfCoordinates(t *testing.T) {
	resp := tripTestAPI(t, &mockQuerier{}).PostCtx(callerCtx(1, "admin"), "/trips/1/complete", map[string]any{
		"ports": []map[string]any{{"name": "Hel"}, {"name": "Jastarnia", "latitude": 54.7}},
	})
	if resp.Code != http.StatusUnprocessableEntity || !strings.Contains(resp.Body.String(), "latitude and longitude") {
		t.Fatalf("status = %d, want 422 for paired coordinates; body=%s", resp.Code, resp.Body)
	}
}
