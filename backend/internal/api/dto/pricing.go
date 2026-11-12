package dto

import (
	"encoding/json"
	"log/slog"
)

// MaxPriceListItemLen caps one entry of a price list, in characters.
const MaxPriceListItemLen = 200

// priceList decodes a JSONB string array column into a non-nil slice so the
// API always serializes it as [] rather than null.
func priceList(raw json.RawMessage) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		slog.Error("decode cruise price list", "err", err)
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}
