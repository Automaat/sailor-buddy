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
// Each body that carries one checks it in its own Resolve with
// countryCodeErrors: a resolver on this type would not run inside a body
// that embeds a struct with its own Resolve, such as TripFields.
type CountryCodes []string

// countryCodeErrors reports each code in codes that is not on the list
// (GET /countries), at its index under prefix's field.
func countryCodeErrors(prefix *huma.PathBuffer, field string, codes CountryCodes) []error {
	var errs []error
	for i, code := range codes {
		if countries.Known(code) {
			continue
		}
		prefix.Push(field)
		errs = append(errs, &huma.ErrorDetail{
			Message:  "unknown country code; use an ISO 3166-1 alpha-2 code from GET /countries",
			Location: prefix.WithIndex(i),
			Value:    code,
		})
		prefix.Pop()
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
