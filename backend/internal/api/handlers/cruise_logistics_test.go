package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type cruiseLogistics struct {
	GroupChatURL *string `json:"group_chat_url"`
	TravelNotes  *string `json:"travel_notes"`
	MinAge       *int64  `json:"min_age"`
}

func TestCruiseHandler_Logistics(t *testing.T) {
	full := map[string]any{
		"name":           "Sycylia 2026",
		"group_chat_url": "https://discord.gg/sycylia",
		"travel_notes":   "Tanie loty do Katanii",
		"min_age":        18,
	}

	t.Run("create stores and returns all three", func(t *testing.T) {
		var stored sqlcdb.CreateCruiseParams
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				stored = arg
				return sqlcdb.Cruise{
					ID: 1, Name: arg.Name, Kind: arg.Kind,
					GroupChatUrl: arg.GroupChatUrl, TravelNotes: arg.TravelNotes, MinAge: arg.MinAge,
				}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", full)
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		if stored.GroupChatUrl != (types.NullString{String: "https://discord.gg/sycylia", Valid: true}) ||
			stored.TravelNotes != (types.NullString{String: "Tanie loty do Katanii", Valid: true}) ||
			stored.MinAge != (types.NullInt64{Int64: 18, Valid: true}) {
			t.Fatalf("stored = %+v", stored)
		}
		var out cruiseLogistics
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.GroupChatURL == nil || *out.GroupChatURL != "https://discord.gg/sycylia" ||
			out.TravelNotes == nil || *out.TravelNotes != "Tanie loty do Katanii" ||
			out.MinAge == nil || *out.MinAge != 18 {
			t.Fatalf("response = %s", resp.Body)
		}
	})

	t.Run("update stores all three and get reads them back", func(t *testing.T) {
		var row sqlcdb.Cruise
		m := &mockQuerier{
			updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
				row = sqlcdb.Cruise{
					ID: arg.ID, Name: arg.Name, Kind: arg.Kind,
					GroupChatUrl: arg.GroupChatUrl, TravelNotes: arg.TravelNotes, MinAge: arg.MinAge,
				}
				return nil
			},
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) { return row, nil },
		}
		api := cruiseTestAPI(t, m)
		ctx := userCtx(context.Background())
		if resp := api.PutCtx(ctx, "/cruises/5", full); resp.Code != http.StatusNoContent {
			t.Fatalf("put: got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		resp := api.GetCtx(userCtxRole(context.Background(), "member"), "/cruises/5")
		if resp.Code != http.StatusOK {
			t.Fatalf("get: got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var out cruiseLogistics
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.GroupChatURL == nil || *out.GroupChatURL != "https://discord.gg/sycylia" ||
			out.TravelNotes == nil || *out.TravelNotes != "Tanie loty do Katanii" ||
			out.MinAge == nil || *out.MinAge != 18 {
			t.Fatalf("response = %s", resp.Body)
		}
	})

	t.Run("blank or omitted fields store null", func(t *testing.T) {
		bodies := []map[string]any{
			{"name": "X"},
			{"name": "X", "group_chat_url": "  ", "travel_notes": "\n"},
		}
		for _, body := range bodies {
			var stored sqlcdb.UpdateCruiseParams
			m := &mockQuerier{
				updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
					stored = arg
					return nil
				},
			}
			resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/1", body)
			if resp.Code != http.StatusNoContent {
				t.Fatalf("body %v: got %d, want 204; body=%s", body, resp.Code, resp.Body)
			}
			if stored.GroupChatUrl.Valid || stored.TravelNotes.Valid || stored.MinAge.Valid {
				t.Fatalf("body %v: stored = %+v, want nulls", body, stored)
			}
		}
	})

	t.Run("http link is accepted and trimmed", func(t *testing.T) {
		var stored types.NullString
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				stored = arg.GroupChatUrl
				return sqlcdb.Cruise{ID: 1, Name: arg.Name, Kind: arg.Kind}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises",
			map[string]any{"name": "X", "group_chat_url": " HTTP://chat.example.com:8443/g/1 "})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		if stored != (types.NullString{String: "HTTP://chat.example.com:8443/g/1", Valid: true}) {
			t.Fatalf("stored = %+v", stored)
		}
	})

	validLinks := []string{
		"https://discord.gg/sycylia",
		"https://chat.example.com:443/g",
		"https://chat.example.com:65535/g",
		"http://192.168.1.10/crew",
		"http://192.168.1.10./crew",
		"http://255.255.255.255:8080/g",
		"http://127.1/g",
		"http://0x7f.0.0.1/g",
		"http://4294967295/g",
		"http://[::1]:8080/g",
		"https://[2001:db8::1]/g",
		"https://chat.example.123abc/g",
		"https://0x7g.example.com/g",
		"http://1.0x/g",
		"https://１２７.0.0.1/g",
		"https://my_chat.example.com/g",
		"https://żagle.pl/czat",
		"https://\U00011F00.example.com/g",
		"https://-chat.example.com/g",
		"https://xn--agle-99a.pl/czat",
		"https://XN--AGLE-99A.pl/czat",
		"https://\u0915\u094d\u200d.example.com/g",
		"https://\u0628\u200c\u0628.example.com/g",
		"https://a\u0301.example.com/g",
		"https://[fe80::1]/g",
		"https://chat.0x10000000000000000g/crew",
		"https://chat.99999999999999999999999a/crew",
	}
	for _, link := range validLinks {
		t.Run("create accepts "+link, func(t *testing.T) {
			var stored types.NullString
			m := &mockQuerier{
				createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
					stored = arg.GroupChatUrl
					return sqlcdb.Cruise{ID: 1, Name: arg.Name, Kind: arg.Kind}, nil
				},
			}
			resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises",
				map[string]any{"name": "X", "group_chat_url": link})
			if resp.Code != http.StatusCreated {
				t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
			}
			if stored != (types.NullString{String: link, Valid: true}) {
				t.Fatalf("stored = %+v", stored)
			}
		})
	}

	invalidLinks := []string{
		"discord.gg/abc",
		"javascript:alert(1)",
		"ftp://example.com/chat",
		"mailto:crew@example.com",
		"https://",
		"//example.com/chat",
		"http://exa mple.com",
		"https://chat.example.com:99999/g",
		"https://chat.example.com:0/g",
		"https://:443/g",
		"https://192.168.1.999/crew",
		"https://1.2.3.4.5/crew",
		"https://chat.123/g",
		"https://256.0.0.1/g",
		"https://1.2.3.256/g",
		"https://1.2.3.08/g",
		"https://4294967296/g",
		"https://0x100000000/g",
		"https://1.0xg.5/g",
		"https://example.0x1g.1/g",
		"https://1.2.3.4.5./g",
		"https://192.168.1.９９９/crew",
		"https://1.2.3.4。5/crew",
		"https://１.２.３.４.５/crew",
		"https://\U00011F00.192.168.1.\uff19\uff19\uff19/crew",
		"https://xn--zz.１.２.３.４.５/crew",
		"https://chat.99999999999999999999999/crew",
		"https://chat.0x10000000000000000/crew",
		"https://1.2.3.4\u200d/crew",
		"https://a\u200cb.example.com/g",
		"https://a\u200db.example.com/g",
		"https://\u0378.example.com/g",
		"https://\ue000.example.com/g",
		"https://\U000F0000.example.com/g",
		"https://\ufffd.example.com/g",
		"https://xn--zz.example.com/g",
		"https://XN--ZZ.example.com/g",
		"https://xn--zva.example.com/g",
		"https://xn--abc-.example.com/g",
		"https://\u017c.xn--abc-.example/g",
		"https://\u017c.xn--.example/g",
		"https://\u017c\u3002xn--.example/g",
		"https://\u017c.\uff58\uff4e--abc-.example/g",
		"https://\u0301abc.example.com/g",
		"https://chat.\u0301abc/g",
		"https://[fe80::1%25eth0]/g",
	}
	for _, link := range invalidLinks {
		t.Run("create rejects "+link, func(t *testing.T) {
			resp := cruiseTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/cruises",
				map[string]any{"name": "X", "group_chat_url": link})
			if resp.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
			}
			if !strings.Contains(resp.Body.String(), "group_chat_url must be an http or https URL") {
				t.Fatalf("body=%s, want group_chat_url message", resp.Body)
			}
		})
		t.Run("update rejects "+link, func(t *testing.T) {
			resp := cruiseTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()), "/cruises/1",
				map[string]any{"name": "X", "group_chat_url": link})
			if resp.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
			}
		})
	}

	for _, age := range []any{-1, 100, 18.5, "18"} {
		t.Run("rejects invalid min_age", func(t *testing.T) {
			resp := cruiseTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/cruises",
				map[string]any{"name": "X", "min_age": age})
			if resp.Code != http.StatusUnprocessableEntity {
				t.Fatalf("min_age %v: got %d, want 422; body=%s", age, resp.Code, resp.Body)
			}
		})
	}

	t.Run("member cannot set logistics", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).PutCtx(userCtxRole(context.Background(), "member"), "/cruises/1", full)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestEnrollment_GetByToken_CruiseLogistics(t *testing.T) {
	newMock := func() *mockQuerier {
		return &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{
					ID: 7, Name: "Sycylia",
					GroupChatUrl: types.NullString{String: "https://discord.gg/sycylia", Valid: true},
					TravelNotes:  types.NullString{String: "Tanie loty do Katanii", Valid: true},
					MinAge:       types.NullInt64{Int64: 18, Valid: true},
				}, nil
			},
			countCruiseEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
				return sqlcdb.CountCruiseEnrollmentsRow{}, nil
			},
			listCruiseTripsFn: func(context.Context, types.NullInt64) ([]sqlcdb.Trip, error) {
				return nil, nil
			},
			countAcceptedByCruiseTripFn: func(context.Context, types.NullInt64) ([]sqlcdb.CountAcceptedByCruiseTripRow, error) {
				return nil, nil
			},
			listUserCruiseTripEnrollFn: func(context.Context, sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
				return nil, nil
			},
			getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
			},
		}
	}
	decode := func(t *testing.T, body []byte) dto.EnrollCruise {
		t.Helper()
		var info dto.EnrollInfo
		if err := json.Unmarshal(body, &info); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if info.Cruise == nil {
			t.Fatalf("no cruise in %s", body)
		}
		return *info.Cruise
	}

	t.Run("logged-in member sees all three", func(t *testing.T) {
		resp := enrollTestAPI(t, newMock()).GetCtx(userCtxRole(context.Background(), "member"), "/enroll/tok")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		c := decode(t, resp.Body.Bytes())
		if c.GroupChatUrl == nil || *c.GroupChatUrl != "https://discord.gg/sycylia" ||
			c.TravelNotes == nil || *c.TravelNotes != "Tanie loty do Katanii" ||
			c.MinAge == nil || *c.MinAge != 18 {
			t.Fatalf("cruise = %s", resp.Body)
		}
	})

	t.Run("anonymous preview hides the group chat link", func(t *testing.T) {
		resp := enrollTestAPI(t, newMock()).GetCtx(context.Background(), "/enroll/tok")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		c := decode(t, resp.Body.Bytes())
		if c.GroupChatUrl != nil {
			t.Fatalf("anonymous caller got group_chat_url %q", *c.GroupChatUrl)
		}
		if c.TravelNotes == nil || c.MinAge == nil {
			t.Fatalf("cruise = %s", resp.Body)
		}
	})
}
