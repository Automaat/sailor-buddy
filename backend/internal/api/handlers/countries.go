package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/countries"
)

type countriesOutput struct {
	Body []dto.Country
}

// RegisterCountryRoutes serves the country list the forms pick from and the
// pages name countries with.
func RegisterCountryRoutes(api huma.API) {
	list := dto.CountriesFrom(countries.All())
	huma.Register(api, huma.Operation{
		OperationID: "list-countries", Method: http.MethodGet, Path: "/countries",
		Summary:     "List the countries cruises, trips and voyages may name",
		Description: "Every ISO 3166-1 alpha-2 code the API accepts in `countries`, with its Polish name, ordered by code.",
		Tags:        []string{"Countries"},
	}, func(context.Context, *struct{}) (*countriesOutput, error) {
		return &countriesOutput{Body: list}, nil
	})
}
