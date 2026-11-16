// Package countries is the one list of countries the app accepts: ISO
// 3166-1 alpha-2 codes with their Polish names. Cruises, trips and voyages
// store codes; the API validates them here, serves the list to the
// frontend (GET /countries) and docgen prints the names from it.
//
// countries.json was generated once from the CLDR region names that ship
// with ICU (Intl.DisplayNames, Node 26, CLDR 48), keeping the 249 officially
// assigned ISO 3166-1 codes. Edit it by hand only to rename a country.
package countries

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Country is one country: its ISO 3166-1 alpha-2 code and its Polish name.
type Country struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

//go:embed countries.json
var data []byte

var (
	all    = mustParse(data)
	byCode = index(all)
)

var codePattern = regexp.MustCompile(`^[A-Z]{2}$`)

func mustParse(raw []byte) []Country {
	list, err := parse(raw)
	if err != nil {
		panic(err)
	}
	return list
}

// parse decodes the list and rejects an empty list, a malformed code, a
// blank name or a repeated code, so a bad edit fails every test instead of
// a request.
func parse(raw []byte) ([]Country, error) {
	var list []Country
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("countries.json: %w", err)
	}
	if len(list) == 0 {
		return nil, errors.New("countries.json: the list is empty")
	}
	seen := make(map[string]bool, len(list))
	for _, c := range list {
		if !codePattern.MatchString(c.Code) {
			return nil, fmt.Errorf("countries.json: code %q is not two upper-case letters", c.Code)
		}
		if strings.TrimSpace(c.Name) == "" {
			return nil, fmt.Errorf("countries.json: %s has no name", c.Code)
		}
		if seen[c.Code] {
			return nil, fmt.Errorf("countries.json: %s is listed twice", c.Code)
		}
		seen[c.Code] = true
	}
	return list, nil
}

func index(list []Country) map[string]string {
	m := make(map[string]string, len(list))
	for _, c := range list {
		m[c.Code] = c.Name
	}
	return m
}

// All returns every country, ordered by code. The slice is a copy.
func All() []Country {
	out := make([]Country, len(all))
	copy(out, all)
	return out
}

// Known reports whether code is on the list.
func Known(code string) bool {
	_, ok := byCode[code]
	return ok
}

// Name is the Polish name of code, or code itself when it is not on the
// list, so a stored code never prints blank.
func Name(code string) string {
	if name, ok := byCode[code]; ok {
		return name
	}
	return code
}

// Format joins the Polish names of codes with ", ", in the given order.
func Format(codes []string) string {
	names := make([]string, len(codes))
	for i, code := range codes {
		names[i] = Name(code)
	}
	return strings.Join(names, ", ")
}
