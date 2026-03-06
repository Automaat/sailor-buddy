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
		raw, err := json.Marshal(data)
		if err != nil {
			return
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			_, _ = w.Write(raw)
			return
		}
		flat := flattenNulls(generic)
		_ = json.NewEncoder(w).Encode(flat)
	}
}

// flattenNulls converts sql.Null* JSON patterns like
// {"String":"x","Valid":true} → "x" and {"Valid":false,...} → null
func flattenNulls(v any) any {
	switch val := v.(type) {
	case map[string]any:
		if isNullStruct(val) {
			valid, _ := val["Valid"].(bool)
			if !valid {
				return nil
			}
			for k, v := range val {
				if k != "Valid" {
					return v
				}
			}
			return nil
		}
		for k, v := range val {
			val[k] = flattenNulls(v)
		}
		return val
	case []any:
		for i, v := range val {
			val[i] = flattenNulls(v)
		}
		return val
	default:
		return v
	}
}

// isNullStruct detects sql.Null* JSON shapes: exactly 2 keys, one being "Valid" (bool),
// the other being "String", "Int64", "Float64", or "Time".
func isNullStruct(m map[string]any) bool {
	if len(m) != 2 {
		return false
	}
	if _, ok := m["Valid"]; !ok {
		return false
	}
	for k := range m {
		switch k {
		case "Valid", "String", "Int64", "Float64", "Time":
			continue
		default:
			return false
		}
	}
	return true
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(v)
}
