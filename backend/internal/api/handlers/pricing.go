package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
)

// defaultCurrency is the currency of a cruise or trip whose body omits it.
const defaultCurrency = "PLN"

// currencyOrDefault returns the body currency, or PLN when the body omits it.
func currencyOrDefault(c string) string {
	if c == "" {
		return defaultCurrency
	}
	return c
}

// encodePriceList trims each entry of an "included" / "not included" list,
// drops blank entries and encodes the rest as a JSON array for the JSONB
// column. An entry longer than dto.MaxPriceListItemLen characters is a 422.
func encodePriceList(field string, items []string) (json.RawMessage, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if utf8.RuneCountInString(item) > dto.MaxPriceListItemLen {
			return nil, huma.Error422UnprocessableEntity(
				fmt.Sprintf("%s entries must be at most %d characters", field, dto.MaxPriceListItemLen))
		}
		out = append(out, item)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode " + field)
	}
	return b, nil
}

// cruisePriceLists encodes both price lists of a cruise body.
func cruisePriceLists(body *dto.CruiseBody) (included, notIncluded json.RawMessage, err error) {
	if included, err = encodePriceList("included", body.Included); err != nil {
		return nil, nil, err
	}
	if notIncluded, err = encodePriceList("not_included", body.NotIncluded); err != nil {
		return nil, nil, err
	}
	return included, notIncluded, nil
}
