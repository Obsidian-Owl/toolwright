package manifest

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaptinlin/jsonschema"
)

// Severity constants for ValidationError.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// ValidationError represents a single validation issue.
type ValidationError struct {
	Path     string // e.g., "tools[0].args[1].name"
	Message  string // Human-readable description
	Rule     string // Machine identifier: "name-format", "semver", etc.
	Severity string // "error" (default) or "warning"
}

var (
	nameRe   = regexp.MustCompile(`^[a-z0-9-]+$`)
	semverRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`)
)

// validFlagTypes is the complete set of recognised flag types.
var validFlagTypes = map[string]bool{
	"string":   true,
	"int":      true,
	"float":    true,
	"bool":     true,
	"object":   true,
	"string[]": true,
	"int[]":    true,
	"float[]":  true,
	"bool[]":   true,
	"object[]": true,
}

// Validate checks a parsed Toolkit for structural and semantic errors.
// Returns all found errors (not fail-fast).
func Validate(t *Toolkit) []ValidationError {
	if t == nil {
		return []ValidationError{}
	}

	var errs []ValidationError

	errs = append(errs, validateMetadata(t.Metadata)...)
	errs = append(errs, validateTools(t.Tools)...)

	if t.Auth != nil {
		errs = append(errs, validateAuth(*t.Auth, "auth")...)
	}

	return errs
}

func validateMetadata(m Metadata) []ValidationError {
	var errs []ValidationError

	if m.Name == "" || !nameRe.MatchString(m.Name) {
		errs = append(errs, ValidationError{
			Path:     "metadata.name",
			Message:  fmt.Sprintf("name %q must match ^[a-z0-9-]+$", m.Name),
			Rule:     "name-format",
			Severity: SeverityError,
		})
	}

	if !semverRe.MatchString(m.Version) {
		errs = append(errs, ValidationError{
			Path:     "metadata.version",
			Message:  fmt.Sprintf("version %q is not valid SemVer", m.Version),
			Rule:     "semver",
			Severity: SeverityError,
		})
	}

	if m.Description == "" {
		errs = append(errs, ValidationError{
			Path:     "metadata.description",
			Message:  "description is required",
			Rule:     "description-required",
			Severity: SeverityError,
		})
	} else if len(m.Description) > 200 {
		errs = append(errs, ValidationError{
			Path:     "metadata.description",
			Message:  fmt.Sprintf("description must be 200 characters or fewer, got %d", len(m.Description)),
			Rule:     "description-length",
			Severity: SeverityError,
		})
	}

	return errs
}

func validateTools(tools []Tool) []ValidationError {
	var errs []ValidationError

	// Check for duplicate tool names.
	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool.Name] {
			errs = append(errs, ValidationError{
				Path:     "tools",
				Message:  fmt.Sprintf("tool name %q is not unique", tool.Name),
				Rule:     "unique-tool-name",
				Severity: SeverityError,
			})
		}
		seen[tool.Name] = true
	}

	// Validate each tool.
	for i, tool := range tools {
		prefix := fmt.Sprintf("tools[%d]", i)
		errs = append(errs, validateTool(tool, prefix)...)
	}

	return errs
}

func validateTool(tool Tool, prefix string) []ValidationError {
	var errs []ValidationError

	// Duplicate arg names.
	seenArgs := make(map[string]bool)
	for _, arg := range tool.Args {
		if seenArgs[arg.Name] {
			errs = append(errs, ValidationError{
				Path:     prefix + ".args",
				Message:  fmt.Sprintf("arg name %q is not unique within tool", arg.Name),
				Rule:     "unique-arg-name",
				Severity: SeverityError,
			})
		}
		seenArgs[arg.Name] = true
	}

	// Duplicate flag names.
	seenFlags := make(map[string]bool)
	for _, flag := range tool.Flags {
		if seenFlags[flag.Name] {
			errs = append(errs, ValidationError{
				Path:     prefix + ".flags",
				Message:  fmt.Sprintf("flag name %q is not unique within tool", flag.Name),
				Rule:     "unique-flag-name",
				Severity: SeverityError,
			})
		}
		seenFlags[flag.Name] = true
	}

	// Flag type checks.
	for j, flag := range tool.Flags {
		flagPrefix := fmt.Sprintf("%s.flags[%d]", prefix, j)
		errs = append(errs, validateFlag(flag, flagPrefix)...)
	}

	// Tool-level auth.
	if tool.Auth != nil {
		errs = append(errs, validateAuth(*tool.Auth, prefix+".auth")...)
	}

	return errs
}

func validateFlag(flag Flag, prefix string) []ValidationError {
	var errs []ValidationError

	// Reject unknown flag types first.
	if !validFlagTypes[flag.Type] {
		errs = append(errs, ValidationError{
			Path:     prefix + ".type",
			Message:  fmt.Sprintf("unknown flag type %q (must be one of: string, int, float, bool, object, string[], int[], float[], bool[], object[])", flag.Type),
			Rule:     "unknown-flag-type",
			Severity: SeverityError,
		})
		return errs
	}

	// Check default value type.
	if flag.Default != nil {
		if err := checkDefaultType(flag.Type, flag.Default); err != nil {
			errs = append(errs, ValidationError{
				Path:     prefix + ".default",
				Message:  err.Error(),
				Rule:     "type-mismatch",
				Severity: SeverityError,
			})
		} else if flag.ItemSchema != nil && (flag.Type == "object" || BaseType(flag.Type) == "object") {
			// For object types with itemSchema, validate the default against the schema.
			if schemaErrs := validateDefaultAgainstItemSchema(flag, prefix); len(schemaErrs) > 0 {
				errs = append(errs, schemaErrs...)
			}
		}
	}

	// Check enum values match declared type.
	if len(flag.Enum) > 0 {
		if err := checkEnumType(flag.Type, flag.Enum); err != nil {
			errs = append(errs, ValidationError{
				Path:     prefix + ".enum",
				Message:  err.Error(),
				Rule:     "type-mismatch",
				Severity: SeverityError,
			})
		}
	}

	// itemSchema validation
	errs = append(errs, validateItemSchema(flag, prefix)...)

	return errs
}

func checkDefaultType(flagType string, value any) error {
	if IsArrayType(flagType) {
		elems, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("default value %v (%T) does not match type %q: expected array", value, value, flagType)
		}
		base := BaseType(flagType)
		for i, elem := range elems {
			if err := checkDefaultType(base, elem); err != nil {
				return fmt.Errorf("default value: element [%d] %w", i, err)
			}
		}
		return nil
	}

	switch flagType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("default value %v (%T) does not match type %q", value, value, flagType)
		}
	case "int":
		switch value.(type) {
		case int, int8, int16, int32, int64:
			// ok
		default:
			return fmt.Errorf("default value %v (%T) does not match type %q", value, value, flagType)
		}
	case "float":
		switch value.(type) {
		case float32, float64, int, int8, int16, int32, int64:
			// int is acceptable for float
		default:
			return fmt.Errorf("default value %v (%T) does not match type %q", value, value, flagType)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("default value %v (%T) does not match type %q", value, value, flagType)
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("default value %v (%T) does not match type %q: expected object", value, value, flagType)
		}
	}
	return nil
}

func checkEnumType(flagType string, enum []string) error {
	if IsArrayType(flagType) {
		return checkEnumType(BaseType(flagType), enum)
	}

	switch flagType {
	case "bool":
		for _, v := range enum {
			if v != "true" && v != "false" {
				return fmt.Errorf("enum value %q is not a valid bool (must be \"true\" or \"false\")", v)
			}
		}
	case "int":
		for _, v := range enum {
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("enum value %q is not a valid int", v)
			}
		}
	case "float":
		for _, v := range enum {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return fmt.Errorf("enum value %q is not a valid float", v)
			}
		}
	case "string":
		// All string values are valid.
	}
	return nil
}

// validJSONSchemaTypes is the set of valid JSON Schema type values per draft 2020-12.
var validJSONSchemaTypes = map[string]bool{
	"null": true, "boolean": true, "object": true,
	"array": true, "number": true, "string": true, "integer": true,
}

// checkItemSchemaTypeValues recursively validates that all "type" values in a
// JSON Schema map use valid JSON Schema types (per draft 2020-12). Returns an
// error describing the first invalid type found.
func checkItemSchemaTypeValues(schema map[string]any) error {
	if t, ok := schema["type"]; ok {
		switch tv := t.(type) {
		case string:
			if !validJSONSchemaTypes[tv] {
				return fmt.Errorf("invalid JSON Schema type %q (must be one of: null, boolean, object, array, number, string, integer)", tv)
			}
		case []any:
			for _, item := range tv {
				if s, ok := item.(string); ok && !validJSONSchemaTypes[s] {
					return fmt.Errorf("invalid JSON Schema type %q (must be one of: null, boolean, object, array, number, string, integer)", s)
				}
			}
		}
	}
	// Recurse into sub-schema keywords.
	for _, key := range []string{"properties", "patternProperties"} {
		if sub, ok := schema[key]; ok {
			if subMap, ok := sub.(map[string]any); ok {
				for _, v := range subMap {
					if subSchema, ok := v.(map[string]any); ok {
						if err := checkItemSchemaTypeValues(subSchema); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	for _, key := range []string{"items", "additionalProperties", "not", "if", "then", "else",
		"unevaluatedItems", "unevaluatedProperties", "propertyNames", "contains"} {
		if sub, ok := schema[key]; ok {
			if subSchema, ok := sub.(map[string]any); ok {
				if err := checkItemSchemaTypeValues(subSchema); err != nil {
					return err
				}
			}
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if sub, ok := schema[key]; ok {
			if subArr, ok := sub.([]any); ok {
				for _, item := range subArr {
					if subSchema, ok := item.(map[string]any); ok {
						if err := checkItemSchemaTypeValues(subSchema); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// validateItemSchema checks itemSchema constraints for a flag.
// - Non-object types must not have itemSchema.
// - Object types without itemSchema produce a warning.
// - Object types with itemSchema must be a valid JSON Schema.
func validateItemSchema(flag Flag, prefix string) []ValidationError {
	isObjectType := flag.Type == "object" || flag.Type == "object[]"

	if !isObjectType {
		if flag.ItemSchema != nil {
			return []ValidationError{{
				Path:     prefix + ".itemSchema",
				Message:  fmt.Sprintf("itemSchema is not allowed on flag type %q", flag.Type),
				Rule:     "item-schema-not-allowed",
				Severity: SeverityError,
			}}
		}
		return nil
	}

	// Object type: itemSchema is optional but encouraged.
	if flag.ItemSchema == nil {
		return []ValidationError{{
			Path:     prefix + ".itemSchema",
			Message:  fmt.Sprintf("object flag %q has no itemSchema; consider adding one to describe the expected structure", flag.Name),
			Rule:     "missing-item-schema",
			Severity: SeverityWarning,
		}}
	}

	// Validate type values in the schema recursively.
	if err := checkItemSchemaTypeValues(flag.ItemSchema); err != nil {
		return []ValidationError{{
			Path:     prefix + ".itemSchema",
			Message:  fmt.Sprintf("itemSchema is not a valid JSON Schema: %v", err),
			Rule:     "invalid-item-schema",
			Severity: SeverityError,
		}}
	}

	// Also attempt compilation to catch other structural errors.
	schemaBytes, err := json.Marshal(flag.ItemSchema)
	if err != nil {
		return []ValidationError{{
			Path:     prefix + ".itemSchema",
			Message:  fmt.Sprintf("itemSchema could not be marshaled: %v", err),
			Rule:     "invalid-item-schema",
			Severity: SeverityError,
		}}
	}

	compiler := jsonschema.NewCompiler()
	if _, err := compiler.Compile(schemaBytes); err != nil {
		return []ValidationError{{
			Path:     prefix + ".itemSchema",
			Message:  fmt.Sprintf("itemSchema is not a valid JSON Schema: %v", err),
			Rule:     "invalid-item-schema",
			Severity: SeverityError,
		}}
	}

	return nil
}

// validateDefaultAgainstItemSchema validates an object flag's default value
// against its itemSchema. Called only when flag has a non-nil default and
// a non-nil itemSchema and the base type is "object".
func validateDefaultAgainstItemSchema(flag Flag, prefix string) []ValidationError {
	schemaBytes, err := json.Marshal(flag.ItemSchema)
	if err != nil {
		return nil // schema marshal error caught by validateItemSchema
	}

	compiler := jsonschema.NewCompiler()
	compiled, err := compiler.Compile(schemaBytes)
	if err != nil {
		return nil // schema compile error caught by validateItemSchema
	}

	// For object[], validate each element against the schema.
	if flag.Type == "object[]" {
		elems, ok := flag.Default.([]interface{})
		if !ok {
			return nil // type mismatch caught by checkDefaultType
		}
		for i, elem := range elems {
			elemBytes, marshalErr := json.Marshal(elem)
			if marshalErr != nil {
				continue
			}
			result := compiled.ValidateJSON(elemBytes)
			if !result.IsValid() {
				detailedErrors := result.GetDetailedErrors()
				parts := make([]string, 0, len(detailedErrors))
				for path, msg := range detailedErrors {
					parts = append(parts, fmt.Sprintf("[%d].%s: %s", i, path, msg))
				}
				return []ValidationError{{
					Path:     prefix + ".default",
					Message:  fmt.Sprintf("default value violates itemSchema: %s", strings.Join(parts, "; ")),
					Rule:     "type-mismatch",
					Severity: SeverityError,
				}}
			}
		}
		return nil
	}

	// For object, validate the default map directly.
	defaultBytes, err := json.Marshal(flag.Default)
	if err != nil {
		return nil
	}

	result := compiled.ValidateJSON(defaultBytes)
	if result.IsValid() {
		return nil
	}

	detailedErrors := result.GetDetailedErrors()
	parts := make([]string, 0, len(detailedErrors))
	for path, msg := range detailedErrors {
		parts = append(parts, fmt.Sprintf("%s: %s", path, msg))
	}

	return []ValidationError{{
		Path:     prefix + ".default",
		Message:  fmt.Sprintf("default value violates itemSchema: %s", strings.Join(parts, "; ")),
		Rule:     "type-mismatch",
		Severity: SeverityError,
	}}
}

func validateAuth(auth Auth, prefix string) []ValidationError {
	var errs []ValidationError

	switch auth.Type {
	case "none":
		// no additional validation

	case "token":
		if auth.TokenEnv == "" {
			errs = append(errs, ValidationError{
				Path:     prefix + ".token_env",
				Message:  "token auth requires token_env",
				Rule:     "auth-token-env",
				Severity: SeverityError,
			})
		}
		if auth.TokenFlag == "" {
			errs = append(errs, ValidationError{
				Path:     prefix + ".token_flag",
				Message:  "token auth requires token_flag",
				Rule:     "auth-token-flag",
				Severity: SeverityError,
			})
		}

	case "oauth2":
		if auth.ProviderURL == "" {
			errs = append(errs, ValidationError{
				Path:     prefix + ".provider_url",
				Message:  "oauth2 auth requires provider_url",
				Rule:     "auth-oauth2-provider-url",
				Severity: SeverityError,
			})
		} else if !strings.HasPrefix(auth.ProviderURL, "https://") {
			errs = append(errs, ValidationError{
				Path:     prefix + ".provider_url",
				Message:  fmt.Sprintf("provider_url must use HTTPS, got %q", auth.ProviderURL),
				Rule:     "auth-provider-url-https",
				Severity: SeverityError,
			})
		}
		if len(auth.Scopes) == 0 {
			errs = append(errs, ValidationError{
				Path:     prefix + ".scopes",
				Message:  "oauth2 auth requires at least one scope",
				Rule:     "auth-oauth2-scopes",
				Severity: SeverityError,
			})
		}

	default:
		errs = append(errs, ValidationError{
			Path:     prefix + ".type",
			Message:  fmt.Sprintf("unknown auth type %q (must be one of: none, token, oauth2)", auth.Type),
			Rule:     "auth-type-unknown",
			Severity: SeverityError,
		})
	}

	return errs
}
