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
}

var devUsers = []devUser{
	{Email: "kasia.dev@gmail.com", Name: "Kasia Kapitan", Sub: "dev-google-captain"},
	{Email: "marek.dev@gmail.com", Name: "Marek Zaloga", Sub: "dev-google-crew"},
}

func main() {
	ctx := context.Background()
	cfg := config.Load()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	authHost := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST")
	if authHost == "" {
		authHost = "localhost:9099"
	}
	authBaseURL := "http://" + strings.TrimPrefix(authHost, "http://")

	for _, user := range devUsers {
		uid, err := ensureFirebaseUser(ctx, authBaseURL, user)
		if err != nil {
			log.Fatalf("seed firebase user %s: %v", user.Email, err)
		}
		userID, err := upsertDBUser(ctx, database, user, uid)
		if err != nil {
			log.Fatalf("seed database user %s: %v", user.Email, err)
		}
		if err := seedUserData(ctx, database, userID, user); err != nil {
			log.Fatalf("seed app data for %s: %v", user.Email, err)
		}
	}

	log.Printf("dev Google users ready: %s, %s", devUsers[0].Email, devUsers[1].Email)
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
	url := fmt.Sprintf("%s/identitytoolkit.googleapis.com/v1/%s?key=fake-api-key", baseURL, method)
	var lastErr error
	client := &http.Client{Timeout: 3 * time.Second}
	for attempt := range 20 {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
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
				updated_at = CURRENT_TIMESTAMP
			WHERE email = $1
			RETURNING id
		),
		inserted AS (
			INSERT INTO users (email, name, password_hash, firebase_uid)
			SELECT $1, $2, '', $3
			WHERE NOT EXISTS (SELECT 1 FROM updated)
			RETURNING id
		)
		SELECT id FROM updated
		UNION ALL
		SELECT id FROM inserted
	`, user.Email, user.Name, firebaseUID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}
	return id, nil
}

func seedUserData(ctx context.Context, database *sql.DB, userID int64, user devUser) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var yachtID int64
	err = tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO yachts (owner_id, name, registration_no, yacht_type)
			SELECT $1, 'S/Y Polarna', 'POL-4242', 'Bavaria 37'
			WHERE NOT EXISTS (
				SELECT 1 FROM yachts WHERE owner_id = $1 AND name = 'S/Y Polarna' AND org_id IS NULL
			)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM yachts WHERE owner_id = $1 AND name = 'S/Y Polarna' AND org_id IS NULL
		LIMIT 1
	`, userID).Scan(&yachtID)
	if err != nil {
		return fmt.Errorf("seed yacht: %w", err)
	}

	crewIDs := make([]int64, 0, 3)
	for _, crew := range []struct {
		name   string
		email  string
		patent string
	}{
		{name: "Anna Nowak", email: "anna@example.dev", patent: "JSM-1024"},
		{name: "Piotr Kowalski", email: "piotr@example.dev", patent: "ISS-2048"},
		{name: user.Name, email: user.Email, patent: "KPT-0001"},
	} {
		var crewID int64
		err = tx.QueryRowContext(ctx, `
			WITH inserted AS (
				INSERT INTO crew_members (owner_id, full_name, email, patent_number)
				SELECT $1, $2, $3, $4
				WHERE NOT EXISTS (
					SELECT 1 FROM crew_members WHERE owner_id = $1 AND full_name = $2 AND org_id IS NULL
				)
				RETURNING id
			)
			SELECT id FROM inserted
			UNION ALL
			SELECT id FROM crew_members WHERE owner_id = $1 AND full_name = $2 AND org_id IS NULL
			LIMIT 1
		`, userID, crew.name, crew.email, crew.patent).Scan(&crewID)
		if err != nil {
			return fmt.Errorf("seed crew %s: %w", crew.name, err)
		}
		crewIDs = append(crewIDs, crewID)
	}

	personalVoyages := []voyageSeed{
		{
			name: "Baltyk 2025", year: 2025,
			embark: "2025-07-04", disembark: "2025-07-12",
			countries: "Polska, Szwecja", startPort: "Gdansk", endPort: "Visby",
			hoursTotal: 84, hoursSail: 52, hoursEngine: 18, hoursOver6bf: 6,
			miles: 410, days: 8, tidalWaters: 1,
			costTotal: 12400, costPerPerson: 3100,
			description: "Rejs testowy z gotowa zaloga i kosztami.",
		},
		{
			name: "Norwegia 2024", year: 2024,
			embark: "2024-06-15", disembark: "2024-06-28",
			countries: "Norwegia", startPort: "Bergen", endPort: "Tromso",
			hoursTotal: 168, hoursSail: 110, hoursEngine: 42, hoursOver6bf: 22,
			miles: 980, days: 13, tidalWaters: 1,
			costTotal: 28000, costPerPerson: 4700,
			description: "Fjordy i archipelag Lofotow.",
		},
		{
			name: "Chorwacja 2023", year: 2023,
			embark: "2023-09-02", disembark: "2023-09-09",
			countries: "Chorwacja", startPort: "Split", endPort: "Dubrovnik",
			hoursTotal: 56, hoursSail: 38, hoursEngine: 14, hoursOver6bf: 0,
			miles: 270, days: 7, tidalWaters: 0,
			costTotal: 9800, costPerPerson: 2200,
			description: "Powrot na poludnie pod koniec sezonu.",
		},
	}

	voyageIDs := make([]int64, 0, len(personalVoyages))
	for _, v := range personalVoyages {
		id, verr := seedVoyageRow(ctx, tx, userID, yachtID, v)
		if verr != nil {
			err = verr
			return verr
		}
		voyageIDs = append(voyageIDs, id)
	}

	personalTrips := []tripSeed{
		{
			name: "Majowka Hel 2026",
			embark: "2026-05-23", disembark: "2026-05-26",
			countries: "Polska", startPort: "Hel", endPort: "Gdynia",
			captainName: user.Name, maxCrew: 6,
			costTotal: 12400, costPerPerson: 3100,
			description: "Krotki weekendowy wypad otwierajacy sezon.",
		},
		{
			name: "Dalmacja 2026",
			embark: "2026-09-12", disembark: "2026-09-20",
			countries: "Chorwacja", startPort: "Sibenik", endPort: "Trogir",
			captainName: user.Name, maxCrew: 8,
			costTotal: 16800, costPerPerson: 2400,
			description: "Wrzesniowy rejs po wyspach Chorwacji.",
		},
	}
	for _, t := range personalTrips {
		if _, terr := seedTripRow(ctx, tx, userID, yachtID, t); terr != nil {
			err = terr
			return terr
		}
	}

	// Crew assigned to the most recent past voyage (Baltyk 2025).
	for index, crewID := range crewIDs {
		role := "zalogant"
		if index == 2 {
			role = "kapitan"
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO crew_assignments (voyage_id, crew_member_id, role, patent_number)
			SELECT $1, $2, $3, NULL
			WHERE NOT EXISTS (
				SELECT 1 FROM crew_assignments WHERE voyage_id = $1 AND crew_member_id = $2
			)
		`, voyageIDs[0], crewID, role); err != nil {
			return fmt.Errorf("seed crew assignment: %w", err)
		}
	}

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
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO trainings (user_id, date, name, organizer, cost, url)
			SELECT $1, $2, $3, $4, $5, $6
			WHERE NOT EXISTS (
				SELECT 1 FROM trainings WHERE user_id = $1 AND name = $3 AND date = $2
			)
		`, userID, training.date, training.name, training.organizer, training.cost, training.url); err != nil {
			return fmt.Errorf("seed training %s: %w", training.name, err)
		}
	}

	orgID, err := seedOrg(ctx, tx, userID)
	if err != nil {
		return err
	}
	if err = seedOrgData(ctx, tx, userID, orgID); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit seed tx: %w", err)
	}
	return nil
}

type voyageSeed struct {
	name                                                         string
	year                                                         int64
	embark, disembark, countries, startPort, endPort             string
	hoursTotal, hoursSail, hoursEngine, hoursOver6bf, miles      float64
	days, tidalWaters                                            int64
	costTotal, costPerPerson                                     float64
	description                                                  string
}

type tripSeed struct {
	name, embark, disembark, countries, startPort, endPort string
	captainName                                            string
	maxCrew                                                int64
	costTotal, costPerPerson                               float64
	description                                            string
}

func seedVoyageRow(ctx context.Context, tx *sql.Tx, userID, yachtID int64, v voyageSeed) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO voyages (
				owner_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
				hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days,
				captain_name, yacht_id, tidal_waters, cost_total, cost_per_person, description
			)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8,
				$9, $10, $11, $12, $13, $14,
				'Kasia Kapitan', $15, $16, $17, $18, $19
			WHERE NOT EXISTS (
				SELECT 1 FROM voyages WHERE owner_id = $1 AND name = $2 AND org_id IS NULL
			)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM voyages WHERE owner_id = $1 AND name = $2 AND org_id IS NULL
		LIMIT 1
	`, userID, v.name, v.year, v.embark, v.disembark, v.countries, v.startPort, v.endPort,
		v.hoursTotal, v.hoursSail, v.hoursEngine, v.hoursOver6bf, v.miles, v.days,
		yachtID, v.tidalWaters, v.costTotal, v.costPerPerson, v.description).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed voyage %s: %w", v.name, err)
	}
	return id, nil
}

func seedTripRow(ctx context.Context, tx *sql.Tx, userID, yachtID int64, t tripSeed) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO trips (
				owner_id, name, embark_date, disembark_date, countries, start_port, end_port,
				captain_name, yacht_id, cost_total, cost_per_person,
				description, max_crew, status
			)
			SELECT $1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11,
				$12, $13, 'planned'::trip_status
			WHERE NOT EXISTS (
				SELECT 1 FROM trips WHERE owner_id = $1 AND name = $2 AND org_id IS NULL
			)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM trips WHERE owner_id = $1 AND name = $2 AND org_id IS NULL
		LIMIT 1
	`, userID, t.name, t.embark, t.disembark, t.countries, t.startPort, t.endPort,
		t.captainName, yachtID, t.costTotal, t.costPerPerson, t.description, t.maxCrew).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed trip %s: %w", t.name, err)
	}
	return id, nil
}

func seedOrg(ctx context.Context, tx *sql.Tx, userID int64) (int64, error) {
	var orgID int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO organizations (name, slug, description, pzz_club_number, city, website)
			SELECT 'Klub Zeglarski Demo', 'demo-club', 'Dane testowe do pracy lokalnej.', 'PZZ-42', 'Gdansk', 'https://example.dev'
			WHERE NOT EXISTS (SELECT 1 FROM organizations WHERE slug = 'demo-club')
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM organizations WHERE slug = 'demo-club'
		LIMIT 1
	`).Scan(&orgID)
	if err != nil {
		return 0, fmt.Errorf("seed org: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO org_members (org_id, user_id, role)
		SELECT $1, $2, 'admin'
		WHERE NOT EXISTS (
			SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2
		)
	`, orgID, userID); err != nil {
		return 0, fmt.Errorf("seed org member: %w", err)
	}
	return orgID, nil
}

func seedOrgData(ctx context.Context, tx *sql.Tx, userID, orgID int64) error {
	yachtKlubowa, err := upsertOrgYacht(ctx, tx, userID, orgID, "S/Y Klubowa", "GDA-777", "Delphia 40")
	if err != nil {
		return err
	}
	yachtMaestro, err := upsertOrgYacht(ctx, tx, userID, orgID, "S/Y Maestro", "GDA-778", "Hanse 415")
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO crew_members (
			owner_id, org_id, full_name, email, patent_number, phone,
			pzz_license_type, pzz_license_number, emergency_contact_name, emergency_contact_phone
		)
		SELECT $1, $2, 'Ewa Klubowa', 'ewa@example.dev', 'JSM-777', '+48123123123',
			'Jachtowy sternik morski', 'PZZ-777', 'Jan Klubowy', '+48456456456'
		WHERE NOT EXISTS (SELECT 1 FROM crew_members WHERE org_id = $2 AND full_name = 'Ewa Klubowa')
	`, userID, orgID); err != nil {
		return fmt.Errorf("seed org crew: %w", err)
	}

	// Future event-level cruise with two child trips (one per yacht).
	bornholmID, err := upsertOrgCruise(ctx, tx, orgID,
		"Bornholm 2026 Flotylla",
		"2026-08-01", "2026-08-09",
		"Polska, Dania", "Kolobrzeg", "Nexo",
		"Klubowa flotylla na Bornholm. Dwa jachty, otwarte zapisy.",
		16, 3000,
	)
	if err != nil {
		return err
	}
	if err := upsertOrgTripWithCruise(ctx, tx, userID, orgID, bornholmID, yachtKlubowa,
		"Bornholm 2026 - S/Y Klubowa", "Kasia Kapitan",
		"2026-08-01", "2026-08-09", "Polska, Dania", "Kolobrzeg", "Nexo",
		"Jacht prowadzacy flotylli.", 8, 18000, 3000); err != nil {
		return err
	}
	if err := upsertOrgTripWithCruise(ctx, tx, userID, orgID, bornholmID, yachtMaestro,
		"Bornholm 2026 - S/Y Maestro", "Marek Zaloga",
		"2026-08-01", "2026-08-09", "Polska, Dania", "Kolobrzeg", "Nexo",
		"Drugi jacht klubowej flotylli.", 8, 18000, 3000); err != nil {
		return err
	}

	// One-time cleanup of legacy demo rows from earlier seed-dev versions.
	if _, err := tx.ExecContext(ctx, `DELETE FROM voyages WHERE org_id = $1 AND name LIKE 'Mazury 2024%'`, orgID); err != nil {
		return fmt.Errorf("cleanup legacy mazury voyages: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cruises WHERE org_id = $1 AND name = 'Mazury 2024'`, orgID); err != nil {
		return fmt.Errorf("cleanup legacy mazury cruise: %w", err)
	}

	// Past event-level cruise with two completed voyages — shows the roll-up of stats.
	sztokholmID, err := upsertOrgCruise(ctx, tx, orgID,
		"Sztokholm 2024",
		"2024-07-06", "2024-07-13",
		"Polska, Szwecja", "Gdansk", "Sztokholm",
		"Klubowa flotylla przez Baltyk do archipelagu sztokholmskiego.",
		16, 3400,
	)
	if err != nil {
		return err
	}
	if err := upsertOrgVoyageWithCruise(ctx, tx, userID, orgID, sztokholmID, yachtKlubowa,
		"Sztokholm 2024 - S/Y Klubowa", 2024,
		"2024-07-06", "2024-07-13", "Polska, Szwecja", "Gdansk", "Sztokholm",
		82, 60, 22, 8, 380, 7, 0,
		16800, 2800, "Jacht prowadzacy - przejscie przez Gotland."); err != nil {
		return err
	}
	if err := upsertOrgVoyageWithCruise(ctx, tx, userID, orgID, sztokholmID, yachtMaestro,
		"Sztokholm 2024 - S/Y Maestro", 2024,
		"2024-07-06", "2024-07-13", "Polska, Szwecja", "Gdansk", "Sztokholm",
		86, 58, 28, 10, 395, 7, 0,
		17400, 2900, "Drugi jacht klubowej flotylli."); err != nil {
		return err
	}

	return nil
}

func upsertOrgYacht(ctx context.Context, tx *sql.Tx, userID, orgID int64, name, regNo, yachtType string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO yachts (owner_id, org_id, name, registration_no, yacht_type)
			SELECT $1, $2, $3, $4, $5
			WHERE NOT EXISTS (SELECT 1 FROM yachts WHERE org_id = $2 AND name = $3)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM yachts WHERE org_id = $2 AND name = $3
		LIMIT 1
	`, userID, orgID, name, regNo, yachtType).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed org yacht %s: %w", name, err)
	}
	return id, nil
}

func upsertOrgCruise(ctx context.Context, tx *sql.Tx, orgID int64,
	name, embark, disembark, countries, startPort, endPort, description string,
	maxCrew int64, costPerPerson float64,
) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO cruises (
				org_id, name, embark_date, disembark_date, countries, start_port, end_port,
				description, max_crew, cost_per_person
			)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			WHERE NOT EXISTS (SELECT 1 FROM cruises WHERE org_id = $1 AND name = $2)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM cruises WHERE org_id = $1 AND name = $2
		LIMIT 1
	`, orgID, name, embark, disembark, countries, startPort, endPort, description, maxCrew, costPerPerson).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed org cruise %s: %w", name, err)
	}
	return id, nil
}

func upsertOrgTripWithCruise(ctx context.Context, tx *sql.Tx,
	userID, orgID, cruiseID, yachtID int64,
	name, captain, embark, disembark, countries, startPort, endPort, description string,
	maxCrew int64, costTotal, costPerPerson float64,
) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO trips (
			owner_id, org_id, cruise_id, name, embark_date, disembark_date, countries, start_port, end_port,
			captain_name, yacht_id, cost_total, cost_per_person, description, max_crew, status
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, 'planned'::trip_status
		WHERE NOT EXISTS (SELECT 1 FROM trips WHERE org_id = $2 AND name = $4)
	`, userID, orgID, cruiseID, name, embark, disembark, countries, startPort, endPort,
		captain, yachtID, costTotal, costPerPerson, description, maxCrew); err != nil {
		return fmt.Errorf("seed org trip %s: %w", name, err)
	}
	return nil
}

func upsertOrgVoyageWithCruise(ctx context.Context, tx *sql.Tx,
	userID, orgID, cruiseID, yachtID int64,
	name string, year int64,
	embark, disembark, countries, startPort, endPort string,
	hoursTotal, hoursSail, hoursEngine, hoursOver6bf, miles float64,
	days, tidalWaters int64,
	costTotal, costPerPerson float64,
	description string,
) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO voyages (
			owner_id, org_id, cruise_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
			hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days,
			yacht_id, tidal_waters, cost_total, cost_per_person, description
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21
		WHERE NOT EXISTS (SELECT 1 FROM voyages WHERE org_id = $2 AND name = $4)
	`, userID, orgID, cruiseID, name, year, embark, disembark, countries, startPort, endPort,
		hoursTotal, hoursSail, hoursEngine, hoursOver6bf, miles, days,
		yachtID, tidalWaters, costTotal, costPerPerson, description); err != nil {
		return fmt.Errorf("seed org voyage %s: %w", name, err)
	}
	return nil
}
