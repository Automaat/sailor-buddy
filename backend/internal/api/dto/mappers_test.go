package dto

import (
	"testing"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

var testTime = time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

func nstr(s string) types.NullString { return types.NullString{String: s, Valid: true} }
func nint(i int64) types.NullInt64   { return types.NullInt64{Int64: i, Valid: true} }
func nflt(f float64) types.NullFloat64 {
	return types.NullFloat64{Float64: f, Valid: true}
}
func ntime() types.NullTime { return types.NullTime{Time: testTime, Valid: true} }

func TestMeFromUser(t *testing.T) {
	t.Run("populated fields map through", func(t *testing.T) {
		u := sqlcdb.User{
			ID:           1,
			Email:        "a@example.com",
			Name:         "Anna",
			AvatarUrl:    nstr("https://img/a.png"),
			Role:         "admin",
			PatentType:   nstr("kapitan_jachtowy"),
			PatentNumber: nstr("KJ-42"),
		}
		me := MeFromUser(u)
		if me.ID != 1 || me.Email != "a@example.com" || me.Name != "Anna" {
			t.Fatalf("unexpected scalar fields: %+v", me)
		}
		if me.AvatarURL != "https://img/a.png" || me.Role != "admin" {
			t.Fatalf("unexpected avatar/role: %+v", me)
		}
		if me.PatentType == nil || *me.PatentType != "kapitan_jachtowy" {
			t.Fatalf("patent_type = %v", me.PatentType)
		}
		if me.PatentNumber == nil || *me.PatentNumber != "KJ-42" {
			t.Fatalf("patent_number = %v", me.PatentNumber)
		}
	})

	t.Run("null optionals become nil", func(t *testing.T) {
		me := MeFromUser(sqlcdb.User{ID: 2, Role: "member"})
		if me.AvatarURL != "" {
			t.Fatalf("want empty avatar, got %q", me.AvatarURL)
		}
		if me.PatentType != nil || me.PatentNumber != nil {
			t.Fatalf("want nil patent fields, got %v / %v", me.PatentType, me.PatentNumber)
		}
	})
}

func TestVoyagesByYearFromDB(t *testing.T) {
	out := VoyagesByYearFromDB([]sqlcdb.GetVoyagesByYearRow{
		{Year: nint(2023), VoyageCount: 4, TotalHours: 120.5, TotalMiles: 600, TotalDays: 20},
		{VoyageCount: 1, TotalHours: 10, TotalMiles: 30, TotalDays: 2},
	})
	if len(out) != 2 {
		t.Fatalf("want 2 rows, got %d", len(out))
	}
	if out[0].Year == nil || *out[0].Year != 2023 {
		t.Fatalf("year = %v", out[0].Year)
	}
	if out[0].VoyageCount != 4 || out[0].TotalHours != 120.5 {
		t.Fatalf("unexpected aggregates: %+v", out[0])
	}
	if out[1].Year != nil {
		t.Fatalf("want nil year for null row, got %v", *out[1].Year)
	}
}

func TestCrewMemberFromDB(t *testing.T) {
	m := sqlcdb.CrewMember{
		ID:                    7,
		CreatedBy:             nint(1),
		UserID:                nint(3),
		FullName:              "Bo Sailor",
		Email:                 nstr("bo@example.com"),
		PatentNumber:          nstr("P-1"),
		Phone:                 nstr("123"),
		PzzLicenseType:        nstr("A"),
		PzzLicenseNumber:      nstr("L-9"),
		EmergencyContactName:  nstr("Kin"),
		EmergencyContactPhone: nstr("999"),
		CreatedAt:             ntime(),
		UpdatedAt:             ntime(),
	}
	got := CrewMemberFromDB(m)
	if got.ID != 7 || got.FullName != "Bo Sailor" {
		t.Fatalf("unexpected scalars: %+v", got)
	}
	if got.CreatedBy == nil || *got.CreatedBy != 1 || got.UserID == nil || *got.UserID != 3 {
		t.Fatalf("unexpected created_by/user_id: %+v", got)
	}
	if got.Email == nil || *got.Email != "bo@example.com" {
		t.Fatalf("email = %v", got.Email)
	}
	if !got.CreatedAt.Equal(testTime) {
		t.Fatalf("created_at = %v", got.CreatedAt)
	}

	empty := CrewMemberFromDB(sqlcdb.CrewMember{ID: 8, FullName: "X"})
	if empty.Email != nil || empty.UserID != nil || empty.PatentNumber != nil {
		t.Fatalf("want nil optionals on bare row, got %+v", empty)
	}
}

func TestCrewMembersFromDB(t *testing.T) {
	out := CrewMembersFromDB([]sqlcdb.CrewMember{{ID: 1, FullName: "A"}, {ID: 2, FullName: "B"}})
	if len(out) != 2 || out[0].ID != 1 || out[1].ID != 2 {
		t.Fatalf("unexpected slice: %+v", out)
	}
	if CrewMembersFromDB(nil) == nil {
		t.Fatalf("want non-nil slice for nil input")
	}
}

func TestCrewAssignmentFromDB(t *testing.T) {
	a := sqlcdb.CrewAssignment{
		ID: 3, TripID: nint(5), CrewMemberID: 9,
		Role: "skipper", PatentNumber: nstr("KJ-1"), CreatedAt: ntime(),
	}
	got := CrewAssignmentFromDB(a)
	if got.ID != 3 || got.CrewMemberID != 9 || got.Role != "skipper" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.TripID == nil || *got.TripID != 5 || got.VoyageID != nil {
		t.Fatalf("unexpected trip/voyage id: %+v", got)
	}
}

func TestTripCrewFromDB(t *testing.T) {
	out := TripCrewFromDB([]sqlcdb.ListTripCrewAssignmentsRow{
		{
			ID: 1, TripID: nint(2), CrewMemberID: 4, Role: "mate",
			PatentNumber: nstr("P"), FullName: "Cara", Email: nstr("c@x"), CreatedAt: ntime(),
		},
	})
	if len(out) != 1 {
		t.Fatalf("want 1, got %d", len(out))
	}
	if out[0].FullName != "Cara" || out[0].TripID == nil || *out[0].TripID != 2 {
		t.Fatalf("unexpected: %+v", out[0])
	}
	if TripCrewFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestVoyageCrewFromDB(t *testing.T) {
	out := VoyageCrewFromDB([]sqlcdb.ListVoyageCrewAssignmentsRow{
		{ID: 1, VoyageID: nint(8), CrewMemberID: 4, Role: "mate", FullName: "Dee"},
	})
	if len(out) != 1 || out[0].FullName != "Dee" {
		t.Fatalf("unexpected: %+v", out)
	}
	if out[0].VoyageID == nil || *out[0].VoyageID != 8 {
		t.Fatalf("voyage_id = %v", out[0].VoyageID)
	}
	if VoyageCrewFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestCruiseFromDB(t *testing.T) {
	c := sqlcdb.Cruise{
		ID: 1, CreatedBy: nint(2), Name: "Baltic",
		EmbarkDate: nstr("2024-06-01"), MaxCrew: nint(8),
		CostPerPerson: nflt(150.5), EnrollToken: nstr("tok"),
		CreatedAt: ntime(), UpdatedAt: ntime(),
	}
	got := CruiseFromDB(c)
	if got.ID != 1 || got.Name != "Baltic" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.MaxCrew == nil || *got.MaxCrew != 8 {
		t.Fatalf("max_crew = %v", got.MaxCrew)
	}
	if got.CostPerPerson == nil || *got.CostPerPerson != 150.5 {
		t.Fatalf("cost_per_person = %v", got.CostPerPerson)
	}
	if got.EnrollToken == nil || *got.EnrollToken != "tok" {
		t.Fatalf("enroll_token = %v", got.EnrollToken)
	}

	bare := CruiseFromDB(sqlcdb.Cruise{ID: 2, Name: "X"})
	if bare.MaxCrew != nil || bare.CostPerPerson != nil || bare.EnrollToken != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
}

func TestCruisesFromDB(t *testing.T) {
	out := CruisesFromDB([]sqlcdb.Cruise{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}})
	if len(out) != 2 || out[1].Name != "B" {
		t.Fatalf("unexpected: %+v", out)
	}
	if CruisesFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestMemberFromUser(t *testing.T) {
	u := sqlcdb.User{
		ID: 1, Name: "Ed", Email: "ed@x", Role: "member",
		AvatarUrl: nstr("a.png"), PatentType: nstr("zeglarz_jachtowy"),
		PatentNumber: nstr("Z-1"), CreatedAt: ntime(),
	}
	got := MemberFromUser(u)
	if got.ID != 1 || got.Name != "Ed" || got.Role != "member" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.AvatarURL == nil || *got.AvatarURL != "a.png" {
		t.Fatalf("avatar = %v", got.AvatarURL)
	}
	if !got.CreatedAt.Equal(testTime) {
		t.Fatalf("created_at = %v", got.CreatedAt)
	}

	bare := MemberFromUser(sqlcdb.User{ID: 2})
	if bare.AvatarURL != nil || bare.PatentType != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
}

func TestMembersFromDB(t *testing.T) {
	out := MembersFromDB([]sqlcdb.User{{ID: 1}, {ID: 2}})
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
	if MembersFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestVoyageOpinionFromDB(t *testing.T) {
	got := VoyageOpinionFromDB(sqlcdb.VoyageOpinion{
		ID: 1, VoyageID: 2, CrewMemberID: 3, FileFormat: "pdf", CreatedAt: ntime(),
	})
	if got.ID != 1 || got.VoyageID != 2 || got.CrewMemberID != 3 || got.FileFormat != "pdf" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.FullName != "" {
		t.Fatalf("want empty full_name from bare row, got %q", got.FullName)
	}
}

func TestVoyageOpinionsFromDB(t *testing.T) {
	out := VoyageOpinionsFromDB([]sqlcdb.ListVoyageVoyageOpinionsRow{
		{ID: 1, VoyageID: 2, CrewMemberID: 3, FileFormat: "docx", FullName: "Fay", CreatedAt: ntime()},
	})
	if len(out) != 1 || out[0].FullName != "Fay" || out[0].FileFormat != "docx" {
		t.Fatalf("unexpected: %+v", out)
	}
	if VoyageOpinionsFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestVoyagePortFromDB(t *testing.T) {
	got := VoyagePortFromDB(sqlcdb.VoyagePort{
		ID: 1, VoyageID: 2, Name: "Gdańsk",
		Latitude: 54.35, Longitude: 18.65, Position: 0, CreatedAt: ntime(),
	})
	if got.Name != "Gdańsk" || got.Latitude != 54.35 || got.Longitude != 18.65 {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.VoyageID != 2 || got.Position != 0 {
		t.Fatalf("unexpected voyage/position: %+v", got)
	}
}

func TestVoyagePortsFromDB(t *testing.T) {
	out := VoyagePortsFromDB([]sqlcdb.VoyagePort{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}})
	if len(out) != 2 || out[0].Name != "A" {
		t.Fatalf("unexpected: %+v", out)
	}
	if VoyagePortsFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestTrainingFromDB(t *testing.T) {
	t.Run("with cost and url", func(t *testing.T) {
		got := TrainingFromDB(sqlcdb.Training{
			ID: 1, UserID: 2, Name: "ISSA", Date: nstr("2024-03-01"),
			Organizer: nstr("Club"), Cost: nflt(499.99), Url: nstr("https://x"),
			CreatedAt: ntime(), UpdatedAt: ntime(),
		})
		if got.ID != 1 || got.UserID != 2 || got.Name != "ISSA" {
			t.Fatalf("unexpected: %+v", got)
		}
		if got.Cost == nil || *got.Cost != 499.99 {
			t.Fatalf("cost = %v", got.Cost)
		}
		if got.Date == nil || *got.Date != "2024-03-01" {
			t.Fatalf("date = %v", got.Date)
		}
	})

	t.Run("null optionals become nil", func(t *testing.T) {
		got := TrainingFromDB(sqlcdb.Training{ID: 3, UserID: 4, Name: "Bare"})
		if got.Cost != nil || got.Date != nil || got.Organizer != nil || got.Url != nil {
			t.Fatalf("want nil optionals, got %+v", got)
		}
	})
}

func TestTrainingsFromDB(t *testing.T) {
	out := TrainingsFromDB([]sqlcdb.Training{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}})
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
	if TrainingsFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestTripFromDB(t *testing.T) {
	got := TripFromDB(sqlcdb.Trip{
		ID: 1, CreatedBy: nint(2), CruiseID: nint(3), Name: "Spring",
		Status: sqlcdb.TripStatus("planned"), YachtID: nint(4),
		CostTotal: nflt(1000), CostPerPerson: nflt(250), MaxCrew: nint(4),
		EnrollToken: nstr("tk"), CreatedAt: ntime(), UpdatedAt: ntime(),
	})
	if got.ID != 1 || got.Name != "Spring" || got.Status != "planned" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.CruiseID == nil || *got.CruiseID != 3 || got.YachtID == nil || *got.YachtID != 4 {
		t.Fatalf("unexpected cruise/yacht id: %+v", got)
	}
	if got.CostTotal == nil || *got.CostTotal != 1000 {
		t.Fatalf("cost_total = %v", got.CostTotal)
	}

	bare := TripFromDB(sqlcdb.Trip{ID: 2, Name: "X", Status: sqlcdb.TripStatus("cancelled")})
	if bare.CruiseID != nil || bare.YachtID != nil || bare.CostTotal != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
	if bare.Status != "cancelled" {
		t.Fatalf("status = %q", bare.Status)
	}
}

func TestTripsFromDB(t *testing.T) {
	out := TripsFromDB([]sqlcdb.Trip{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}})
	if len(out) != 2 || out[1].ID != 2 {
		t.Fatalf("unexpected: %+v", out)
	}
	if TripsFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestVoyageFromDB(t *testing.T) {
	got := VoyageFromDB(sqlcdb.Voyage{
		ID: 1, CreatedBy: nint(2), CruiseID: nint(3), Name: "Logged",
		Year: nint(2024), YachtID: nint(5),
		HoursTotal: 100, HoursSail: 60, HoursEngine: 40, HoursOver6bf: 5,
		Miles: 300, Days: 10, TidalWaters: 1,
		CostTotal: nflt(800), CostPerPerson: nflt(200),
		CreatedAt: ntime(), UpdatedAt: ntime(),
	})
	if got.ID != 1 || got.Name != "Logged" || got.HoursTotal != 100 {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.Year == nil || *got.Year != 2024 {
		t.Fatalf("year = %v", got.Year)
	}
	if got.Days != 10 || got.TidalWaters != 1 || got.Miles != 300 {
		t.Fatalf("unexpected numeric fields: %+v", got)
	}

	bare := VoyageFromDB(sqlcdb.Voyage{ID: 2, Name: "X"})
	if bare.Year != nil || bare.CruiseID != nil || bare.CostTotal != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
}

func TestVoyagesFromDB(t *testing.T) {
	out := VoyagesFromDB([]sqlcdb.Voyage{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}})
	if len(out) != 2 || out[0].Name != "A" {
		t.Fatalf("unexpected: %+v", out)
	}
	if VoyagesFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestYachtFromDB(t *testing.T) {
	got := YachtFromDB(sqlcdb.Yacht{
		ID: 1, CreatedBy: nint(2), Name: "Bavaria",
		RegistrationNo: nstr("REG-1"), YachtType: nstr("sailing"),
		CreatedAt: ntime(), UpdatedAt: ntime(),
	})
	if got.ID != 1 || got.Name != "Bavaria" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.RegistrationNo == nil || *got.RegistrationNo != "REG-1" {
		t.Fatalf("registration_no = %v", got.RegistrationNo)
	}

	bare := YachtFromDB(sqlcdb.Yacht{ID: 2, Name: "X"})
	if bare.RegistrationNo != nil || bare.YachtType != nil || bare.CreatedBy != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
}

func TestYachtsFromDB(t *testing.T) {
	out := YachtsFromDB([]sqlcdb.Yacht{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}})
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
	if YachtsFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestTripEnrollmentToDTO(t *testing.T) {
	got := TripEnrollmentToDTO(sqlcdb.TripEnrollment{
		ID: 1, TripID: 5, UserID: 9, Note: nstr("hi"),
		Status: "accepted", CreatedAt: ntime(), UpdatedAt: ntime(),
	})
	if got.ID != 1 || got.UserID != 9 || got.Status != "accepted" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.TripID == nil || *got.TripID != 5 {
		t.Fatalf("trip_id = %v", got.TripID)
	}
	if got.CruiseID != nil {
		t.Fatalf("want nil cruise_id, got %v", *got.CruiseID)
	}
	if got.Note == nil || *got.Note != "hi" {
		t.Fatalf("note = %v", got.Note)
	}
}

func TestCruiseEnrollmentToDTO(t *testing.T) {
	t.Run("assigned to a trip", func(t *testing.T) {
		got := CruiseEnrollmentToDTO(sqlcdb.CruiseEnrollment{
			ID: 1, CruiseID: 7, UserID: 9, TripID: nint(3),
			Status: "waitlisted", CreatedAt: ntime(), UpdatedAt: ntime(),
		})
		if got.CruiseID == nil || *got.CruiseID != 7 {
			t.Fatalf("cruise_id = %v", got.CruiseID)
		}
		if got.TripID == nil || *got.TripID != 3 {
			t.Fatalf("trip_id = %v", got.TripID)
		}
	})

	t.Run("unassigned has nil trip_id", func(t *testing.T) {
		got := CruiseEnrollmentToDTO(sqlcdb.CruiseEnrollment{ID: 2, CruiseID: 7, UserID: 9, Status: "pending"})
		if got.TripID != nil {
			t.Fatalf("want nil trip_id, got %v", *got.TripID)
		}
	})
}

func TestTripEnrollmentsFromDB(t *testing.T) {
	out := TripEnrollmentsFromDB([]sqlcdb.ListTripEnrollmentsRow{
		{
			ID: 1, TripID: 5, UserID: 9, Status: "accepted",
			UserName: "Gail", UserEmail: "gail@x", CreatedAt: ntime(), UpdatedAt: ntime(),
		},
	})
	if len(out) != 1 {
		t.Fatalf("want 1, got %d", len(out))
	}
	if out[0].UserName != "Gail" || out[0].UserEmail != "gail@x" {
		t.Fatalf("unexpected joined fields: %+v", out[0])
	}
	if out[0].TripID == nil || *out[0].TripID != 5 {
		t.Fatalf("trip_id = %v", out[0].TripID)
	}
	if TripEnrollmentsFromDB(nil) == nil {
		t.Fatalf("want non-nil slice")
	}
}

func TestEnrollTripFromRow(t *testing.T) {
	got := EnrollTripFromRow(sqlcdb.GetTripByEnrollTokenRow{
		ID: 1, Name: "Token Trip", EmbarkDate: nstr("2024-07-01"),
		MaxCrew: nint(6), CaptainName: nstr("Hank"),
	})
	if got.ID != 1 || got.Name != "Token Trip" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.MaxCrew == nil || *got.MaxCrew != 6 {
		t.Fatalf("max_crew = %v", got.MaxCrew)
	}
	if got.CaptainName == nil || *got.CaptainName != "Hank" {
		t.Fatalf("captain_name = %v", got.CaptainName)
	}

	bare := EnrollTripFromRow(sqlcdb.GetTripByEnrollTokenRow{ID: 2, Name: "X"})
	if bare.MaxCrew != nil || bare.CaptainName != nil || bare.EmbarkDate != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
}

func TestEnrollCruiseFromRow(t *testing.T) {
	got := EnrollCruiseFromRow(sqlcdb.GetCruiseByEnrollTokenRow{
		ID: 1, Name: "Token Cruise", MaxCrew: nint(10), CostPerPerson: nflt(300),
	})
	if got.ID != 1 || got.Name != "Token Cruise" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.MaxCrew == nil || *got.MaxCrew != 10 {
		t.Fatalf("max_crew = %v", got.MaxCrew)
	}
	if got.CostPerPerson == nil || *got.CostPerPerson != 300 {
		t.Fatalf("cost_per_person = %v", got.CostPerPerson)
	}

	bare := EnrollCruiseFromRow(sqlcdb.GetCruiseByEnrollTokenRow{ID: 2, Name: "X"})
	if bare.MaxCrew != nil || bare.CostPerPerson != nil {
		t.Fatalf("want nil optionals, got %+v", bare)
	}
}
