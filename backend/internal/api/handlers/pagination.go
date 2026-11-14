package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// pageParams carries the paging query parameters shared by every paginated
// list operation. huma derives the OpenAPI parameter definitions — including
// the defaults and bounds — from these struct tags. The fields are int32 to
// match the sqlc-generated LIMIT/OFFSET parameters.
type pageParams struct {
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32  `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
	After  string `query:"after" doc:"A previous page's next_cursor: the window starts after that page's last item, so rows deleted meanwhile shift nothing"`
}

// nameKey is the keyset cursor of a list sorted by (name, id).
type nameKey struct {
	Name string `json:"n"`
	ID   int64  `json:"i"`
}

// dateKey is the keyset cursor of a list sorted by a nullable date, then id.
type dateKey struct {
	Date *string `json:"d"`
	ID   int64   `json:"i"`
}

// voyageKey is the keyset cursor of the voyage list: (year, embark_date, id).
type voyageKey struct {
	Year *int64  `json:"y"`
	Date *string `json:"d"`
	ID   int64   `json:"i"`
}

func errInvalidCursor() error {
	return huma.Error422UnprocessableEntity("invalid after cursor")
}

// encodeCursor serialises a row's sort key into an opaque next_cursor.
func encodeCursor(key any) string {
	b, err := json.Marshal(key)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// cursorKey is a decoded keyset cursor. valid rejects one without a
// positive row id, or with a NUL in a text key, which Postgres refuses.
type cursorKey interface{ valid() bool }

func (k *nameKey) valid() bool   { return k.ID > 0 && !strings.ContainsRune(k.Name, 0) }
func (k *dateKey) valid() bool   { return k.ID > 0 && noNUL(k.Date) }
func (k *voyageKey) valid() bool { return k.ID > 0 && noNUL(k.Date) }

func noNUL(s *string) bool { return s == nil || !strings.ContainsRune(*s, 0) }

// decodeCursor parses an after cursor into key and reports whether one was
// given. A malformed or invalid cursor is a 422. The cursor has no length
// cap: it carries a whole name, and names have none.
func decodeCursor(after string, key cursorKey) (bool, error) {
	if after == "" {
		return false, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(after)
	if err != nil {
		return false, errInvalidCursor()
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(key); err != nil || dec.More() || !key.valid() {
		return false, errInvalidCursor()
	}
	return true, nil
}

// afterID is the after_id query argument of a decoded cursor: NULL without
// one, which lists from the start.
func afterID(ok bool, id int64) types.NullInt64 {
	return types.NullInt64{Int64: id, Valid: ok}
}

// afterText is a cursor's text key component as a query argument.
func afterText(ok bool, s string) types.NullString {
	return types.NullString{String: s, Valid: ok}
}

// afterNullText is a cursor's nullable text key component.
func afterNullText(s *string) types.NullString {
	if s == nil {
		return types.NullString{}
	}
	return types.NullString{String: *s, Valid: true}
}

// nullTextKey turns a row's nullable text column into a cursor component.
func nullTextKey(s types.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// fetchLimit is the row count a list query asks for: one past the window,
// so the extra row tells whether more items follow.
func fetchLimit(limit int32) int32 { return limit + 1 }

// buildPage trims the extra row fetched past limit and assembles the page,
// with a next_cursor from the last item's sort key when more items follow.
func buildPage[R, T any](
	rows []R, total int64, limit, offset int32,
	items func([]R) []T, key func(R) any,
) dto.Page[T] {
	window := make([]R, 0, min(len(rows), int(limit)))
	more := false
	for _, r := range rows {
		if len(window) == int(limit) {
			more = true
			break
		}
		window = append(window, r)
	}
	next := ""
	if more && len(window) > 0 {
		next = encodeCursor(key(window[len(window)-1]))
	}
	return dto.NewPage(items(window), total, limit, offset, more, next)
}
