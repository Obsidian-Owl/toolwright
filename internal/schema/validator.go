package schema

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"github.com/kaptinlin/jsonschema"
)

// Validator validates JSON data against JSON Schemas loaded from a filesystem.
type Validator struct {
	fs fs.FS
}

// NewValidator creates a validator that loads schemas from the given FS.
// In production this is typically an embed.FS; in tests a fstest.MapFS.
func NewValidator(schemaFS fs.FS) *Validator {
	return &Validator{fs: schemaFS}
}

// Validate checks data against the schema at schemaPath within the FS.
func (v *Validator) Validate(schemaPath string, data []byte) error {
	if schemaPath == "" {
		return fmt.Errorf("schema path must not be empty")
	}

	schemaBytes, err := fs.ReadFile(v.fs, schemaPath)
	if err != nil {
		return fmt.Errorf("schema not found: %s: %w", schemaPath, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("data must not be empty")
	}

	compiler := jsonschema.NewCompiler()
	compiled, err := compiler.Compile(schemaBytes)
	if err != nil {
		return fmt.Errorf("failed to compile schema %s: %w", schemaPath, err)
	}

	result := compiled.ValidateJSON(data)
	if result.IsValid() {
		return nil
	}

	detailedErrors := result.GetDetailedErrors()
	if len(detailedErrors) == 0 {
		return fmt.Errorf("validation failed against schema %s", schemaPath)
	}

	parts := make([]string, 0, len(detailedErrors))
	for path, msg := range detailedErrors {
		parts = append(parts, fmt.Sprintf("%s: %s", path, msg))
	}

	return fmt.Errorf("validation failed: %s", strings.Join(parts, "; "))
}
