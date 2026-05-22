package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

type devUser struct {
	Email string
	Name  string
	Sub   string
	Role  string
}

// devUsers are the seeded club members. The first is the club admin; the rest
// are regular members. Club data is shared, so every member sees the same
// yachts, crew, cruises, trips and voyages.
var devUsers = []devUser{
	{Email: "kasia.dev@gmail.com", Name: "Kasia Admin", Sub: "dev-google-captain", Role: "admin"},
	{Email: "marek.dev@gmail.com", Name: "Marek Zaloga", Sub: "dev-google-crew", Role: "member"},
	{Email: "jan.dev@gmail.com", Name: "Jan Kapitan", Sub: "dev-google-jan", Role: "member"},
	{Email: "aneta.dev@gmail.com", Name: "Aneta Solo", Sub: "dev-google-aneta", Role: "member"},
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	cfg := config.Load()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	if err := db.Migrate(database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	authHost := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST")
	if authHost == "" {
		authHost = "localhost:9099"
	}
	authBaseURL := "http://" + strings.TrimPrefix(authHost, "http://")

	var adminUserID int64
	for i, user := range devUsers {
		uid, err := ensureFirebaseUser(ctx, authBaseURL, user)
		if err != nil {
			return fmt.Errorf("seed firebase user %s: %w", user.Email, err)
		}
		userID, err := upsertDBUser(ctx, database, user, uid)
		if err != nil {
			return fmt.Errorf("seed database user %s: %w", user.Email, err)
		}
		if i == 0 {
			adminUserID = userID
		}
		if err := seedTrainings(ctx, database, userID); err != nil {
			return fmt.Errorf("seed trainings for %s: %w", user.Email, err)
		}
	}

	if err := seedClubData(ctx, database, adminUserID); err != nil {
		return fmt.Errorf("seed club data: %w", err)
	}

	emails := make([]string, len(devUsers))
	for i, u := range devUsers {
		emails[i] = u.Email
	}
	log.Printf("dev Google users ready: %s", strings.Join(emails, ", "))
	return nil
}

func ensureFirebaseUser(ctx context.Context, baseURL string, user devUser) (string, error) {
	idToken, err := json.Marshal(map[string]any{
		"sub":            user.Sub,
		"email":          user.Email,
		"email_verified": true,
		"name":           user.Name,
		"picture":        fmt.Sprintf("https://example.dev/%s.png", user.Sub),
	})
	if err != nil {
		return "", fmt.Errorf("marshal mock Google token: %w", err)
	}
	body := map[string]any{
		"requestUri":          "http://localhost:5173",
		"postBody":            url.Values{"providerId": {"google.com"}, "id_token": {string(idToken)}}.Encode(),
		"returnIdpCredential": true,
		"returnSecureToken":   true,
	}
	var out struct {
		LocalID string `json:"localId"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := firebasePost(ctx, baseURL, "accounts:signInWithIdp", body, &out); err != nil {
		return "", err
	}
	if out.Error.Message != "" {
		return "", firebaseError(out.Error.Message)
	}
	if out.LocalID == "" {
		return "", errors.New("missing localId from signInWithIdp")
	}
	return out.LocalID, nil
}

func firebasePost(ctx context.Context, baseURL, method string, body map[string]any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal firebase payload: %w", err)
	}
	endpoint := fmt.Sprintf("%s/identitytoolkit.googleapis.com/v1/%s?key=fake-api-key", baseURL, method)
	var lastErr error
	client := &http.Client{Timeout: 3 * time.Second}
	for attempt := range 20 {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build firebase request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
			continue
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("read firebase response: %w", err)
		}
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("close firebase response: %w", err)
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("firebase status %d: %s", resp.StatusCode, string(respBody))
			time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
			continue
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode firebase response: %w", err)
		}
		return nil
	}
	return fmt.Errorf("firebase emulator unavailable: %w", lastErr)
}

type firebaseError string

func (e firebaseError) Error() string {
	return string(e)
}

func upsertDBUser(ctx context.Context, database *sql.DB, user devUser, firebaseUID string) (int64, error) {
	var id int64
	err := database.QueryRowContext(ctx, `
		WITH updated AS (
			UPDATE users SET
				name = $2,
				firebase_uid = $3,
				role = $4,
				updated_at = CURRENT_TIMESTAMP
			WHERE email = $1
			RETURNING id
		),
		inserted AS (
			INSERT INTO users (email, name, password_hash, firebase_uid, role)
			SELECT $1, $2, '', $3, $4
			WHERE NOT EXISTS (SELECT 1 FROM updated)
			RETURNING id
		)
		SELECT id FROM updated
		UNION ALL
		SELECT id FROM inserted
	`, user.Email, user.Name, firebaseUID, user.Role).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}
	return id, nil
}

func seedTrainings(ctx context.Context, database *sql.DB, userID int64) error {
	for _, training := range []struct {
		date      string
		name      string
		organizer string
		cost      float64
		url       string
	}{
		{date: "2025-03-12", name: "SRC", organizer: "Sail Training Center", cost: 650, url: "https://example.dev/src"},
		{date: "2025-11-08", name: "Pierwsza pomoc", organizer: "WOPR", cost: 280, url: "https://example.dev/first-aid"},
	} {
		if _, err := database.ExecContext(ctx, `
			INSERT INTO trainings (user_id, date, name, organizer, cost, url)
			SELECT $1, $2, $3, $4, $5, $6
			WHERE NOT EXISTS (
				SELECT 1 FROM trainings WHERE user_id = $1 AND name = $3 AND date = $2
			)
		`, userID, training.date, training.name, training.organizer, training.cost, training.url); err != nil {
			return fmt.Errorf("seed training %s: %w", training.name, err)
		}
	}
	return nil
}

// seedClubData seeds the shared club records once. createdBy is the admin user.
func seedClubData(ctx context.Context, database *sql.DB, createdBy int64) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	yachtKlubowa, err := upsertYacht(ctx, tx, createdBy, "S/Y Klubowa", "GDA-777", "Delphia 40")
	if err != nil {
		return err
	}
	yachtMaestro, err := upsertYacht(ctx, tx, createdBy, "S/Y Maestro", "GDA-778", "Hanse 415")
	if err != nil {
		return err
	}

	crewIDs, err := seedCrew(ctx, tx, createdBy)
	if err != nil {
		return err
	}

	// Future event-level cruise with two child trips (one per yacht).
	bornholmID, err := upsertCruise(ctx, tx, createdBy,
		"Bornholm 2026 Flotylla",
		"2026-08-01", "2026-08-09",
		"Polska, Dania", "Kolobrzeg", "Nexo",
		"Klubowa flotylla na Bornholm. Dwa jachty, otwarte zapisy.",
		16, 3000,
	)
	if err != nil {
		return err
	}
	if err := upsertTrip(ctx, tx, createdBy, bornholmID, yachtKlubowa,
		"Bornholm 2026 - S/Y Klubowa", "Kasia Admin",
		"2026-08-01", "2026-08-09", "Polska, Dania", "Kolobrzeg", "Nexo",
		"Jacht prowadzacy flotylli.", 8, 18000, 3000); err != nil {
		return err
	}
	if err := upsertTrip(ctx, tx, createdBy, bornholmID, yachtMaestro,
		"Bornholm 2026 - S/Y Maestro", "Marek Zaloga",
		"2026-08-01", "2026-08-09", "Polska, Dania", "Kolobrzeg", "Nexo",
		"Drugi jacht klubowej flotylli.", 8, 18000, 3000); err != nil {
		return err
	}

	// Past event-level cruise with two completed voyages — shows the roll-up.
	sztokholmID, err := upsertCruise(ctx, tx, createdBy,
		"Sztokholm 2024",
		"2024-07-06", "2024-07-13",
		"Polska, Szwecja", "Gdansk", "Sztokholm",
		"Klubowa flotylla przez Baltyk do archipelagu sztokholmskiego.",
		16, 3400,
	)
	if err != nil {
		return err
	}
	voyageID, err := upsertVoyage(ctx, tx, createdBy, sztokholmID, yachtKlubowa,
		"Sztokholm 2024 - S/Y Klubowa", 2024,
		"2024-07-06", "2024-07-13", "Polska, Szwecja", "Gdansk", "Sztokholm",
		82, 60, 22, 8, 380, 7, 0,
		16800, 2800, "Jacht prowadzacy - przejscie przez Gotland.")
	if err != nil {
		return err
	}
	if _, err := upsertVoyage(ctx, tx, createdBy, sztokholmID, yachtMaestro,
		"Sztokholm 2024 - S/Y Maestro", 2024,
		"2024-07-06", "2024-07-13", "Polska, Szwecja", "Gdansk", "Sztokholm",
		86, 58, 28, 10, 395, 7, 0,
		17400, 2900, "Drugi jacht klubowej flotylli."); err != nil {
		return err
	}

	if err := seedCrewAssignments(ctx, tx, crewIDs, voyageID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed tx: %w", err)
	}
	return nil
}

func upsertYacht(ctx context.Context, tx *sql.Tx, createdBy int64, name, regNo, yachtType string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO yachts (created_by, name, registration_no, yacht_type)
			SELECT $1, $2, $3, $4
			WHERE NOT EXISTS (SELECT 1 FROM yachts WHERE name = $2)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM yachts WHERE name = $2
		LIMIT 1
	`, createdBy, name, regNo, yachtType).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed yacht %s: %w", name, err)
	}
	return id, nil
}

func seedCrew(ctx context.Context, tx *sql.Tx, createdBy int64) ([]int64, error) {
	crewIDs := make([]int64, 0, 3)
	for _, crew := range []struct {
		name   string
		email  string
		patent string
	}{
		{name: "Anna Nowak", email: "anna@example.dev", patent: "JSM-1024"},
		{name: "Piotr Kowalski", email: "piotr@example.dev", patent: "ISS-2048"},
		{name: "Ewa Klubowa", email: "ewa@example.dev", patent: "KPT-0001"},
	} {
		var crewID int64
		err := tx.QueryRowContext(ctx, `
			WITH inserted AS (
				INSERT INTO crew_members (created_by, full_name, email, patent_number)
				SELECT $1, $2, $3, $4
				WHERE NOT EXISTS (SELECT 1 FROM crew_members WHERE full_name = $2)
				RETURNING id
			)
			SELECT id FROM inserted
			UNION ALL
			SELECT id FROM crew_members WHERE full_name = $2
			LIMIT 1
		`, createdBy, crew.name, crew.email, crew.patent).Scan(&crewID)
		if err != nil {
			return nil, fmt.Errorf("seed crew %s: %w", crew.name, err)
		}
		crewIDs = append(crewIDs, crewID)
	}
	return crewIDs, nil
}

// seedCrewAssignments assigns the seeded crew to a completed voyage.
func seedCrewAssignments(ctx context.Context, tx *sql.Tx, crewIDs []int64, voyageID int64) error {
	for index, crewID := range crewIDs {
		role := "zalogant"
		if index == 2 {
			role = "kapitan"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO crew_assignments (voyage_id, crew_member_id, role, patent_number)
			SELECT $1, $2, $3, NULL
			WHERE NOT EXISTS (
				SELECT 1 FROM crew_assignments WHERE voyage_id = $1 AND crew_member_id = $2
			)
		`, voyageID, crewID, role); err != nil {
			return fmt.Errorf("seed crew assignment: %w", err)
		}
	}
	return nil
}

func upsertCruise(ctx context.Context, tx *sql.Tx, createdBy int64,
	name, embark, disembark, countries, startPort, endPort, description string,
	maxCrew int64, costPerPerson float64,
) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO cruises (
				created_by, name, embark_date, disembark_date, countries, start_port, end_port,
				description, max_crew, cost_per_person
			)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			WHERE NOT EXISTS (SELECT 1 FROM cruises WHERE name = $2)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM cruises WHERE name = $2
		LIMIT 1
	`, createdBy, name, embark, disembark, countries, startPort, endPort, description, maxCrew, costPerPerson).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed cruise %s: %w", name, err)
	}
	return id, nil
}

func upsertTrip(ctx context.Context, tx *sql.Tx,
	createdBy, cruiseID, yachtID int64,
	name, captain, embark, disembark, countries, startPort, endPort, description string,
	maxCrew int64, costTotal, costPerPerson float64,
) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO trips (
			created_by, cruise_id, name, embark_date, disembark_date, countries, start_port, end_port,
			captain_name, yacht_id, cost_total, cost_per_person, description, max_crew, status
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, 'planned'::trip_status
		WHERE NOT EXISTS (SELECT 1 FROM trips WHERE name = $3)
	`, createdBy, cruiseID, name, embark, disembark, countries, startPort, endPort,
		captain, yachtID, costTotal, costPerPerson, description, maxCrew); err != nil {
		return fmt.Errorf("seed trip %s: %w", name, err)
	}
	return nil
}

func upsertVoyage(ctx context.Context, tx *sql.Tx,
	createdBy, cruiseID, yachtID int64,
	name string, year int64,
	embark, disembark, countries, startPort, endPort string,
	hoursTotal, hoursSail, hoursEngine, hoursOver6bf, miles float64,
	days, tidalWaters int64,
	costTotal, costPerPerson float64,
	description string,
) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO voyages (
				created_by, cruise_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
				hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days,
				yacht_id, tidal_waters, cost_total, cost_per_person, description
			)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9,
				$10, $11, $12, $13, $14, $15,
				$16, $17, $18, $19, $20
			WHERE NOT EXISTS (SELECT 1 FROM voyages WHERE name = $3)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM voyages WHERE name = $3
		LIMIT 1
	`, createdBy, cruiseID, name, year, embark, disembark, countries, startPort, endPort,
		hoursTotal, hoursSail, hoursEngine, hoursOver6bf, miles, days,
		yachtID, tidalWaters, costTotal, costPerPerson, description).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed voyage %s: %w", name, err)
	}
	return id, nil
}
