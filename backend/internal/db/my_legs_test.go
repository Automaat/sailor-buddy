package db

import (
	"slices"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestListUserUpcomingLegs(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	insert := func(query string, args ...any) int64 {
		t.Helper()
		var id int64
		mustScan(t, conn.QueryRow(query, args...), &id)
		return id
	}

	const today = "2026-11-15"
	me := insert(`INSERT INTO users (email, name) VALUES ('jan@example.dev', 'Jan') RETURNING id`)
	other := insert(`INSERT INTO users (email, name) VALUES ('ola@example.dev', 'Ola') RETURNING id`)
	mePerson := insert(`INSERT INTO crew_members (full_name, user_id) VALUES ('Jan', $1) RETURNING id`, me)
	otherPerson := insert(`INSERT INTO crew_members (full_name, user_id) VALUES ('Ola', $1) RETURNING id`, other)
	sameEmailPerson := insert(`INSERT INTO crew_members (full_name, email) VALUES ('Jan bez konta', 'jan@example.dev') RETURNING id`)

	series := insert(`INSERT INTO cruise_series (name) VALUES ('Sycylia') RETURNING id`)
	edition := insert(`INSERT INTO cruises (name, series_id, edition, published_at) VALUES ('Sycylia 2027', $1, '2027', now()) RETURNING id`, series)
	standalone := insert(`INSERT INTO cruises (name, published_at) VALUES ('Bałtyk', now()) RETURNING id`)
	hiddenCruise := insert(`INSERT INTO cruises (name) VALUES ('Szkic') RETURNING id`)

	type legSpec struct {
		cruise          int64
		embark, disemb  any
		status          string
		published       bool
		captainPersonID any
	}
	leg := func(name string, s legSpec) int64 {
		t.Helper()
		if s.status == "" {
			s.status = "planned"
		}
		return insert(`INSERT INTO trips (name, cruise_id, embark_date, disembark_date, status, captain_person_id, published_at)
			VALUES ($1, $2, $3, $4, $5::trip_status, $6, CASE WHEN $7::BOOLEAN THEN now() END) RETURNING id`,
			name, s.cruise, s.embark, s.disemb, s.status, s.captainPersonID, s.published)
	}
	upcoming := func(cruise int64, embark string) legSpec {
		return legSpec{cruise: cruise, embark: embark, disemb: embark[:8] + "28", published: true}
	}
	crew := func(trip, person int64) {
		exec(`INSERT INTO crew_assignments (trip_id, crew_member_id) VALUES ($1, $2)`, trip, person)
	}
	signUp := func(trip, person int64, status string) {
		exec(`INSERT INTO trip_enrollments (trip_id, crew_member_id, status) VALUES ($1, $2, $3)`, trip, person, status)
	}
	cruiseEnroll := func(cruise int64, trip, user any, status string) {
		exec(`INSERT INTO cruise_enrollments (cruise_id, trip_id, user_id, status) VALUES ($1, $2, $3, $4)`, cruise, trip, user, status)
	}

	captained := leg("captain", legSpec{cruise: edition, embark: "2027-01-10", disemb: "2027-01-17", published: true, captainPersonID: mePerson})
	crewed := leg("crew", upcoming(edition, "2027-02-10"))
	crew(crewed, mePerson)
	accepted := leg("accepted", upcoming(standalone, "2027-03-10"))
	signUp(accepted, mePerson, "accepted")
	pending := leg("pending", upcoming(standalone, "2027-04-10"))
	signUp(pending, mePerson, "pending")
	waitlisted := leg("waitlisted", upcoming(standalone, "2027-05-10"))
	signUp(waitlisted, mePerson, "waitlisted")

	ownCruise := func(name string) int64 {
		return insert(`INSERT INTO cruises (name, published_at) VALUES ($1, now()) RETURNING id`, name)
	}
	legacyCruise := ownCruise("legacy")
	legacy := leg("legacy cruise enrollment", upcoming(legacyCruise, "2027-06-10"))
	cruiseEnroll(legacyCruise, legacy, me, "pending")

	captainAndCrew := leg("captain, crew and sign-up", upcoming(standalone, "2027-07-10"))
	exec(`UPDATE trips SET captain_person_id = $1 WHERE id = $2`, mePerson, captainAndCrew)
	crew(captainAndCrew, mePerson)
	signUp(captainAndCrew, mePerson, "accepted")
	crewAndSignUp := leg("crew and sign-up", upcoming(standalone, "2027-08-10"))
	crew(crewAndSignUp, mePerson)
	signUp(crewAndSignUp, mePerson, "accepted")
	acceptedOverLegacyCruise := ownCruise("accepted over legacy")
	acceptedOverLegacy := leg("accepted sign-up over pending cruise enrollment", upcoming(acceptedOverLegacyCruise, "2027-09-10"))
	signUp(acceptedOverLegacy, mePerson, "accepted")
	cruiseEnroll(acceptedOverLegacyCruise, acceptedOverLegacy, me, "pending")
	pendingOverWaitlistedCruise := ownCruise("pending over waitlisted")
	pendingOverWaitlisted := leg("pending cruise enrollment over waitlisted sign-up", upcoming(pendingOverWaitlistedCruise, "2027-10-10"))
	signUp(pendingOverWaitlisted, mePerson, "waitlisted")
	cruiseEnroll(pendingOverWaitlistedCruise, pendingOverWaitlisted, me, "pending")

	endsToday := leg("ends today", legSpec{cruise: standalone, embark: "2026-11-08", disemb: today, published: true})
	crew(endsToday, mePerson)
	embarkOnlyAhead := leg("embark date only, ahead", legSpec{cruise: standalone, embark: "2026-12-01", published: true})
	crew(embarkOnlyAhead, mePerson)
	undated := leg("undated", legSpec{cruise: standalone, published: true})
	signUp(undated, mePerson, "pending")
	blankDates := leg("blank dates", legSpec{cruise: standalone, embark: "", disemb: "", published: true})
	crew(blankDates, mePerson)

	hiddenLeg := leg("hidden leg", legSpec{cruise: standalone, embark: "2027-11-10", disemb: "2027-11-12"})
	crew(hiddenLeg, mePerson)
	legOfHiddenCruise := leg("leg of a hidden cruise", legSpec{cruise: hiddenCruise, embark: "2027-12-10", disemb: "2027-12-12", published: true})
	crew(legOfHiddenCruise, mePerson)

	excluded := map[string]int64{}
	exclude := func(name string, s legSpec) int64 {
		id := leg(name, s)
		excluded[name] = id
		return id
	}
	signUp(exclude("rejected sign-up", upcoming(standalone, "2027-01-11")), mePerson, "rejected")
	rejectedCruise := ownCruise("rejected")
	cruiseEnroll(rejectedCruise, exclude("rejected cruise enrollment", upcoming(rejectedCruise, "2027-01-12")), me, "rejected")
	cancelled := exclude("cancelled", legSpec{cruise: standalone, embark: "2027-01-13", disemb: "2027-01-14", status: "cancelled", published: true})
	crew(cancelled, mePerson)
	crew(exclude("past", legSpec{cruise: standalone, embark: "2026-11-01", disemb: "2026-11-14", published: true}), mePerson)
	crew(exclude("embark date only, past", legSpec{cruise: standalone, embark: "2026-11-14", published: true}), mePerson)
	othersLeg := exclude("someone else's", legSpec{cruise: standalone, embark: "2027-01-15", disemb: "2027-01-16", published: true, captainPersonID: otherPerson})
	crew(othersLeg, otherPerson)
	signUp(othersLeg, otherPerson, "accepted")
	cruiseEnroll(standalone, othersLeg, other, "accepted")
	crew(exclude("person without an account with my e-mail", upcoming(standalone, "2027-01-18")), sameEmailPerson)
	unassignedCruise := ownCruise("unassigned")
	exclude("leg of a cruise enrollment not assigned to a leg", upcoming(unassignedCruise, "2027-01-19"))
	cruiseEnroll(unassignedCruise, nil, me, "accepted")

	type got struct {
		id       int64
		relation string
	}
	list := func(includeHidden bool) []got {
		t.Helper()
		rows, err := sqlcdb.New(conn).ListUserUpcomingLegs(t.Context(), sqlcdb.ListUserUpcomingLegsParams{
			UserID: me, Today: today, IncludeHidden: includeHidden,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		out := make([]got, len(rows))
		for i, r := range rows {
			out[i] = got{r.ID, r.Relation}
			if r.ID == captained {
				if !r.SeriesName.Valid || r.SeriesName.String != "Sycylia" || r.Edition.String != "2027" ||
					r.CruiseName != "Sycylia 2027" || r.CruiseID != edition || r.SeriesID.Int64 != series {
					t.Errorf("captained leg labels = %+v, want series Sycylia edition 2027", r)
				}
			}
			if r.ID == accepted && (r.SeriesName.Valid || r.Edition.Valid || r.CruiseName != "Bałtyk") {
				t.Errorf("standalone leg labels = %+v, want cruise Bałtyk without a series", r)
			}
		}
		return out
	}

	visible := []got{
		{endsToday, "crew"},
		{embarkOnlyAhead, "crew"},
		{captained, "captain"},
		{crewed, "crew"},
		{accepted, "accepted"},
		{pending, "pending"},
		{waitlisted, "waitlisted"},
		{legacy, "pending"},
		{captainAndCrew, "captain"},
		{crewAndSignUp, "crew"},
		{acceptedOverLegacy, "accepted"},
		{pendingOverWaitlisted, "pending"},
	}
	undatedLegs := []got{{undated, "pending"}, {blankDates, "crew"}}

	t.Run("member", func(t *testing.T) {
		want := slices.Concat(visible, undatedLegs)
		if g := list(false); !slices.Equal(g, want) {
			t.Fatalf("legs = %v\nwant   %v\nexcluded %v, hidden %d %d", g, want, excluded, hiddenLeg, legOfHiddenCruise)
		}
	})

	t.Run("include hidden", func(t *testing.T) {
		want := slices.Concat(visible, []got{{hiddenLeg, "crew"}, {legOfHiddenCruise, "crew"}}, undatedLegs)
		if g := list(true); !slices.Equal(g, want) {
			t.Fatalf("legs = %v\nwant   %v\nexcluded %v", g, want, excluded)
		}
	})
}
