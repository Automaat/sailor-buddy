package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// BootstrapAdmin provisions the explicitly selected, already registered UID.
// It never transfers an account or adds a second administrator.
func BootstrapAdmin(ctx context.Context, database *sql.DB, uid string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(uid) != uid {
		return errors.New("an exact Firebase UID is required")
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := sqlcdb.New(tx)
	if err := q.LockAdminRoles(ctx); err != nil {
		return fmt.Errorf("lock admin roles: %w", err)
	}
	admins, err := q.LockAdmins(ctx)
	if err != nil {
		return fmt.Errorf("read admins: %w", err)
	}
	user, err := q.GetUserByFirebaseUID(ctx, types.NullString{String: uid, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("selected Firebase UID has no registered application account; sign in once first")
	}
	if err != nil {
		return fmt.Errorf("read selected account: %w", err)
	}
	if user.Role == "admin" {
		return tx.Commit()
	}
	if len(admins) > 0 {
		return errors.New("an administrator already exists; use normal role management")
	}
	if err := q.UpdateUserRole(ctx, sqlcdb.UpdateUserRoleParams{ID: user.ID, Role: "admin"}); err != nil {
		return fmt.Errorf("set initial admin: %w", err)
	}
	return tx.Commit()
}
