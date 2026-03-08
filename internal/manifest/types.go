package manifest

import (
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

// Toolkit represents a parsed toolwright manifest.
type Toolkit struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Tools      []Tool   `yaml:"tools"`
	Auth       *Auth    `yaml:"auth,omitempty"`
	Generate   Generate `yaml:"generate,omitempty"`
}

// Metadata holds toolkit metadata fields.
type Metadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Author      string `yaml:"author,omitempty"`
	License     string `yaml:"license,omitempty"`
	Repository  string `yaml:"repository,omitempty"`
}

// Tool represents a single tool definition within a toolkit.
type Tool struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Entrypoint  string           `yaml:"entrypoint"`
	Args        []Arg            `yaml:"args,omitempty"`
	Flags       []Flag           `yaml:"flags,omitempty"`
	Output      Output           `yaml:"output,omitempty"`
	Auth        *Auth            `yaml:"auth,omitempty"`
	Examples    []Example        `yaml:"examples,omitempty"`
	ExitCodes   map[int]string   `yaml:"exit_codes,omitempty"`
	Annotations *ToolAnnotations `yaml:"annotations,omitempty"`
}

// ToolAnnotations holds MCP-spec tool annotations describing tool behaviour.
type ToolAnnotations struct {
	ReadOnly    *bool  `yaml:"readOnly,omitempty"`
	Destructive *bool  `yaml:"destructive,omitempty"`
	Idempotent  *bool  `yaml:"idempotent,omitempty"`
	OpenWorld   *bool  `yaml:"openWorld,omitempty"`
	Title       string `yaml:"title,omitempty"`
}

// MarshalYAML serialises ToolAnnotations to a YAML mapping node, ensuring
// that *bool fields set to false are emitted as "false" rather than being
// silently dropped. Without this custom marshaler, future versions of
// go.yaml.in/yaml/v3 could change how omitempty handles *bool(false),
// breaking round-trip fidelity.
func (a ToolAnnotations) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	addBoolPtr := func(key string, v *bool) {
		if v == nil {
			return
		}
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool"}
		if *v {
			valNode.Value = "true"
		} else {
			valNode.Value = "false"
		}
		node.Content = append(node.Content, keyNode, valNode)
	}

	addBoolPtr("readOnly", a.ReadOnly)
	addBoolPtr("destructive", a.Destructive)
	addBoolPtr("idempotent", a.Idempotent)
	addBoolPtr("openWorld", a.OpenWorld)

	if a.Title != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "title"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: a.Title},
		)
	}

	return node, nil
}

// BoolPtr returns a pointer to the provided bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// Auth represents authentication configuration. It supports both string
// shorthand (e.g., "none") and full object form via custom UnmarshalYAML.
type Auth struct {
	Type        string     `yaml:"type"`
	TokenEnv    string     `yaml:"token_env,omitempty"`
	TokenFlag   string     `yaml:"token_flag,omitempty"`
	TokenHeader string     `yaml:"token_header,omitempty"`
	ProviderURL string     `yaml:"provider_url,omitempty"`
	ClientID    string     `yaml:"client_id,omitempty"`
	Endpoints   *Endpoints `yaml:"endpoints,omitempty"`
	Scopes      []string   `yaml:"scopes,omitempty"`
	Audience    string     `yaml:"audience,omitempty"`
}

// Endpoints represents manual OAuth endpoint overrides.
type Endpoints struct {
	Authorization string `yaml:"authorization"`
	Token         string `yaml:"token"`
	JWKS          string `yaml:"jwks"`
}

// Arg represents a positional argument for a tool.
type Arg struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

// Flag represents a named flag for a tool.
type Flag struct {
	Name        string         `yaml:"name"`
	Type        string         `yaml:"type"`
	Required    bool           `yaml:"required"`
	Default     any            `yaml:"default,omitempty"`
	Enum        []string       `yaml:"enum,omitempty"`
	Description string         `yaml:"description"`
	ItemSchema  map[string]any `yaml:"itemSchema,omitempty"`
}

// Output describes a tool's output format and optional schema.
type Output struct {
	Format   string `yaml:"format"`
	Schema   any    `yaml:"schema,omitempty"` // string path or map[string]any
	MimeType string `yaml:"mimeType,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshalling for Output.
// The type-alias decode naturally handles the Schema union: yaml/v3 decodes
// a scalar schema: as string and a mapping schema: as map[string]any into
// an any field.
func (o *Output) UnmarshalYAML(value *yaml.Node) error {
	type outputAlias Output
	var alias outputAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*o = Output(alias)
	return nil
}

// MarshalYAML implements custom YAML marshalling for Output to ensure
// round-trip fidelity. The omitempty tag on Schema any treats "" (empty
// string) as non-empty (it's a non-nil interface), so we emit it explicitly.
// When Schema is nil it is omitted; when MimeType is empty it is omitted.
func (o Output) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	// format (omit when empty for backward compat with zero-value Output)
	if o.Format != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "format"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: o.Format},
		)
	}

	// schema (omit when nil)
	if o.Schema != nil {
		schemaKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "schema"}
		var schemaVal yaml.Node
		if err := schemaVal.Encode(o.Schema); err != nil {
			return nil, fmt.Errorf("marshal output schema: %w", err)
		}
		// Encode wraps in a document node; unwrap it.
		if schemaVal.Kind == yaml.DocumentNode && len(schemaVal.Content) == 1 {
			node.Content = append(node.Content, schemaKey, schemaVal.Content[0])
		} else {
			node.Content = append(node.Content, schemaKey, &schemaVal)
		}
	}

	// mimeType (omit when empty)
	if o.MimeType != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mimeType"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: o.MimeType},
		)
	}

	return node, nil
}

// Example represents a usage example for a tool.
type Example struct {
	Description string            `yaml:"description"`
	Args        []string          `yaml:"args,omitempty"`
	Flags       map[string]string `yaml:"flags,omitempty"`
}

// Generate holds code generation configuration.
type Generate struct {
	CLI CLIConfig `yaml:"cli,omitempty"`
	MCP MCPConfig `yaml:"mcp,omitempty"`
}

// CLIConfig holds CLI generation settings.
type CLIConfig struct {
	Target string `yaml:"target"`
}

// MCPConfig holds MCP server generation settings.
type MCPConfig struct {
	Target    string   `yaml:"target"`
	Transport []string `yaml:"transport"`
}

// UnmarshalYAML implements custom YAML unmarshalling for Auth to support
// string shorthand (e.g., "none") in addition to the full object form.
func (a *Auth) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		a.Type = value.Value
		return nil
	}
	// Use a type alias to avoid infinite recursion when decoding a mapping node.
	type authAlias Auth
	var alias authAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*a = Auth(alias)
	return nil
}

// ResolvedAuth returns the effective auth for a tool. If the tool has its own
// auth, that is returned. Otherwise, the toolkit-level auth is returned. If
// neither is set, it returns Auth{Type: "none"}.
func (t *Toolkit) ResolvedAuth(tool Tool) Auth {
	if tool.Auth != nil {
		return *tool.Auth
	}
	if t.Auth != nil {
		return *t.Auth
	}
	return Auth{Type: "none"}
}

// MarshalYAML implements custom YAML marshalling for Example to preserve
// an empty (non-nil) Args slice as "args: []" rather than omitting it.
// This ensures a round-trip through marshal/unmarshal preserves the distinction
// between a manifest that specified "args: []" and one that omitted args entirely.
func (e Example) MarshalYAML() (interface{}, error) {
	type exampleAlias struct {
		Description string            `yaml:"description"`
		Args        []string          `yaml:"args,omitempty"`
		Flags       map[string]string `yaml:"flags,omitempty"`
	}
	out := exampleAlias(e)
	// When Args is a non-nil empty slice, use a node that emits "args: []"
	// so round-trip preserves the semantic difference.
	if e.Args != nil && len(e.Args) == 0 {
		type exampleWithEmptyArgs struct {
			Description string            `yaml:"description"`
			Args        []string          `yaml:"args"`
			Flags       map[string]string `yaml:"flags,omitempty"`
		}
		return exampleWithEmptyArgs(e), nil
	}
	return out, nil
}

// Parse reads YAML from r and returns a parsed Toolkit.
func Parse(r io.Reader) (*Toolkit, error) {
	dec := yaml.NewDecoder(r)
	var tk Toolkit
	if err := dec.Decode(&tk); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if tk.APIVersion != "toolwright/v1" {
		return nil, fmt.Errorf("parse manifest: unsupported apiVersion %q (want \"toolwright/v1\")", tk.APIVersion)
	}
	if tk.Kind != "Toolkit" {
		return nil, fmt.Errorf("parse manifest: unsupported kind %q (want \"Toolkit\")", tk.Kind)
	}
	return &tk, nil
}

// ParseFile reads a YAML file at path and returns a parsed Toolkit.
func ParseFile(path string) (*Toolkit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	defer f.Close()
	tk, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return tk, nil
}

// validArrayTypes maps valid array type strings to their scalar base.
var validArrayTypes = map[string]string{
	"string[]": "string",
	"int[]":    "int",
	"float[]":  "float",
	"bool[]":   "bool",
	"object[]": "object",
}

// IsArrayType reports whether flagType is a valid array type (e.g., "string[]").
func IsArrayType(flagType string) bool {
	_, ok := validArrayTypes[flagType]
	return ok
}

// BaseType returns the scalar base of an array type (e.g., "string[]" → "string").
// Returns "" for non-array types.
func BaseType(flagType string) string {
	return validArrayTypes[flagType]
}
