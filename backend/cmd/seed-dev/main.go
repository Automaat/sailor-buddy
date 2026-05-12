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
		INSERT INTO users (email, name, password_hash, firebase_uid)
		VALUES ($1, $2, '', $3)
		ON CONFLICT (firebase_uid) DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, user.Email, user.Name, firebaseUID).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = database.QueryRowContext(ctx, `
		UPDATE users SET
			name = $1,
			firebase_uid = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE email = $3
		RETURNING id
	`, user.Name, firebaseUID, user.Email).Scan(&id)
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

	completedCruiseID, err := seedCruise(ctx, tx, userID, yachtID, "Baltyk 2025", "completed")
	if err != nil {
		return err
	}
	if _, err = seedCruise(ctx, tx, userID, yachtID, "Majowka Hel 2026", "planned"); err != nil {
		return err
	}

	for index, crewID := range crewIDs {
		role := "zalogant"
		if index == 2 {
			role = "kapitan"
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO crew_assignments (cruise_id, crew_member_id, role, patent_number)
			SELECT $1, $2, $3, NULL
			WHERE NOT EXISTS (
				SELECT 1 FROM crew_assignments WHERE cruise_id = $1 AND crew_member_id = $2
			)
		`, completedCruiseID, crewID, role); err != nil {
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

func seedCruise(ctx context.Context, tx *sql.Tx, userID, yachtID int64, name, status string) (int64, error) {
	year := int64(2025)
	embark := "2025-07-04"
	disembark := "2025-07-12"
	startPort := "Gdansk"
	endPort := "Visby"
	countries := "Polska, Szwecja"
	if status == "planned" {
		year = 2026
		embark = "2026-05-23"
		disembark = "2026-05-26"
		startPort = "Hel"
		endPort = "Gdynia"
		countries = "Polska"
	}

	var cruiseID int64
	err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO cruises (
				owner_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
				hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days,
				captain_name, yacht_id, tidal_waters, cost_total, cost_per_person,
				description, max_crew, status
			)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8,
				84, 52, 18, 6, 410, 8,
				'Kasia Kapitan', $9, 1, 12400, 3100,
				'Rejs testowy z gotowa zaloga i kosztami.', 6, $10::cruise_status
			WHERE NOT EXISTS (
				SELECT 1 FROM cruises WHERE owner_id = $1 AND name = $2 AND org_id IS NULL
			)
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM cruises WHERE owner_id = $1 AND name = $2 AND org_id IS NULL
		LIMIT 1
	`, userID, name, year, embark, disembark, countries, startPort, endPort, yachtID, status).Scan(&cruiseID)
	if err != nil {
		return 0, fmt.Errorf("seed cruise %s: %w", name, err)
	}
	return cruiseID, nil
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
	var yachtID int64
	if err := tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO yachts (owner_id, org_id, name, registration_no, yacht_type)
			SELECT $1, $2, 'S/Y Klubowa', 'GDA-777', 'Delphia 40'
			WHERE NOT EXISTS (SELECT 1 FROM yachts WHERE org_id = $2 AND name = 'S/Y Klubowa')
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM yachts WHERE org_id = $2 AND name = 'S/Y Klubowa'
		LIMIT 1
	`, userID, orgID).Scan(&yachtID); err != nil {
		return fmt.Errorf("seed org yacht: %w", err)
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

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO cruises (
			owner_id, org_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
			hours_total, hours_sail, hours_engine, miles, days, captain_name, yacht_id,
			tidal_waters, cost_total, cost_per_person, description, max_crew, status
		)
		SELECT $1, $2, 'Klubowy Bornholm', 2026, '2026-08-01', '2026-08-09', 'Polska, Dania',
			'Kolobrzeg', 'Nexo', 96, 58, 24, 520, 9, 'Kasia Kapitan', $3,
			1, 18000, 3000, 'Organizacyjny rejs demo.', 8, 'planned'::cruise_status
		WHERE NOT EXISTS (SELECT 1 FROM cruises WHERE org_id = $2 AND name = 'Klubowy Bornholm')
	`, userID, orgID, yachtID); err != nil {
		return fmt.Errorf("seed org cruise: %w", err)
	}
	return nil
}
