// Package jsonschema provides JSON Schema generation for skill-loop configuration files.
package jsonschema

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/yoshikawatakumi/skill-loop/internal/config"
)

// Generate creates a JSON Schema from the Config type for editor autocomplete and validation.
func Generate() ([]byte, error) {
	r := &jsonschema.Reflector{
		// Use yaml struct tags for property names instead of Go field names.
		FieldNameTag: "yaml",
	}

	s := r.Reflect(&config.Config{})

	s.ID = "https://raw.githubusercontent.com/yoshikawatakumi/skill-loop/main/schema.json"
	s.Title = "skill-loop"
	s.Description = "Schema for skill-loop YAML configuration files (skill-loop.yml)"

	return json.MarshalIndent(s, "", "  ")
}
