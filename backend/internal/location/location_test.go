package location

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestParseName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{"plain", "Hel", "Hel", nil},
		{"surrounding blanks", " \t Hel\u00a0\n", "Hel", nil},
		{"inner blanks collapse", "Nowa \u00a0\t Wieś", "Nowa Wieś", nil},
		{"invisible blanks", "\u200bJastarnia\ufeff", "Jastarnia", nil},
		{"inner invisible becomes a space", "Zatoka\u200bPucka", "Zatoka Pucka", nil},
		{"decomposed accents stay", "Gdan\u0301sk", "Gdan\u0301sk", nil},
		{"empty", "", "", nil},
		{"blank", " \u2800\u3164 ", "", nil},
		{"longest", strings.Repeat("ż", MaxNameLen), strings.Repeat("ż", MaxNameLen), nil},
		{"longest after trimming", "  " + strings.Repeat("ż", MaxNameLen) + "  ", strings.Repeat("ż", MaxNameLen), nil},
		{"too long", strings.Repeat("ż", MaxNameLen+1), "", ErrNameTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseName(tt.in)
			if !errors.Is(err, tt.err) || got != tt.want {
				t.Fatalf("ParseName(%q) = %q, %v; want %q, %v", tt.in, got, err, tt.want, tt.err)
			}
		})
	}
}

func TestParseOptionalPort(t *testing.T) {
	long := strings.Repeat("a", MaxNameLen+1)
	tests := []struct {
		name     string
		in       string
		lat, lon *float64
		want     *Port
		err      error
	}{
		{"empty", "", nil, nil, nil, nil},
		{"blank", " \u200b\t", nil, nil, nil, nil},
		{"name only", " Visby ", nil, nil, &Port{Name: "Visby"}, nil},
		{"located", "Hel", new(54.6), new(18.8), &Port{Name: "Hel", Coordinates: &Coordinates{54.6, 18.8}}, nil},
		{"coordinates without a name", "  ", new(54.6), new(18.8), nil, ErrNameRequired},
		{"one coordinate without a name", "", new(54.6), nil, nil, ErrNameRequired},
		{"unpaired", "Hel", nil, new(18.8), nil, ErrUnpaired},
		{"latitude out of range", "Hel", new(-90.5), new(18.8), nil, ErrLatitudeInvalid},
		{"longitude out of range", "Hel", new(54.6), new(180.5), nil, ErrLongitudeInvalid},
		{"too long", long, nil, nil, nil, ErrNameTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := ParseOptionalPort(tt.in, tt.lat, tt.lon)
			if !errors.Is(err, tt.err) || ok != (tt.want != nil) {
				t.Fatalf("ParseOptionalPort(%q) = %+v, %v, %v; want %+v, %v", tt.in, got, ok, err, tt.want, tt.err)
			}
			if tt.want != nil && !reflect.DeepEqual(got, *tt.want) {
				t.Fatalf("ParseOptionalPort(%q) = %+v, want %+v", tt.in, got, *tt.want)
			}
		})
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{"two characters", "he", "he", nil},
		{"trimmed", "\u200b he \ufeff", "he", nil},
		{"inner blanks stay", "a\u200bb", "a\u200bb", nil},
		{"two decomposed letters", "a\u0301e\u0301", "a\u0301e\u0301", nil},
		{"one character", " a ", "", ErrQueryTooShort},
		{"one decomposed letter", "a\u0301", "", ErrQueryTooShort},
		{"blank", "\u3164\u3164", "", ErrQueryTooShort},
		{"longest", strings.Repeat("a", MaxNameLen), strings.Repeat("a", MaxNameLen), nil},
		{"longest once inner blanks collapse", "aa  " + strings.Repeat("a", MaxNameLen-3), "aa  " + strings.Repeat("a", MaxNameLen-3), nil},
		{"too long", strings.Repeat("ł", MaxNameLen+1), "", ErrNameTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuery(tt.in)
			if !errors.Is(err, tt.err) || got != tt.want {
				t.Fatalf("ParseQuery(%q) = %q, %v; want %q, %v", tt.in, got, err, tt.want, tt.err)
			}
		})
	}
}

func TestNewCoordinates(t *testing.T) {
	tests := []struct {
		name     string
		lat, lon float64
		err      error
	}{
		{"inside", 54.6, 18.8, nil},
		{"corners", -90, 180, nil},
		{"other corners", 90, -180, nil},
		{"latitude above", 90.0001, 0, ErrLatitudeInvalid},
		{"latitude below", -91, 0, ErrLatitudeInvalid},
		{"longitude above", 0, 180.5, ErrLongitudeInvalid},
		{"longitude below", 0, -181, ErrLongitudeInvalid},
		{"NaN latitude", math.NaN(), 0, ErrLatitudeInvalid},
		{"NaN longitude", 0, math.NaN(), ErrLongitudeInvalid},
		{"infinite latitude", math.Inf(1), 0, ErrLatitudeInvalid},
		{"infinite longitude", 0, math.Inf(-1), ErrLongitudeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCoordinates(tt.lat, tt.lon)
			if !errors.Is(err, tt.err) {
				t.Fatalf("NewCoordinates(%v, %v) error = %v, want %v", tt.lat, tt.lon, err, tt.err)
			}
			if err == nil && (got.Latitude != tt.lat || got.Longitude != tt.lon) {
				t.Fatalf("NewCoordinates(%v, %v) = %+v", tt.lat, tt.lon, got)
			}
		})
	}
}

func TestParseCoordinatePair(t *testing.T) {
	tests := []struct {
		name     string
		lat, lon *float64
		ok       bool
		err      error
	}{
		{"neither", nil, nil, false, nil},
		{"both", new(54.6), new(18.8), true, nil},
		{"latitude only", new(54.6), nil, false, ErrUnpaired},
		{"longitude only", nil, new(18.8), false, ErrUnpaired},
		{"out of range", new(91.0), new(18.8), false, ErrLatitudeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok, err := ParseCoordinatePair(tt.lat, tt.lon)
			if ok != tt.ok || !errors.Is(err, tt.err) {
				t.Fatalf("ParseCoordinatePair = %+v, %v, %v; want ok %v, %v", c, ok, err, tt.ok, tt.err)
			}
			if ok && (c.Latitude != *tt.lat || c.Longitude != *tt.lon) {
				t.Fatalf("ParseCoordinatePair = %+v", c)
			}
		})
	}
}

func TestParseCoordinates(t *testing.T) {
	tests := []struct {
		name     string
		lat, lon string
		err      error
	}{
		{"decimal", "54.6083", "18.8011", nil},
		{"text latitude", "north", "18", ErrLatitudeInvalid},
		{"empty longitude", "54", "", ErrLongitudeInvalid},
		{"NaN", "NaN", "18", ErrLatitudeInvalid},
		{"out of range", "54", "181", ErrLongitudeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseCoordinates(tt.lat, tt.lon); !errors.Is(err, tt.err) {
				t.Fatalf("ParseCoordinates(%q, %q) error = %v, want %v", tt.lat, tt.lon, err, tt.err)
			}
		})
	}
}

func TestNewPort(t *testing.T) {
	t.Run("named with coordinates", func(t *testing.T) {
		p, err := NewPort("  Hel ", new(54.6), new(18.8))
		if err != nil || p.Name != "Hel" || p.Coordinates == nil || *p.Coordinates != (Coordinates{54.6, 18.8}) {
			t.Fatalf("NewPort = %+v, %v", p, err)
		}
		lat, lon := p.LatLon()
		if lat == nil || lon == nil || *lat != 54.6 || *lon != 18.8 {
			t.Fatalf("LatLon = %v, %v", lat, lon)
		}
	})
	t.Run("name only", func(t *testing.T) {
		p, err := NewPort("Zatoka Pucka", nil, nil)
		if err != nil || p.Coordinates != nil {
			t.Fatalf("NewPort = %+v, %v", p, err)
		}
		if lat, lon := p.LatLon(); lat != nil || lon != nil {
			t.Fatalf("LatLon = %v, %v, want nil", lat, lon)
		}
	})
	errs := []struct {
		name     string
		in       string
		lat, lon *float64
		err      error
		field    Field
	}{
		{"blank name", " \u200b ", nil, nil, ErrNameRequired, FieldName},
		{"too long name", strings.Repeat("a", MaxNameLen+1), nil, nil, ErrNameTooLong, FieldName},
		{"unpaired", "Hel", nil, new(18.8), ErrUnpaired, FieldLatitude},
		{"longitude out of range", "Hel", new(54.6), new(-180.1), ErrLongitudeInvalid, FieldLongitude},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPort(tt.in, tt.lat, tt.lon)
			if !errors.Is(err, tt.err) {
				t.Fatalf("NewPort error = %v, want %v", err, tt.err)
			}
			le, ok := errors.AsType[*Error](err)
			if !ok || le.Field != tt.field {
				t.Fatalf("NewPort error field = %+v, want %q", le, tt.field)
			}
		})
	}
}

func TestBaseCharCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"Hel", 3},
		{" \u200bHel\ufeff ", 3},
		{"Gdan\u0301sk", 6},
		{"Gdańsk", 6},
		{"\u0301\u0301", 0},
		{"\u3164\u2800", 0},
	}
	for _, tt := range tests {
		if got := BaseCharCount(tt.in); got != tt.want {
			t.Errorf("BaseCharCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFold(t *testing.T) {
	tests := map[string]string{
		"Świnoujście":        "swinoujscie",
		"  ŁEBA ":            "leba",
		"Svolvær":            "svolvaer",
		"København":          "kobenhavn",
		"Gdan\u0301sk":       "gdansk",
		"Nowa\u00a0  Wieś":   "nowa wies",
		"Hel\u200b":          "hel",
		"Λευκάδα":            "λευκαδα",
		"Szczecin, Polska":   "szczecin, polska",
		"Straße am Hafen":    "strasse am hafen",
		"Đurđevac":           "durdevac",
		"Œuilly":             "oeuilly",
		"Þórshöfn":           "thorshofn",
		"Ísafjörður":         "isafjordur",
		"Kızkalesi":          "kizkalesi",
		"Kraków  i  okolice": "krakow i okolice",
	}
	for in, want := range tests {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}
