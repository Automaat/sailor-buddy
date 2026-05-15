package handlers

import "github.com/danielgtaylor/huma/v2"

// apiError is the JSON error envelope for huma-served routes. It matches the
// {"error": "..."} shape emitted by the legacy chi handlers so frontend error
// handling stays uniform while the API migrates resource by resource.
type apiError struct {
	status  int
	Message string `json:"error" doc:"Human-readable error message"`
}

func (e *apiError) Error() string  { return e.Message }
func (e *apiError) GetStatus() int { return e.status }

// init overrides huma's default RFC 9457 error model with the legacy envelope.
func init() {
	huma.NewError = func(status int, msg string, _ ...error) huma.StatusError {
		return &apiError{status: status, Message: msg}
	}
}
