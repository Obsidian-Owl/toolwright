package manifest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a single validation issue.
type ValidationError struct {
	Path    string // e.g., "tools[0].args[1].name"
	Message string // Human-readable description
	Rule    string // Machine identifier: "name-format", "semver", etc.
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
	"string[]": true,
	"int[]":    true,
	"float[]":  true,
	"bool[]":   true,
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
			Path:    "metadata.name",
			Message: fmt.Sprintf("name %q must match ^[a-z0-9-]+$", m.Name),
			Rule:    "name-format",
		})
	}

	if !semverRe.MatchString(m.Version) {
		errs = append(errs, ValidationError{
			Path:    "metadata.version",
			Message: fmt.Sprintf("version %q is not valid SemVer", m.Version),
			Rule:    "semver",
		})
	}

	if m.Description == "" {
		errs = append(errs, ValidationError{
			Path:    "metadata.description",
			Message: "description is required",
			Rule:    "description-required",
		})
	} else if len(m.Description) > 200 {
		errs = append(errs, ValidationError{
			Path:    "metadata.description",
			Message: fmt.Sprintf("description must be 200 characters or fewer, got %d", len(m.Description)),
			Rule:    "description-length",
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
				Path:    "tools",
				Message: fmt.Sprintf("tool name %q is not unique", tool.Name),
				Rule:    "unique-tool-name",
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
				Path:    prefix + ".args",
				Message: fmt.Sprintf("arg name %q is not unique within tool", arg.Name),
				Rule:    "unique-arg-name",
			})
		}
		seenArgs[arg.Name] = true
	}

	// Duplicate flag names.
	seenFlags := make(map[string]bool)
	for _, flag := range tool.Flags {
		if seenFlags[flag.Name] {
			errs = append(errs, ValidationError{
				Path:    prefix + ".flags",
				Message: fmt.Sprintf("flag name %q is not unique within tool", flag.Name),
				Rule:    "unique-flag-name",
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
			Path:    prefix + ".type",
			Message: fmt.Sprintf("unknown flag type %q (must be one of: string, int, float, bool, string[], int[], float[], bool[])", flag.Type),
			Rule:    "unknown-flag-type",
		})
		return errs
	}

	// Check default value type.
	if flag.Default != nil {
		if err := checkDefaultType(flag.Type, flag.Default); err != nil {
			errs = append(errs, ValidationError{
				Path:    prefix + ".default",
				Message: err.Error(),
				Rule:    "type-mismatch",
			})
		}
	}

	// Check enum values match declared type.
	if len(flag.Enum) > 0 {
		if err := checkEnumType(flag.Type, flag.Enum); err != nil {
			errs = append(errs, ValidationError{
				Path:    prefix + ".enum",
				Message: err.Error(),
				Rule:    "type-mismatch",
			})
		}
	}

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

func validateAuth(auth Auth, prefix string) []ValidationError {
	var errs []ValidationError

	switch auth.Type {
	case "none":
		// no additional validation

	case "token":
		if auth.TokenEnv == "" {
			errs = append(errs, ValidationError{
				Path:    prefix + ".token_env",
				Message: "token auth requires token_env",
				Rule:    "auth-token-env",
			})
		}
		if auth.TokenFlag == "" {
			errs = append(errs, ValidationError{
				Path:    prefix + ".token_flag",
				Message: "token auth requires token_flag",
				Rule:    "auth-token-flag",
			})
		}

	case "oauth2":
		if auth.ProviderURL == "" {
			errs = append(errs, ValidationError{
				Path:    prefix + ".provider_url",
				Message: "oauth2 auth requires provider_url",
				Rule:    "auth-oauth2-provider-url",
			})
		} else if !strings.HasPrefix(auth.ProviderURL, "https://") {
			errs = append(errs, ValidationError{
				Path:    prefix + ".provider_url",
				Message: fmt.Sprintf("provider_url must use HTTPS, got %q", auth.ProviderURL),
				Rule:    "auth-provider-url-https",
			})
		}
		if len(auth.Scopes) == 0 {
			errs = append(errs, ValidationError{
				Path:    prefix + ".scopes",
				Message: "oauth2 auth requires at least one scope",
				Rule:    "auth-oauth2-scopes",
			})
		}

	default:
		errs = append(errs, ValidationError{
			Path:    prefix + ".type",
			Message: fmt.Sprintf("unknown auth type %q (must be one of: none, token, oauth2)", auth.Type),
			Rule:    "auth-type-unknown",
		})
	}

	return errs
}
