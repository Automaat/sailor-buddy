// Command openapi writes the OpenAPI 3.1 spec for the huma-served routes to
// backend/openapi.yaml. It is the source for the generated frontend types.
package main

import (
	"log"
	"os"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api"
)

func main() {
	spec, err := api.OpenAPIYAML()
	if err != nil {
		log.Fatalf("build openapi spec: %v", err)
	}
	const out = "openapi.yaml"
	if err := os.WriteFile(out, spec, 0o644); err != nil {
		log.Fatalf("write %s: %v", out, err)
	}
	log.Printf("wrote %s (%d bytes)", out, len(spec))
}
