package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"
)

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if rv := reflect.ValueOf(data); rv.Kind() == reflect.Slice && rv.IsNil() {
			_, _ = w.Write([]byte("[]\n"))
			return
		}
		_ = json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(v)
}
