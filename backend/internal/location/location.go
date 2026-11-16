// Package location holds the rules for port names, port search queries and
// coordinates, shared by the geocoding proxy, port suggestions and every
// write of a port.
package location

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// MaxNameLen is the most characters a port name or a port search query
// holds, counted in runes after NormalizeName.
const MaxNameLen = 100

// MinQueryLen is the fewest base characters, as BaseCharCount counts them,
// that a search query holds.
const MinQueryLen = 2

// Field names the input an Error is about.
type Field string

const (
	FieldName      Field = "name"
	FieldLatitude  Field = "latitude"
	FieldLongitude Field = "longitude"
)

// Error is a rejected port name, query or coordinate.
type Error struct {
	Field   Field
	Message string
}

func (e *Error) Error() string { return e.Message }

var (
	ErrNameRequired     = &Error{FieldName, "port name must not be blank"}
	ErrNameTooLong      = &Error{FieldName, fmt.Sprintf("expected at most %d characters", MaxNameLen)}
	ErrQueryTooShort    = &Error{FieldName, fmt.Sprintf("expected at least %d visible characters", MinQueryLen)}
	ErrUnpaired         = &Error{FieldLatitude, "latitude and longitude must be given together"}
	ErrLatitudeInvalid  = &Error{FieldLatitude, "latitude must be a number from -90 to 90"}
	ErrLongitudeInvalid = &Error{FieldLongitude, "longitude must be a number from -180 to 180"}
)

// Coordinates is a position in degrees: latitude within ±90, longitude
// within ±180, neither NaN.
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// Port is a port name with an optional position. A port without
// coordinates is known by name only and is not drawn on a map.
type Port struct {
	Name        string
	Coordinates *Coordinates
}

// NewPort applies the name and coordinate rules to a port that must have a
// name.
func NewPort(name string, lat, lon *float64) (Port, error) {
	n, err := ParseName(name)
	if err != nil {
		return Port{}, err
	}
	if n == "" {
		return Port{}, ErrNameRequired
	}
	c, located, err := ParseCoordinatePair(lat, lon)
	if err != nil {
		return Port{}, err
	}
	p := Port{Name: n}
	if located {
		p.Coordinates = &c
	}
	return p, nil
}

// LatLon returns the port's coordinates as optional values, both nil for a
// port known by name only.
func (p Port) LatLon() (lat, lon *float64) {
	if p.Coordinates == nil {
		return nil, nil
	}
	return &p.Coordinates.Latitude, &p.Coordinates.Longitude
}

// IsBlank reports whether r carries no searchable text:
//   - whitespace;
//   - format (Cf, such as zero-width spaces and the byte order mark) and
//     control (Cc, such as NUL) characters;
//   - the other default-ignorable code points: Hangul fillers (U+115F,
//     U+1160, U+3164, U+FFA0), the combining grapheme joiner (U+034F) and the
//     variation selectors (U+FE00..U+FE0F and others);
//   - the braille blank (U+2800), which renders as an empty cell;
//   - bytes that are not valid UTF-8.
//
// It matches explicit code points, not whole categories: the combining
// marks of decomposed (NFD) letters, such as the acute in "ź", stay.
func IsBlank(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cc, r) ||
		unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r) ||
		unicode.Is(unicode.Variation_Selector, r) ||
		r == '\u2800' || r == utf8.RuneError
}

// TrimBlanks strips leading and trailing runes that IsBlank matches. Blank
// runes between other characters stay. The result may still start with a
// combining mark or hold only marks, so a length check counts with
// BaseCharCount instead of the rune count.
func TrimBlanks(s string) string {
	return strings.TrimFunc(s, IsBlank)
}

// NormalizeName is the port name rule: no blanks around the name and one
// space for each run of blanks inside it. An empty result means no port.
func NormalizeName(s string) string {
	return strings.Join(strings.FieldsFunc(s, IsBlank), " ")
}

// ParseName normalizes a port name and rejects one longer than MaxNameLen.
// An empty result means no port.
func ParseName(s string) (string, error) {
	n := NormalizeName(s)
	if utf8.RuneCountInString(n) > MaxNameLen {
		return "", ErrNameTooLong
	}
	return n, nil
}

// ParseOptionalName applies ParseName to an optional port name, such as a
// leg's start or end port. An absent name gives "", which means no port.
func ParseOptionalName(s *string) (string, error) {
	if s == nil {
		return "", nil
	}
	return ParseName(*s)
}

// ParseQuery checks a typed search for a port or town: it must hold at
// least MinQueryLen base characters and, normalized as a name, at most
// MaxNameLen runes. It returns the query with its surrounding blanks
// trimmed; inner blanks stay as typed.
func ParseQuery(s string) (string, error) {
	q := TrimBlanks(s)
	if BaseCharCount(q) < MinQueryLen {
		return "", ErrQueryTooShort
	}
	if utf8.RuneCountInString(NormalizeName(q)) > MaxNameLen {
		return "", ErrNameTooLong
	}
	return q, nil
}

// BaseCharCount counts the runes in s that are neither blank nor combining
// marks (M). A letter and its decomposed accents count once, so NFC and NFD
// spellings of a name have the same count.
func BaseCharCount(s string) int {
	n := 0
	for _, r := range s {
		if !IsBlank(r) && !unicode.Is(unicode.M, r) {
			n++
		}
	}
	return n
}

// placeFolds spells out letters that Unicode does not decompose into a base
// letter and a mark, so "Łeba" folds like "Leba" and "Svolvær" like
// "Svolvaer", as PostgreSQL's unaccent does.
var placeFolds = strings.NewReplacer(
	"ł", "l", "đ", "d", "ø", "o", "æ", "ae", "œ", "oe", "ß", "ss", "þ", "th", "ð", "d", "ı", "i",
)

// Fold lowercases a place name, strips its accents and normalizes its
// blanks, so spellings that differ only in case or diacritics compare equal.
func Fold(s string) string {
	s = placeFolds.Replace(strings.ToLower(s))
	stripped, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	if err == nil {
		s = stripped
	}
	return NormalizeName(s)
}

// NewCoordinates checks a position. NaN, which the JSON encoder cannot
// write, fails as does ±Inf, which is out of range.
func NewCoordinates(lat, lon float64) (Coordinates, error) {
	if math.IsNaN(lat) || math.Abs(lat) > 90 {
		return Coordinates{}, ErrLatitudeInvalid
	}
	if math.IsNaN(lon) || math.Abs(lon) > 180 {
		return Coordinates{}, ErrLongitudeInvalid
	}
	return Coordinates{Latitude: lat, Longitude: lon}, nil
}

// ParseCoordinatePair checks optional coordinates, which come both or
// neither. ok reports whether they came.
func ParseCoordinatePair(lat, lon *float64) (c Coordinates, ok bool, err error) {
	if lat == nil && lon == nil {
		return Coordinates{}, false, nil
	}
	if lat == nil || lon == nil {
		return Coordinates{}, false, ErrUnpaired
	}
	c, err = NewCoordinates(*lat, *lon)
	if err != nil {
		return Coordinates{}, false, err
	}
	return c, true, nil
}

// ParseCoordinates parses a position given as decimal text, as geocoding
// services send it.
func ParseCoordinates(rawLat, rawLon string) (Coordinates, error) {
	lat, latErr := strconv.ParseFloat(rawLat, 64)
	if latErr != nil {
		return Coordinates{}, ErrLatitudeInvalid
	}
	lon, lonErr := strconv.ParseFloat(rawLon, 64)
	if lonErr != nil {
		return Coordinates{}, ErrLongitudeInvalid
	}
	return NewCoordinates(lat, lon)
}
