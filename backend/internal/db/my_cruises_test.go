package db

import (
	"context"
	"slices"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestListUserCruiseIDs(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var me, other int64
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('jan@example.dev', 'Jan') RETURNING id`), &me)
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('ola@example.dev', 'Ola') RETURNING id`), &other)

	cruise := func(name string) int64 {
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO cruises (name) VALUES ($1) RETURNING id`, name), &id)
		return id
	}
	trip := func(cruiseID int64, captain any) int64 {
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO trips (name, cruise_id, captain_user_id) VALUES ('leg', $1, $2) RETURNING id`,
			cruiseID, captain), &id)
		return id
	}
	voyage := func(cruiseID int64, captain any) int64 {
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO voyages (name, cruise_id, captain_user_id) VALUES ('leg', $1, $2) RETURNING id`,
			cruiseID, captain), &id)
		return id
	}
	crew := func(userID any, email any) int64 {
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO crew_members (full_name, user_id, email) VALUES ('crew', $1, $2) RETURNING id`,
			userID, email), &id)
		return id
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}

	acceptedCruise := cruise("accepted cruise enrollment")
	exec(`INSERT INTO cruise_enrollments (cruise_id, user_id, status) VALUES ($1, $2, 'accepted')`, acceptedCruise, me)

	pendingCruise := cruise("pending cruise enrollment")
	exec(`INSERT INTO cruise_enrollments (cruise_id, user_id, status) VALUES ($1, $2, 'pending')`, pendingCruise, me)

	acceptedTrip := cruise("accepted trip enrollment")
	exec(`INSERT INTO trip_enrollments (trip_id, user_id, status) VALUES ($1, $2, 'accepted')`, trip(acceptedTrip, nil), me)

	captainedTrip := cruise("captained trip")
	trip(captainedTrip, me)

	captainedVoyage := cruise("captained voyage")
	voyage(captainedVoyage, me)

	linkedCrew := cruise("crew linked by account")
	exec(`INSERT INTO crew_assignments (trip_id, crew_member_id) VALUES ($1, $2)`, trip(linkedCrew, nil), crew(me, nil))

	emailCrew := cruise("crew matched by email")
	exec(`INSERT INTO crew_assignments (voyage_id, crew_member_id) VALUES ($1, $2)`,
		voyage(emailCrew, nil), crew(nil, "JAN@example.dev"))

	othersCruise := cruise("someone else's cruise")
	exec(`INSERT INTO crew_assignments (voyage_id, crew_member_id) VALUES ($1, $2)`,
		voyage(othersCruise, other), crew(other, "ola@example.dev"))
	exec(`INSERT INTO cruise_enrollments (cruise_id, user_id, status) VALUES ($1, $2, 'accepted')`, othersCruise, other)

	var previousOwner int64
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('ex-jan@example.dev', 'Ex') RETURNING id`), &previousOwner)
	staleEmailCruise := cruise("another account's person with my email")
	exec(`INSERT INTO crew_assignments (voyage_id, crew_member_id) VALUES ($1, $2)`,
		voyage(staleEmailCruise, nil), crew(previousOwner, "jan@example.dev"))

	got, err := sqlcdb.New(conn).ListUserCruiseIDs(context.Background(), me)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []int64{acceptedCruise, acceptedTrip, captainedTrip, captainedVoyage, linkedCrew, emailCrew}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("cruise ids = %v, want %v (pending %d, others %d, stale e-mail %d excluded)",
			got, want, pendingCruise, othersCruise, staleEmailCruise)
	}
}
