package dto

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/countries"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// Country is one country the app accepts.
type Country struct {
	Code string `json:"code" doc:"ISO 3166-1 alpha-2 code" example:"NO"`
	Name string `json:"name" doc:"Polish name" example:"Norwegia"`
}

// CountriesFrom maps the country list to the API model.
func CountriesFrom(list []countries.Country) []Country {
	out := make([]Country, len(list))
	for i, c := range list {
		out[i] = Country{Code: c.Code, Name: c.Name}
	}
	return out
}

// CountryCodes is a request's list of ISO 3166-1 alpha-2 country codes.
// Resolve rejects a code that is not on the list (GET /countries).
type CountryCodes []string

// Resolve reports each unknown code at its index.
func (c CountryCodes) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	var errs []error
	for i, code := range c {
		if countries.Known(code) {
			continue
		}
		errs = append(errs, &huma.ErrorDetail{
			Message:  "unknown country code; use an ISO 3166-1 alpha-2 code from GET /countries",
			Location: prefix.WithIndex(i),
			Value:    code,
		})
	}
	return errs
}

// Stored is the list for the countries column; nil stores as '{}'.
func (c CountryCodes) Stored() types.TextArray {
	return types.TextArray(c)
}

// countryCodes is a stored list for a response: never nil, so it
// serializes to [] rather than null.
func countryCodes(a types.TextArray) []string {
	if a == nil {
		return []string{}
	}
	return []string(a)
}
