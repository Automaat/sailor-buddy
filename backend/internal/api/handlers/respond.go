package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"
)

// respondJSON writes data as a JSON response. Nullable columns use the
// types.Null* wrappers, which marshal to a plain value or null, so no
// post-processing of the encoded payload is needed.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	if rv := reflect.ValueOf(data); rv.Kind() == reflect.Slice && rv.IsNil() {
		_, _ = w.Write([]byte("[]\n"))
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(v)
}
