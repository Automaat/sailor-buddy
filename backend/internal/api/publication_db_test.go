package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

// visibilityFixture is a series with a hidden edition and a published one,
// each with published and hidden legs. The approved member sailed both
// voyages of the published edition.
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
			SELECT $1, cm.id, 'zalogant' FROM crew_members cm JOIN users u ON u.id = cm.user_id WHERE u.firebase_uid = 'approved'`, v); err != nil {
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

	t.Run("pending account reads nothing", func(t *testing.T) {
		for _, path := range []string{"/api/cruises", fmt.Sprintf("/api/cruises/%d", f.cruise), fmt.Sprintf("/api/trips/%d", f.trip)} {
			if rr := securityRequest(h, "pending", http.MethodGet, path, ""); rr.Code != http.StatusForbidden {
				t.Fatalf("%s: %d %s", path, rr.Code, rr.Body)
			}
		}
		for _, tok := range []string{f.hiddenCruiseToken, f.hiddenTripToken, f.hiddenInPub} {
			if rr := securityRequest(h, "pending", http.MethodGet, "/api/enroll/"+tok, ""); rr.Code != http.StatusNotFound {
				t.Fatalf("hidden token %s: %d %s", tok, rr.Code, rr.Body)
			}
		}
	})

	for _, persona := range []string{"approved", "captain", "admin"} {
		seesHidden := persona != "approved"
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
			wantEditions := map[int64]float64{f.cruise: 100}
			if seesHidden {
				wantEditions = map[int64]float64{f.cruise: 140, f.hiddenCruise: 500}
			}
			if len(editions) != len(wantEditions) {
				t.Errorf("editions = %+v, want %v", editions, wantEditions)
			}
			for _, e := range editions {
				if miles, ok := wantEditions[e.CruiseID]; !ok || e.TotalMiles != miles || (e.PublishedAt == nil) != (e.CruiseID == f.hiddenCruise) {
					t.Errorf("edition %+v, want miles %v", e, wantEditions[e.CruiseID])
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
		mustDecode(t, mustGet(t, h, "approved", "/api/dashboard"), &dash)
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
		rr := securityRequest(h, "approved", http.MethodGet, path, "")
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
		for _, token := range []string{"approved", "captain"} {
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
		if rr := securityRequest(h, "approved", http.MethodPost, fmt.Sprintf("/api/trips/%d/sign-up", f.trip), ""); rr.Code != http.StatusCreated {
			t.Fatalf("sign-up: %d %s", rr.Code, rr.Body)
		}
		if code, body := post("admin", fmt.Sprintf("/api/trips/%d/unpublish", f.trip), ""); code != http.StatusOK {
			t.Fatalf("unpublish: %d %s", code, body)
		}
		if memberSees(fmt.Sprintf("/api/trips/%d", f.trip)) {
			t.Fatal("unpublished trip still visible")
		}
		if rr := securityRequest(h, "approved", http.MethodDelete, fmt.Sprintf("/api/trips/%d/sign-up", f.trip), ""); rr.Code != http.StatusNotFound {
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
		member := securityRequest(h, "approved", http.MethodGet, fmt.Sprintf("/api/voyages/%d", voyage.ID), "")
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
