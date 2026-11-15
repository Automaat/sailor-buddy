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

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

const recordCaptainID = 7

const recordCaptainPersonID = 70

func captainVoyage(_ context.Context, id int64) (sqlcdb.Voyage, error) {
	return sqlcdb.Voyage{ID: id, CaptainPersonID: types.NullInt64{Int64: recordCaptainPersonID, Valid: true}}, nil
}

func recordCaptain(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
	if id != recordCaptainPersonID {
		return sqlcdb.CrewMember{}, sql.ErrNoRows
	}
	return sqlcdb.CrewMember{ID: id, UserID: types.NullInt64{Int64: recordCaptainID, Valid: true}}, nil
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
				getVoyageFn:     captainVoyage,
				getCrewMemberFn: recordCaptain,
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
		getVoyageFn:     captainVoyage,
		getCrewMemberFn: recordCaptain,
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
				getVoyageFn:     captainVoyage,
				getCrewMemberFn: recordCaptain,
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
		getVoyageFn:     captainVoyage,
		getCrewMemberFn: recordCaptain,
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
				getCrewMemberFn: recordCaptain,
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
		getVoyageFn:     captainVoyage,
		getCrewMemberFn: recordCaptain,
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
			getVoyageFn:     captainVoyage,
			getCrewMemberFn: recordCaptain,
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

func TestVoyageListYachts_Authorization(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		voyage error
		want   int
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), want: http.StatusOK},
		{name: "assigned captain outside the captain group", ctx: callerCtx(recordCaptainID, "member"), want: http.StatusOK},
		{name: "other member", ctx: callerCtx(8, "member"), want: http.StatusForbidden},
		{name: "captain group, not this voyage's captain", ctx: captainGroupCtx(9), want: http.StatusForbidden},
		{name: "missing voyage", ctx: callerCtx(1, "admin"), voyage: sql.ErrNoRows, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *sqlcdb.ListYachtsParams
			m := &mockQuerier{
				getVoyageFn: func(ctx context.Context, id int64) (sqlcdb.Voyage, error) {
					if tt.voyage != nil {
						return sqlcdb.Voyage{}, tt.voyage
					}
					return captainVoyage(ctx, id)
				},
				getCrewMemberFn: recordCaptain,
				listYachtsFn: func(_ context.Context, arg sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) {
					got = &arg
					return []sqlcdb.Yacht{{ID: 3, Name: "Zawisza"}}, nil
				},
				countYachtsFn: func(context.Context) (int64, error) { return 1, nil },
			}
			resp := voyageTestAPI(t, m).GetCtx(tt.ctx, "/voyages/5/yachts?limit=10")
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusOK {
				if got != nil {
					t.Fatal("yachts listed despite rejection")
				}
				return
			}
			if got == nil || got.RowLimit != 11 {
				t.Fatalf("list args = %+v, want one row past a 10-row window", got)
			}
			var page dto.Page[dto.Yacht]
			if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "Zawisza" {
				t.Fatalf("unexpected page: %+v", page)
			}
		})
	}
}

func TestVoyageGet_YachtSummary(t *testing.T) {
	yachtType := types.NullString{String: "Bavaria 46", Valid: true}
	tests := []struct {
		name    string
		yachtID types.NullInt64
		yacht   error
		want    int
		summary *dto.YachtSummary
	}{
		{name: "no yacht", want: http.StatusOK},
		{
			name: "yacht", yachtID: types.NullInt64{Int64: 3, Valid: true}, want: http.StatusOK,
			summary: &dto.YachtSummary{ID: 3, Name: "Zawisza", YachtType: &yachtType.String},
		},
		{name: "yacht deleted meanwhile", yachtID: types.NullInt64{Int64: 3, Valid: true}, yacht: sql.ErrNoRows, want: http.StatusOK},
		{name: "yacht read fails", yachtID: types.NullInt64{Int64: 3, Valid: true}, yacht: errors.New("fail"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
					return sqlcdb.Voyage{ID: id, Name: "Baltic", YachtID: tt.yachtID, PublishedAt: published}, nil
				},
				getYachtFn: func(_ context.Context, id int64) (sqlcdb.Yacht, error) {
					if tt.yacht != nil {
						return sqlcdb.Yacht{}, tt.yacht
					}
					return sqlcdb.Yacht{
						ID: id, Name: "Zawisza", YachtType: yachtType,
						OwnerName: types.NullString{String: "Armator", Valid: true},
					}, nil
				},
			}
			resp := voyageTestAPI(t, m).GetCtx(callerCtx(8, "member"), "/voyages/5")
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusOK {
				return
			}
			var v dto.VoyageDetail
			if err := json.Unmarshal(resp.Body.Bytes(), &v); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if v.ID != 5 || v.Name != "Baltic" {
				t.Fatalf("unexpected voyage: %+v", v.Voyage)
			}
			if !reflect.DeepEqual(v.Yacht, tt.summary) {
				t.Fatalf("yacht = %+v, want %+v", v.Yacht, tt.summary)
			}
			if strings.Contains(resp.Body.String(), "Armator") {
				t.Fatalf("summary leaks yacht details: %s", resp.Body)
			}
		})
	}
}
