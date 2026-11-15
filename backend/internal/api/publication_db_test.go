package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

// visibilityFixture is a series with a hidden edition and a published one,
// each with published and hidden legs. The member sailed both voyages of
// the published edition.
type visibilityFixture struct {
	series                                  int64
	hiddenCruise, hiddenCruiseTrip          int64
	hiddenCruiseVoyage                      int64
	cruise, trip, hiddenTrip                int64
	voyage, hiddenVoyage                    int64
	cruiseToken, hiddenCruiseToken          string
	tripToken, hiddenTripToken, hiddenInPub string
}

func insertID(t *testing.T, conn *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := conn.QueryRow(query, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return id
}

func seedVisibility(t *testing.T, conn *sql.DB) visibilityFixture {
	t.Helper()
	f := visibilityFixture{
		cruiseToken: "pub-cruise", hiddenCruiseToken: "hidden-cruise",
		tripToken: "pub-trip", hiddenTripToken: "hidden-trip", hiddenInPub: "pub-trip-in-hidden",
	}
	f.series = insertID(t, conn, `INSERT INTO cruise_series(name) VALUES('Series') RETURNING id`)
	f.hiddenCruise = insertID(t, conn, `INSERT INTO cruises(name,series_id,edition,enroll_token) VALUES('Draft edition',$1,'2027',$2) RETURNING id`, f.series, f.hiddenCruiseToken)
	f.hiddenCruiseTrip = insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,enroll_token,published_at) VALUES('Published leg of draft',$1,'PLN',$2,now()) RETURNING id`, f.hiddenCruise, f.hiddenInPub)
	f.hiddenCruiseVoyage = insertID(t, conn, `INSERT INTO voyages(name,cruise_id,miles,published_at) VALUES('Published voyage of draft',$1,500,now()) RETURNING id`, f.hiddenCruise)
	f.cruise = insertID(t, conn, `INSERT INTO cruises(name,series_id,edition,enroll_token,published_at) VALUES('Edition',$1,'2026',$2,now()) RETURNING id`, f.series, f.cruiseToken)
	f.trip = insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,enroll_token,published_at) VALUES('Leg',$1,'PLN',$2,now()) RETURNING id`, f.cruise, f.tripToken)
	f.hiddenTrip = insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,enroll_token) VALUES('Draft leg',$1,'PLN',$2) RETURNING id`, f.cruise, f.hiddenTripToken)
	f.voyage = insertID(t, conn, `INSERT INTO voyages(name,cruise_id,miles,days,published_at) VALUES('Voyage',$1,100,2,now()) RETURNING id`, f.cruise)
	f.hiddenVoyage = insertID(t, conn, `INSERT INTO voyages(name,cruise_id,miles,days) VALUES('Draft voyage',$1,40,1) RETURNING id`, f.cruise)
	for _, v := range []int64{f.voyage, f.hiddenVoyage} {
		if _, err := conn.Exec(`INSERT INTO crew_assignments(voyage_id,crew_member_id,role)
			SELECT $1, cm.id, 'zalogant' FROM crew_members cm JOIN users u ON u.id = cm.user_id WHERE u.firebase_uid = 'member'`, v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`UPDATE users SET is_captain = true WHERE firebase_uid = 'captain'`); err != nil {
		t.Fatal(err)
	}
	return f
}

func pageIDs[T any](t *testing.T, raw []byte, id func(T) int64) (ids []int64, total int64) {
	t.Helper()
	var page dto.Page[T]
	mustDecode(t, raw, &page)
	for _, item := range page.Items {
		ids = append(ids, id(item))
	}
	return ids, page.Total
}

func mustGet(t *testing.T, h http.Handler, token, path string) []byte {
	t.Helper()
	rr := securityRequest(h, token, http.MethodGet, path, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %s as %s: %d %s", path, token, rr.Code, rr.Body)
	}
	return rr.Body.Bytes()
}

func mustDecode(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestHiddenRecordVisibility runs the visibility rule through every persona:
// members never see a hidden cruise, trip or voyage, nor a published leg of a
// hidden cruise; admins and the captain group see everything. Share links
// and sign-ups refuse hidden records for everyone.
func TestHiddenRecordVisibility(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	f := seedVisibility(t, conn)

	t.Run("unverified account reads nothing", func(t *testing.T) {
		for _, path := range []string{"/api/cruises", fmt.Sprintf("/api/cruises/%d", f.cruise), fmt.Sprintf("/api/trips/%d", f.trip)} {
			if rr := securityRequest(h, "unverified", http.MethodGet, path, ""); rr.Code != http.StatusForbidden {
				t.Fatalf("%s: %d %s", path, rr.Code, rr.Body)
			}
		}
		for _, tok := range []string{f.hiddenCruiseToken, f.hiddenTripToken, f.hiddenInPub} {
			if rr := securityRequest(h, "unverified", http.MethodGet, "/api/enroll/"+tok, ""); rr.Code != http.StatusNotFound {
				t.Fatalf("hidden token %s: %d %s", tok, rr.Code, rr.Body)
			}
		}
	})

	for _, persona := range []string{"member", "captain", "admin"} {
		seesHidden := persona != "member"
		t.Run(persona, func(t *testing.T) {
			hiddenStatus := http.StatusNotFound
			if seesHidden {
				hiddenStatus = http.StatusOK
			}
			details := map[string]int{
				fmt.Sprintf("/api/cruises/%d", f.cruise):                    http.StatusOK,
				fmt.Sprintf("/api/trips/%d", f.trip):                        http.StatusOK,
				fmt.Sprintf("/api/voyages/%d", f.voyage):                    http.StatusOK,
				fmt.Sprintf("/api/cruises/%d", f.hiddenCruise):              hiddenStatus,
				fmt.Sprintf("/api/trips/%d", f.hiddenTrip):                  hiddenStatus,
				fmt.Sprintf("/api/trips/%d", f.hiddenCruiseTrip):            hiddenStatus,
				fmt.Sprintf("/api/voyages/%d", f.hiddenVoyage):              hiddenStatus,
				fmt.Sprintf("/api/voyages/%d", f.hiddenCruiseVoyage):        hiddenStatus,
				fmt.Sprintf("/api/cruises/%d/trips", f.hiddenCruise):        hiddenStatus,
				fmt.Sprintf("/api/cruises/%d/voyages", f.hiddenCruise):      hiddenStatus,
				fmt.Sprintf("/api/cruises/%d/leg-sign-ups", f.hiddenCruise): hiddenStatus,
				fmt.Sprintf("/api/trips/%d/crew", f.hiddenTrip):             hiddenStatus,
				fmt.Sprintf("/api/trips/%d/sign-up", f.hiddenTrip):          hiddenStatus,
				fmt.Sprintf("/api/voyages/%d/crew", f.hiddenVoyage):         hiddenStatus,
				fmt.Sprintf("/api/voyages/%d/ports", f.hiddenVoyage):        hiddenStatus,
				fmt.Sprintf("/api/voyages/%d/opinions", f.hiddenVoyage):     hiddenStatus,
			}
			for path, want := range details {
				if rr := securityRequest(h, persona, http.MethodGet, path, ""); rr.Code != want {
					t.Errorf("GET %s: %d, want %d; %s", path, rr.Code, want, rr.Body)
				}
			}

			cruises, cruiseTotal := pageIDs(t, mustGet(t, h, persona, "/api/cruises?limit=100"), func(c dto.Cruise) int64 { return c.ID })
			trips, tripTotal := pageIDs(t, mustGet(t, h, persona, "/api/trips?limit=100"), func(c dto.Trip) int64 { return c.ID })
			voyages, voyageTotal := pageIDs(t, mustGet(t, h, persona, "/api/voyages?limit=100"), func(c dto.Voyage) int64 { return c.ID })
			wantCruises := []int64{f.cruise}
			wantTrips := []int64{f.trip}
			wantVoyages := []int64{f.voyage}
			if seesHidden {
				wantCruises = append(wantCruises, f.hiddenCruise)
				wantTrips = append(wantTrips, f.hiddenTrip, f.hiddenCruiseTrip)
				wantVoyages = append(wantVoyages, f.hiddenVoyage, f.hiddenCruiseVoyage)
			}
			for _, c := range []struct {
				name  string
				got   []int64
				total int64
				want  []int64
			}{{"cruises", cruises, cruiseTotal, wantCruises}, {"trips", trips, tripTotal, wantTrips}, {"voyages", voyages, voyageTotal, wantVoyages}} {
				slices.Sort(c.got)
				slices.Sort(c.want)
				if !slices.Equal(c.got, c.want) || c.total != int64(len(c.want)) {
					t.Errorf("%s list = %v (total %d), want %v", c.name, c.got, c.total, c.want)
				}
			}

			var legs []dto.Trip
			mustDecode(t, mustGet(t, h, persona, fmt.Sprintf("/api/cruises/%d/trips", f.cruise)), &legs)
			if len(legs) != 1+boolInt(seesHidden) {
				t.Errorf("cruise legs = %d, want %d", len(legs), 1+boolInt(seesHidden))
			}
			var cruiseVoyages []dto.Voyage
			mustDecode(t, mustGet(t, h, persona, fmt.Sprintf("/api/cruises/%d/voyages", f.cruise)), &cruiseVoyages)
			if len(cruiseVoyages) != 1+boolInt(seesHidden) {
				t.Errorf("cruise voyages = %d, want %d", len(cruiseVoyages), 1+boolInt(seesHidden))
			}
			var signUps []dto.LegSignUp
			mustDecode(t, mustGet(t, h, persona, fmt.Sprintf("/api/cruises/%d/leg-sign-ups", f.cruise)), &signUps)
			for _, s := range signUps {
				if s.TripID == f.hiddenTrip && (!seesHidden || s.Open) {
					t.Errorf("hidden leg in sign-ups: %+v", s)
				}
			}

			var editions []dto.CruiseSeriesEdition
			mustDecode(t, mustGet(t, h, persona, fmt.Sprintf("/api/series/%d/editions", f.series)), &editions)
			wantEditions := map[int64]bool{f.cruise: true}
			if seesHidden {
				wantEditions = map[int64]bool{f.cruise: true, f.hiddenCruise: true}
			}
			if len(editions) != len(wantEditions) {
				t.Errorf("editions = %+v, want %v", editions, wantEditions)
			}
			for _, e := range editions {
				if !wantEditions[e.CruiseID] || (e.PublishedAt == nil) != (e.CruiseID == f.hiddenCruise) {
					t.Errorf("unexpected edition %+v", e)
				}
			}

			for _, tok := range []string{f.hiddenCruiseToken, f.hiddenTripToken, f.hiddenInPub} {
				if rr := securityRequest(h, persona, http.MethodGet, "/api/enroll/"+tok, ""); rr.Code != http.StatusNotFound {
					t.Errorf("token preview %s: %d %s", tok, rr.Code, rr.Body)
				}
				if rr := securityRequest(h, persona, http.MethodPost, "/api/enroll/"+tok, "{}"); rr.Code != http.StatusNotFound {
					t.Errorf("token enroll %s: %d %s", tok, rr.Code, rr.Body)
				}
			}
			var info dto.EnrollInfo
			mustDecode(t, mustGet(t, h, persona, "/api/enroll/"+f.cruiseToken), &info)
			for _, leg := range info.Trips {
				if leg.ID == f.hiddenTrip {
					t.Errorf("cruise link offers the hidden leg: %+v", info.Trips)
				}
			}
			for _, c := range info.TripAccepted {
				if c.TripID == f.hiddenTrip {
					t.Errorf("cruise link counts the hidden leg: %+v", info.TripAccepted)
				}
			}
			rr := securityRequest(h, persona, http.MethodPost, "/api/enroll/"+f.cruiseToken+"/legs", fmt.Sprintf(`{"trip_ids":[%d]}`, f.hiddenTrip))
			if rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("cruise link sign-up to hidden leg: %d %s", rr.Code, rr.Body)
			}
			signUp := http.StatusNotFound
			if seesHidden {
				signUp = http.StatusUnprocessableEntity
			}
			if rr := securityRequest(h, persona, http.MethodPost, fmt.Sprintf("/api/trips/%d/sign-up", f.hiddenTrip), ""); rr.Code != signUp {
				t.Errorf("sign-up to hidden leg: %d, want %d; %s", rr.Code, signUp, rr.Body)
			}
		})
	}

	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM trip_enrollments`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("sign-ups to hidden legs = %d, err %v", count, err)
	}

	t.Run("member sea time counts visible voyages only", func(t *testing.T) {
		var dash dto.Dashboard
		mustDecode(t, mustGet(t, h, "member", "/api/dashboard"), &dash)
		if dash.Personal.Total.VoyageCount != 1 || dash.Personal.Total.TotalMiles != 100 {
			t.Fatalf("personal = %+v, want the published voyage only", dash.Personal.Total)
		}
		mustDecode(t, mustGet(t, h, "admin", "/api/dashboard"), &dash)
		if dash.Club == nil || dash.Club.Total.VoyageCount != 3 {
			t.Fatalf("admin club totals = %+v, want every voyage", dash.Club)
		}
	})
}

// TestPublication covers the admin-only publish and unpublish operations.
func TestPublication(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	f := seedVisibility(t, conn)
	post := func(token, path, body string) (int, string) {
		t.Helper()
		rr := securityRequest(h, token, http.MethodPost, path, body)
		return rr.Code, rr.Body.String()
	}
	memberSees := func(path string) bool {
		t.Helper()
		rr := securityRequest(h, "member", http.MethodGet, path, "")
		if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
			t.Fatalf("GET %s: %d %s", path, rr.Code, rr.Body)
		}
		return rr.Code == http.StatusOK
	}

	t.Run("non-admins are refused", func(t *testing.T) {
		paths := []string{
			fmt.Sprintf("/api/cruises/%d/publish", f.hiddenCruise), fmt.Sprintf("/api/cruises/%d/unpublish", f.cruise),
			fmt.Sprintf("/api/trips/%d/publish", f.hiddenTrip), fmt.Sprintf("/api/trips/%d/unpublish", f.trip),
			fmt.Sprintf("/api/voyages/%d/publish", f.hiddenVoyage), fmt.Sprintf("/api/voyages/%d/unpublish", f.voyage),
		}
		for _, token := range []string{"member", "captain"} {
			for _, path := range paths {
				if code, body := post(token, path, ""); code != http.StatusForbidden {
					t.Errorf("%s POST %s: %d %s", token, path, code, body)
				}
			}
		}
		if memberSees(fmt.Sprintf("/api/cruises/%d", f.hiddenCruise)) || !memberSees(fmt.Sprintf("/api/cruises/%d", f.cruise)) {
			t.Fatal("a refused call changed visibility")
		}
	})

	t.Run("unknown records are 404", func(t *testing.T) {
		for _, path := range []string{"/api/cruises/999999/publish", "/api/trips/999999/publish", "/api/voyages/999999/unpublish"} {
			if code, body := post("admin", path, ""); code != http.StatusNotFound {
				t.Errorf("POST %s: %d %s", path, code, body)
			}
		}
	})

	t.Run("a leg of a hidden series edition stays hidden", func(t *testing.T) {
		if code, body := post("admin", fmt.Sprintf("/api/trips/%d/publish", f.hiddenCruiseTrip), ""); code != http.StatusOK {
			t.Fatalf("publish: %d %s", code, body)
		}
		if memberSees(fmt.Sprintf("/api/cruises/%d", f.hiddenCruise)) || memberSees(fmt.Sprintf("/api/trips/%d", f.hiddenCruiseTrip)) {
			t.Fatal("publishing a leg published its series edition")
		}
	})

	t.Run("publishing keeps an earlier publication time", func(t *testing.T) {
		var before, after string
		if err := conn.QueryRow(`SELECT published_at::text FROM cruises WHERE id=$1`, f.cruise).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if code, body := post("admin", fmt.Sprintf("/api/cruises/%d/publish", f.cruise), ""); code != http.StatusOK {
			t.Fatalf("publish: %d %s", code, body)
		}
		if err := conn.QueryRow(`SELECT published_at::text FROM cruises WHERE id=$1`, f.cruise).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatalf("published_at %s -> %s", before, after)
		}
		if memberSees(fmt.Sprintf("/api/trips/%d", f.hiddenTrip)) {
			t.Fatal("publishing without include_legs published a leg")
		}
	})

	t.Run("include_legs publishes every hidden leg", func(t *testing.T) {
		if code, body := post("admin", fmt.Sprintf("/api/cruises/%d/publish", f.cruise), `{"include_legs":true}`); code != http.StatusOK {
			t.Fatalf("publish: %d %s", code, body)
		}
		if !memberSees(fmt.Sprintf("/api/trips/%d", f.hiddenTrip)) || !memberSees(fmt.Sprintf("/api/voyages/%d", f.hiddenVoyage)) {
			t.Fatal("hidden legs still hidden")
		}
	})

	t.Run("a leg of a hidden standalone cruise publishes the cruise", func(t *testing.T) {
		for _, kind := range []string{"trips", "voyages"} {
			cruise := insertID(t, conn, `INSERT INTO cruises(name) VALUES('Standalone') RETURNING id`)
			var leg int64
			if kind == "trips" {
				leg = insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency) VALUES('Solo',$1,'PLN') RETURNING id`, cruise)
			} else {
				leg = insertID(t, conn, `INSERT INTO voyages(name,cruise_id) VALUES('Solo',$1) RETURNING id`, cruise)
			}
			if code, body := post("admin", fmt.Sprintf("/api/%s/%d/publish", kind, leg), ""); code != http.StatusOK {
				t.Fatalf("publish %s: %d %s", kind, code, body)
			}
			if !memberSees(fmt.Sprintf("/api/cruises/%d", cruise)) || !memberSees(fmt.Sprintf("/api/%s/%d", kind, leg)) {
				t.Fatalf("standalone %s or its cruise still hidden", kind)
			}
		}
	})

	t.Run("unpublish is allowed with enrollments", func(t *testing.T) {
		if rr := securityRequest(h, "member", http.MethodPost, fmt.Sprintf("/api/trips/%d/sign-up", f.trip), ""); rr.Code != http.StatusCreated {
			t.Fatalf("sign-up: %d %s", rr.Code, rr.Body)
		}
		if code, body := post("admin", fmt.Sprintf("/api/trips/%d/unpublish", f.trip), ""); code != http.StatusOK {
			t.Fatalf("unpublish: %d %s", code, body)
		}
		if memberSees(fmt.Sprintf("/api/trips/%d", f.trip)) {
			t.Fatal("unpublished trip still visible")
		}
		if rr := securityRequest(h, "member", http.MethodDelete, fmt.Sprintf("/api/trips/%d/sign-up", f.trip), ""); rr.Code != http.StatusNotFound {
			t.Fatalf("withdraw from hidden leg: %d %s", rr.Code, rr.Body)
		}
		if code, body := post("admin", fmt.Sprintf("/api/cruises/%d/unpublish", f.cruise), ""); code != http.StatusOK {
			t.Fatalf("unpublish cruise: %d %s", code, body)
		}
		if memberSees(fmt.Sprintf("/api/cruises/%d", f.cruise)) || memberSees(fmt.Sprintf("/api/voyages/%d", f.voyage)) {
			t.Fatal("hidden cruise or its published voyage still visible")
		}
	})
}

// TestTripCompletionKeepsPublication completes a published and a hidden
// trip: each voyage starts as visible as its trip was.
func TestTripCompletionKeepsPublication(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	cruise := insertID(t, conn, `INSERT INTO cruises(name,published_at) VALUES('Event',now()) RETURNING id`)
	for _, published := range []bool{true, false} {
		trip := insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,published_at)
			VALUES('Leg',$1,'PLN',CASE WHEN $2 THEN now() END) RETURNING id`, cruise, published)
		rr := securityRequest(h, "admin", http.MethodPost, fmt.Sprintf("/api/trips/%d/complete", trip), "{}")
		if rr.Code != http.StatusCreated {
			t.Fatalf("complete: %d %s", rr.Code, rr.Body)
		}
		var voyage dto.Voyage
		mustDecode(t, rr.Body.Bytes(), &voyage)
		if (voyage.PublishedAt != nil) != published {
			t.Fatalf("trip published=%v gave voyage published_at %v", published, voyage.PublishedAt)
		}
		member := securityRequest(h, "member", http.MethodGet, fmt.Sprintf("/api/voyages/%d", voyage.ID), "")
		if (member.Code == http.StatusOK) != published {
			t.Fatalf("member sees voyage of published=%v trip: %d", published, member.Code)
		}
	}
}

// TestPublicationMigrationKeepsExistingRowsPublic reapplies the publication
// migration over existing rows, including one without created_at: every one
// of them reads as published, and a row added afterwards starts hidden.
func TestPublicationMigrationKeepsExistingRowsPublic(t *testing.T) {
	conn := securityDatabase(t)
	if _, err := conn.Exec(`ALTER TABLE cruises DROP COLUMN published_at;
 ALTER TABLE trips DROP COLUMN published_at;
 ALTER TABLE voyages DROP COLUMN published_at;
 DELETE FROM schema_migrations WHERE version='032_publication.sql';
 INSERT INTO cruises(name) VALUES ('Old');
 INSERT INTO cruises(name, created_at) VALUES ('No timestamp', NULL);
 INSERT INTO trips(name,cruise_id,currency) SELECT 'Old leg', id, 'PLN' FROM cruises;
 INSERT INTO voyages(name,cruise_id) SELECT 'Old voyage', id FROM cruises;`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	var hidden int
	if err := conn.QueryRow(`SELECT
		(SELECT count(*) FROM cruises WHERE published_at IS NULL) +
		(SELECT count(*) FROM trips WHERE published_at IS NULL) +
		(SELECT count(*) FROM voyages WHERE published_at IS NULL)`).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if hidden != 0 {
		t.Fatalf("hidden rows after migration = %d", hidden)
	}
	if _, err := conn.Exec(`INSERT INTO cruises(name) VALUES ('New')`); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`SELECT count(*) FROM cruises WHERE published_at IS NULL`).Scan(&hidden); err != nil || hidden != 1 {
		t.Fatalf("new cruise hidden rows = %d err=%v, want 1", hidden, err)
	}
}

// TestHiddenVoyageStaysWritableByItsCaptain: hiding a voyage does not change
// who may edit it. Its assigned captain, outside the captain group, still
// writes the log; another member gets 404 on the hidden voyage and 403 on a
// published one.
func TestHiddenVoyageStaysWritableByItsCaptain(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	f := seedVisibility(t, conn)
	if _, err := conn.Exec(`UPDATE voyages SET captain_person_id =
		(SELECT cm.id FROM crew_members cm JOIN users u ON u.id = cm.user_id WHERE u.firebase_uid = 'subject')
		WHERE id = ANY($1)`, []int64{f.voyage, f.hiddenVoyage}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		token  string
		voyage int64
		want   int
	}{
		{"subject", f.hiddenVoyage, http.StatusNoContent},
		{"subject", f.voyage, http.StatusNoContent},
		{"member", f.hiddenVoyage, http.StatusNotFound},
		{"member", f.voyage, http.StatusForbidden},
		{"captain", f.hiddenVoyage, http.StatusForbidden},
		{"admin", f.hiddenVoyage, http.StatusNoContent},
	}
	for _, c := range cases {
		for _, req := range []struct{ method, path, body string }{
			{http.MethodPut, fmt.Sprintf("/api/voyages/%d/log", c.voyage), `{"miles":12}`},
			{http.MethodPost, fmt.Sprintf("/api/voyages/%d/ports", c.voyage), `{"name":"Hel"}`},
		} {
			want := c.want
			if req.method == http.MethodPost && want == http.StatusNoContent {
				want = http.StatusCreated
			}
			rr := securityRequest(h, c.token, req.method, req.path, req.body)
			if rr.Code != want {
				t.Errorf("%s %s %s: %d %s, want %d", c.token, req.method, req.path, rr.Code, rr.Body, want)
			}
		}
	}
}

// TestHiddenVoyageHidesYachtPickerFromItsCaptain: the voyage's captain
// outside the captain group reads the yacht picker list only while they can
// see the voyage; a hidden voyage or one under a hidden cruise answers 404.
func TestHiddenVoyageHidesYachtPickerFromItsCaptain(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	f := seedVisibility(t, conn)
	if _, err := conn.Exec(`UPDATE voyages SET captain_person_id =
		(SELECT cm.id FROM crew_members cm JOIN users u ON u.id = cm.user_id WHERE u.firebase_uid = 'subject')
		WHERE id = ANY($1)`, []int64{f.voyage, f.hiddenVoyage, f.hiddenCruiseVoyage}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		token  string
		voyage int64
		want   int
	}{
		{"subject", f.voyage, http.StatusOK},
		{"subject", f.hiddenVoyage, http.StatusNotFound},
		{"subject", f.hiddenCruiseVoyage, http.StatusNotFound},
		{"member", f.hiddenVoyage, http.StatusNotFound},
		{"captain", f.hiddenVoyage, http.StatusForbidden},
		{"admin", f.hiddenVoyage, http.StatusOK},
	}
	for _, c := range cases {
		path := fmt.Sprintf("/api/voyages/%d/yachts", c.voyage)
		if rr := securityRequest(h, c.token, http.MethodGet, path, ""); rr.Code != c.want {
			t.Errorf("%s GET %s: %d %s, want %d", c.token, path, rr.Code, rr.Body, c.want)
		}
	}
}

// TestCruiseShareLinkHidesHiddenLegEnrollments: a published cruise's share
// link lists the caller's leg enrollments only for the legs it offers.
func TestCruiseShareLinkHidesHiddenLegEnrollments(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	f := seedVisibility(t, conn)
	if _, err := conn.Exec(`INSERT INTO crew_members(user_id, full_name)
		SELECT id, email FROM users WHERE firebase_uid = 'member' ON CONFLICT (user_id) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO trip_enrollments(trip_id, crew_member_id)
		SELECT t, cm.id FROM unnest($1::BIGINT[]) t, users u JOIN crew_members cm ON cm.user_id = u.id
		WHERE u.firebase_uid = 'member'`, []int64{f.trip, f.hiddenTrip}); err != nil {
		t.Fatal(err)
	}
	var info dto.EnrollInfo
	mustDecode(t, mustGet(t, h, "member", "/api/enroll/"+f.cruiseToken), &info)
	var legs []int64
	for _, e := range info.LegEnrollments {
		if e.TripID != nil {
			legs = append(legs, *e.TripID)
		}
	}
	if !slices.Equal(legs, []int64{f.trip}) {
		t.Fatalf("leg enrollments = %v, want only the published leg %d", legs, f.trip)
	}
}

// TestPublishLegLocksCruiseFirst: publishing a leg takes its cruise lock
// before the leg's, the order cruise publication with include_legs uses, so
// the two cannot deadlock. While another transaction holds the cruise, the
// leg publish waits without holding the leg.
func TestPublishLegLocksCruiseFirst(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	cruise := insertID(t, conn, `INSERT INTO cruises(name) VALUES('Standalone') RETURNING id`)
	trip := insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency) VALUES('Leg',$1,'PLN') RETURNING id`, cruise)
	voyage := insertID(t, conn, `INSERT INTO voyages(name,cruise_id) VALUES('Voyage',$1) RETURNING id`, cruise)

	for _, leg := range []struct {
		table string
		id    int64
	}{{"trips", trip}, {"voyages", voyage}} {
		t.Run(leg.table, func(t *testing.T) {
			holder, err := conn.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback() }()
			if _, err := holder.Exec(`SELECT id FROM cruises WHERE id = $1 FOR NO KEY UPDATE`, cruise); err != nil {
				t.Fatal(err)
			}
			done := make(chan int, 1)
			go func() {
				rr := securityRequest(h, "admin", http.MethodPost, fmt.Sprintf("/api/%s/%d/publish", leg.table, leg.id), "")
				done <- rr.Code
			}()
			waitForLockWait(t, conn)
			probe, err := conn.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			_, lockErr := probe.Exec(fmt.Sprintf(`SELECT id FROM %s WHERE id = $1 FOR UPDATE NOWAIT`, leg.table), leg.id)
			_ = probe.Rollback()
			if lockErr != nil {
				t.Errorf("leg locked while its publish waits for the cruise: %v", lockErr)
			}
			if err := holder.Commit(); err != nil {
				t.Fatal(err)
			}
			if code := <-done; code != http.StatusOK {
				t.Fatalf("publish %s: %d", leg.table, code)
			}
		})
	}
	var hidden int
	if err := conn.QueryRow(`SELECT
		(SELECT count(*) FROM cruises WHERE id = $1 AND published_at IS NULL) +
		(SELECT count(*) FROM trips WHERE id = $2 AND published_at IS NULL) +
		(SELECT count(*) FROM voyages WHERE id = $3 AND published_at IS NULL)`, cruise, trip, voyage).Scan(&hidden); err != nil || hidden != 0 {
		t.Fatalf("hidden after publish = %d err=%v", hidden, err)
	}
}

// waitForLockWait blocks until a session of this database waits on a lock.
func waitForLockWait(t *testing.T, conn *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := conn.QueryRow(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no session waited on the held cruise lock")
}

// TestStandaloneLegsHidesHiddenRecords: /standalone-legs lists the legs of
// cruises without a series and such cruises without a leg, leaving out a
// hidden leg, a leg of a hidden cruise and a hidden cruise, except for the
// captain group and admins. Legs of a series never appear.
func TestStandaloneLegsHidesHiddenRecords(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	seedVisibility(t, conn)
	cruise := insertID(t, conn, `INSERT INTO cruises(name,published_at) VALUES('Standalone',now()) RETURNING id`)
	trip := insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,embark_date,published_at) VALUES('Leg',$1,'PLN','2026-07-01',now()) RETURNING id`, cruise)
	hiddenTrip := insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,embark_date) VALUES('Draft leg',$1,'PLN','') RETURNING id`, cruise)
	voyage := insertID(t, conn, `INSERT INTO voyages(name,cruise_id,embark_date,published_at) VALUES('Voyage',$1,'2025-06-01',now()) RETURNING id`, cruise)
	hiddenWrapper := insertID(t, conn, `INSERT INTO cruises(name) VALUES('Draft wrapper') RETURNING id`)
	wrapperTrip := insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency,embark_date,published_at) VALUES('Leg of draft',$1,'PLN','2026-08-01',now()) RETURNING id`, hiddenWrapper)
	empty := insertID(t, conn, `INSERT INTO cruises(name,published_at) VALUES('No legs',now()) RETURNING id`)
	hiddenEmpty := insertID(t, conn, `INSERT INTO cruises(name) VALUES('Draft without legs') RETURNING id`)

	member := []string{
		fmt.Sprint("cruise ", empty), fmt.Sprint("trip ", trip), fmt.Sprint("voyage ", voyage),
	}
	all := []string{
		fmt.Sprint("cruise ", hiddenEmpty), fmt.Sprint("cruise ", empty), fmt.Sprint("trip ", hiddenTrip),
		fmt.Sprint("trip ", wrapperTrip), fmt.Sprint("trip ", trip), fmt.Sprint("voyage ", voyage),
	}
	want := map[string][]string{"member": member, "captain": all, "admin": all}
	for persona, entries := range want {
		var got []dto.StandaloneLeg
		mustDecode(t, mustGet(t, h, persona, "/api/standalone-legs"), &got)
		gotEntries := make([]string, len(got))
		for i, leg := range got {
			gotEntries[i] = fmt.Sprint(leg.Kind, " ", leg.ID)
			if leg.Kind == "trip" && leg.ID == wrapperTrip && (leg.CruiseID != hiddenWrapper || leg.CruisePublishedAt != nil) {
				t.Errorf("%s wrapper leg = %+v, want its hidden cruise %d", persona, leg, hiddenWrapper)
			}
		}
		if !slices.Equal(gotEntries, entries) {
			t.Errorf("%s entries = %v, want %v", persona, gotEntries, entries)
		}
	}
}

// TestMyLegsHidesHiddenRecords: /auth/me/legs leaves out a hidden leg and a
// leg of a hidden cruise, except for the captain group and admins.
func TestMyLegsHidesHiddenRecords(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	f := seedVisibility(t, conn)
	for _, persona := range []string{"member", "captain"} {
		for _, trip := range []int64{f.trip, f.hiddenTrip, f.hiddenCruiseTrip} {
			if _, err := conn.Exec(`INSERT INTO crew_assignments(trip_id,crew_member_id,role)
				SELECT $1, cm.id, 'zalogant' FROM crew_members cm JOIN users u ON u.id = cm.user_id
				WHERE u.firebase_uid = $2`, trip, persona); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := map[string][]int64{
		"member":  {f.trip},
		"captain": {f.hiddenCruiseTrip, f.trip, f.hiddenTrip},
	}
	for persona, ids := range want {
		var got dto.MyLegs
		mustDecode(t, mustGet(t, h, persona, "/api/auth/me/legs"), &got)
		gotIDs := make([]int64, len(got.Legs))
		for i, leg := range got.Legs {
			gotIDs[i] = leg.ID
			if leg.Relation != "crew" {
				t.Errorf("%s leg %d relation = %q, want crew", persona, leg.ID, leg.Relation)
			}
		}
		if !slices.Equal(gotIDs, ids) {
			t.Errorf("%s leg ids = %v, want %v", persona, gotIDs, ids)
		}
	}
}

// TestPublishLegMovedAfterCruiseLock moves the leg to a hidden standalone
// cruise after its publish locked the old cruise, while a cruise publish of
// the new cruise with include_legs holds that cruise. The leg publish waits
// for the move with the old cruise locked; once the move commits it must
// start over on the new cruise instead of waiting for it while holding the
// leg, so neither side aborts as a deadlock victim (40P01).
func TestPublishLegMovedAfterCruiseLock(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)

	for _, leg := range []struct {
		table, insert string
	}{
		{"trips", `INSERT INTO trips(name,cruise_id,currency) VALUES('Leg',$1,'PLN') RETURNING id`},
		{"voyages", `INSERT INTO voyages(name,cruise_id) VALUES('Voyage',$1) RETURNING id`},
	} {
		t.Run(leg.table, func(t *testing.T) {
			from := insertID(t, conn, `INSERT INTO cruises(name) VALUES('From') RETURNING id`)
			to := insertID(t, conn, `INSERT INTO cruises(name) VALUES('To') RETURNING id`)
			id := insertID(t, conn, leg.insert, from)

			move, movePID := beginWithPID(t, conn)
			if _, err := move.Exec(fmt.Sprintf(`UPDATE %s SET cruise_id = $1 WHERE id = $2`, leg.table), to, id); err != nil {
				t.Fatal(err)
			}
			done := make(chan int, 1)
			go func() {
				rr := securityRequest(h, "admin", http.MethodPost, fmt.Sprintf("/api/%s/%d/publish", leg.table, id), "")
				done <- rr.Code
			}()
			waitForBlockedBy(t, conn, movePID)

			cruisePublish, cruisePID := beginWithPID(t, conn)
			if _, err := cruisePublish.Exec(`UPDATE cruises SET published_at = now() WHERE id = $1`, to); err != nil {
				t.Fatal(err)
			}
			if err := move.Commit(); err != nil {
				t.Fatal(err)
			}
			waitForBlockedBy(t, conn, cruisePID)

			if _, err := cruisePublish.Exec(fmt.Sprintf(
				`UPDATE %s SET published_at = now() WHERE cruise_id = $1 AND published_at IS NULL`, leg.table), to); err != nil {
				t.Fatalf("publish cruise legs: %v", err)
			}
			if err := cruisePublish.Commit(); err != nil {
				t.Fatalf("commit cruise publish: %v", err)
			}
			if code := <-done; code != http.StatusOK {
				t.Fatalf("publish %s: %d", leg.table, code)
			}

			var cruiseID int64
			var published bool
			if err := conn.QueryRow(fmt.Sprintf(`SELECT cruise_id, published_at IS NOT NULL FROM %s WHERE id = $1`, leg.table), id).
				Scan(&cruiseID, &published); err != nil {
				t.Fatal(err)
			}
			if cruiseID != to || !published {
				t.Fatalf("leg cruise=%d published=%v, want %d true", cruiseID, published, to)
			}
		})
	}
}

// TestPublishCruiseRetriesDeadlock holds a hidden voyage, lets a cruise
// publish with include_legs lock the cruise's hidden leg and wait for that
// voyage, then makes the holder wait for the leg. The publish waits first and
// the holder's deadlock_timeout is far longer, so the publish is the victim:
// it must run again after the holder commits instead of answering 500.
func TestPublishCruiseRetriesDeadlock(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	cruise := insertID(t, conn, `INSERT INTO cruises(name) VALUES('Hidden') RETURNING id`)
	trip := insertID(t, conn, `INSERT INTO trips(name,cruise_id,currency) VALUES('Leg',$1,'PLN') RETURNING id`, cruise)
	voyage := insertID(t, conn, `INSERT INTO voyages(name,cruise_id) VALUES('Voyage',$1) RETURNING id`, cruise)

	holder, holderPID := beginWithPID(t, conn)
	for _, step := range []string{
		"SET LOCAL deadlock_timeout = '60s'",
		fmt.Sprintf("SELECT id FROM voyages WHERE id = %d FOR NO KEY UPDATE", voyage),
	} {
		if _, err := holder.Exec(step); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	done := make(chan int, 1)
	go func() {
		rr := securityRequest(h, "admin", http.MethodPost, fmt.Sprintf("/api/cruises/%d/publish", cruise), `{"include_legs":true}`)
		done <- rr.Code
	}()
	waitForBlockedBy(t, conn, holderPID)

	if _, err := holder.Exec(`SELECT id FROM trips WHERE id = $1 FOR UPDATE`, trip); err != nil {
		t.Fatalf("holder locks the leg: %v", err)
	}
	if err := holder.Commit(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != http.StatusOK {
		t.Fatalf("publish cruise: got %d, want 200", code)
	}
	var hidden int
	if err := conn.QueryRow(`SELECT (SELECT count(*) FROM trips WHERE id = $1 AND published_at IS NULL)
		+ (SELECT count(*) FROM voyages WHERE id = $2 AND published_at IS NULL)`, trip, voyage).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if hidden != 0 {
		t.Fatalf("%d legs still hidden after the publish", hidden)
	}
}

// beginWithPID starts a transaction and returns it with its backend PID.
func beginWithPID(t *testing.T, conn *sql.DB) (tx *sql.Tx, pid int) {
	t.Helper()
	tx, err := conn.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if err := tx.QueryRow(`SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	return tx, pid
}

// waitForBlockedBy blocks until a session of this database waits on a lock
// the backend pid holds.
func waitForBlockedBy(t *testing.T, conn *sql.DB, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := conn.QueryRow(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND $1 = ANY(pg_blocking_pids(pid))`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no session waited on a lock of backend %d", pid)
}
