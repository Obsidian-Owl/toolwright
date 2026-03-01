package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// ---------------------------------------------------------------------------
// Test fixtures — full manifest YAML
// ---------------------------------------------------------------------------

// fullManifestYAML is a realistic manifest with two tools, toolkit-level auth,
// and a generate section. Used across multiple tests. Every field that exists
// in the type system is exercised here so a sloppy parser that skips fields
// will be caught.
const fullManifestYAML = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: petstore-tools
  version: 1.0.0
  description: Pet store management toolkit
  author: Jane Doe
  license: MIT
  repository: https://github.com/example/petstore-tools
tools:
  - name: list-pets
    description: List all pets with optional filtering
    entrypoint: ./scripts/list-pets.sh
    args:
      - name: species
        type: string
        required: false
        description: Filter by species
    flags:
      - name: limit
        type: int
        required: false
        default: 25
        description: Maximum number of results
      - name: format
        type: string
        required: false
        default: table
        enum:
          - table
          - json
          - csv
        description: Output format
    output:
      format: json
      schema: schemas/pets.schema.json
    examples:
      - description: List first 10 cats
        args:
          - cat
        flags:
          limit: "10"
          format: json
      - description: List all pets as table
        args: []
        flags:
          format: table
    exit_codes:
      0: success
      1: general error
      2: authentication failure
  - name: add-pet
    description: Add a new pet to the store
    entrypoint: ./scripts/add-pet.sh
    args:
      - name: name
        type: string
        required: true
        description: Pet name
      - name: species
        type: string
        required: true
        description: Pet species
    flags:
      - name: age
        type: int
        required: false
        description: Pet age in years
      - name: vaccinated
        type: bool
        required: false
        default: false
        description: Whether the pet is vaccinated
    output:
      format: json
    auth:
      type: token
      token_env: PETSTORE_TOKEN
      token_flag: --token
      token_header: "Authorization: Bearer"
    examples:
      - description: Add a vaccinated cat
        args:
          - Whiskers
          - cat
        flags:
          age: "3"
          vaccinated: "true"
    exit_codes:
      0: success
      1: general error
      3: duplicate pet
auth:
  type: oauth2
  provider_url: https://auth.petstore.example.com
  scopes:
    - pets:read
    - pets:write
  token_env: PETSTORE_TOKEN
  token_flag: --token
  audience: https://api.petstore.example.com
generate:
  cli:
    target: go
  mcp:
    target: typescript
    transport:
      - stdio
      - sse
`

// expectedToolkit returns the fully-populated Toolkit struct that fullManifestYAML
// should parse into. This is the single source of truth for AC-1 assertions.
func expectedToolkit() *Toolkit {
	return &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: Metadata{
			Name:        "petstore-tools",
			Version:     "1.0.0",
			Description: "Pet store management toolkit",
			Author:      "Jane Doe",
			License:     "MIT",
			Repository:  "https://github.com/example/petstore-tools",
		},
		Tools: []Tool{
			{
				Name:        "list-pets",
				Description: "List all pets with optional filtering",
				Entrypoint:  "./scripts/list-pets.sh",
				Args: []Arg{
					{Name: "species", Type: "string", Required: false, Description: "Filter by species"},
				},
				Flags: []Flag{
					{Name: "limit", Type: "int", Required: false, Default: 25, Description: "Maximum number of results"},
					{Name: "format", Type: "string", Required: false, Default: "table", Enum: []string{"table", "json", "csv"}, Description: "Output format"},
				},
				Output: Output{Format: "json", Schema: "schemas/pets.schema.json"},
				Examples: []Example{
					{
						Description: "List first 10 cats",
						Args:        []string{"cat"},
						Flags:       map[string]string{"limit": "10", "format": "json"},
					},
					{
						Description: "List all pets as table",
						Args:        []string{},
						Flags:       map[string]string{"format": "table"},
					},
				},
				ExitCodes: map[int]string{0: "success", 1: "general error", 2: "authentication failure"},
			},
			{
				Name:        "add-pet",
				Description: "Add a new pet to the store",
				Entrypoint:  "./scripts/add-pet.sh",
				Args: []Arg{
					{Name: "name", Type: "string", Required: true, Description: "Pet name"},
					{Name: "species", Type: "string", Required: true, Description: "Pet species"},
				},
				Flags: []Flag{
					{Name: "age", Type: "int", Required: false, Description: "Pet age in years"},
					{Name: "vaccinated", Type: "bool", Required: false, Default: false, Description: "Whether the pet is vaccinated"},
				},
				Output: Output{Format: "json"},
				Auth: &Auth{
					Type:        "token",
					TokenEnv:    "PETSTORE_TOKEN",
					TokenFlag:   "--token",
					TokenHeader: "Authorization: Bearer",
				},
				Examples: []Example{
					{
						Description: "Add a vaccinated cat",
						Args:        []string{"Whiskers", "cat"},
						Flags:       map[string]string{"age": "3", "vaccinated": "true"},
					},
				},
				ExitCodes: map[int]string{0: "success", 1: "general error", 3: "duplicate pet"},
			},
		},
		Auth: &Auth{
			Type:        "oauth2",
			ProviderURL: "https://auth.petstore.example.com",
			Scopes:      []string{"pets:read", "pets:write"},
			TokenEnv:    "PETSTORE_TOKEN",
			TokenFlag:   "--token",
			Audience:    "https://api.petstore.example.com",
		},
		Generate: Generate{
			CLI: CLIConfig{Target: "go"},
			MCP: MCPConfig{Target: "typescript", Transport: []string{"stdio", "sse"}},
		},
	}
}

// ---------------------------------------------------------------------------
// AC-1: YAML manifest parses into Toolkit struct
// ---------------------------------------------------------------------------

func TestParse_FullManifest(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err, "Parse should not return an error for a valid manifest")
	require.NotNil(t, got, "Parse must return a non-nil Toolkit")

	want := expectedToolkit()

	// Use go-cmp for deep structural comparison so that any missing field
	// produces a clear diff. A naive parser that skips nested structs will
	// fail here.
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Parse(fullManifest) mismatch (-want +got):\n%s", diff)
	}
}

func TestParse_FullManifest_TopLevelFields(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)

	// Explicit field-level assertions to ensure nothing is silently zero-valued.
	assert.Equal(t, "toolwright/v1", got.APIVersion, "APIVersion")
	assert.Equal(t, "Toolkit", got.Kind, "Kind")
	assert.Equal(t, "petstore-tools", got.Metadata.Name, "Metadata.Name")
	assert.Equal(t, "1.0.0", got.Metadata.Version, "Metadata.Version")
	assert.Equal(t, "Pet store management toolkit", got.Metadata.Description, "Metadata.Description")
	assert.Equal(t, "Jane Doe", got.Metadata.Author, "Metadata.Author")
	assert.Equal(t, "MIT", got.Metadata.License, "Metadata.License")
	assert.Equal(t, "https://github.com/example/petstore-tools", got.Metadata.Repository, "Metadata.Repository")
}

func TestParse_FullManifest_ToolCount(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Tools, 2, "Should have exactly 2 tools")
}

func TestParse_FullManifest_Tool0_Fields(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 2)

	tool := got.Tools[0]

	assert.Equal(t, "list-pets", tool.Name)
	assert.Equal(t, "List all pets with optional filtering", tool.Description)
	assert.Equal(t, "./scripts/list-pets.sh", tool.Entrypoint)

	// Args
	require.Len(t, tool.Args, 1, "list-pets should have 1 arg")
	assert.Equal(t, "species", tool.Args[0].Name)
	assert.Equal(t, "string", tool.Args[0].Type)
	assert.False(t, tool.Args[0].Required)
	assert.Equal(t, "Filter by species", tool.Args[0].Description)

	// Flags
	require.Len(t, tool.Flags, 2, "list-pets should have 2 flags")
	assert.Equal(t, "limit", tool.Flags[0].Name)
	assert.Equal(t, "int", tool.Flags[0].Type)
	assert.False(t, tool.Flags[0].Required)
	assert.Equal(t, 25, tool.Flags[0].Default, "Default for limit flag should be int 25, not string")
	assert.Equal(t, "Maximum number of results", tool.Flags[0].Description)

	assert.Equal(t, "format", tool.Flags[1].Name)
	assert.Equal(t, "string", tool.Flags[1].Type)
	assert.Equal(t, "table", tool.Flags[1].Default)
	assert.Equal(t, []string{"table", "json", "csv"}, tool.Flags[1].Enum, "Enum values must be preserved in order")

	// Output
	assert.Equal(t, "json", tool.Output.Format)
	assert.Equal(t, "schemas/pets.schema.json", tool.Output.Schema)

	// Examples
	require.Len(t, tool.Examples, 2, "list-pets should have 2 examples")
	assert.Equal(t, "List first 10 cats", tool.Examples[0].Description)
	assert.Equal(t, []string{"cat"}, tool.Examples[0].Args)
	assert.Equal(t, map[string]string{"limit": "10", "format": "json"}, tool.Examples[0].Flags)

	assert.Equal(t, "List all pets as table", tool.Examples[1].Description)
	assert.Equal(t, []string{}, tool.Examples[1].Args, "Empty args list should be an empty slice, not nil")
	assert.Equal(t, map[string]string{"format": "table"}, tool.Examples[1].Flags)

	// ExitCodes
	require.Len(t, tool.ExitCodes, 3, "list-pets should have 3 exit codes")
	assert.Equal(t, "success", tool.ExitCodes[0])
	assert.Equal(t, "general error", tool.ExitCodes[1])
	assert.Equal(t, "authentication failure", tool.ExitCodes[2])

	// No tool-level auth
	assert.Nil(t, tool.Auth, "list-pets should not have tool-level auth")
}

func TestParse_FullManifest_Tool1_Auth(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 2)

	tool := got.Tools[1]
	assert.Equal(t, "add-pet", tool.Name)

	require.NotNil(t, tool.Auth, "add-pet must have tool-level auth")
	assert.Equal(t, "token", tool.Auth.Type)
	assert.Equal(t, "PETSTORE_TOKEN", tool.Auth.TokenEnv)
	assert.Equal(t, "--token", tool.Auth.TokenFlag)
	assert.Equal(t, "Authorization: Bearer", tool.Auth.TokenHeader)
	assert.Nil(t, tool.Auth.Endpoints, "token auth should not have endpoints")
	assert.Empty(t, tool.Auth.Scopes, "token auth should not have scopes")
}

func TestParse_FullManifest_ToolkitAuth(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)

	require.NotNil(t, got.Auth, "Toolkit-level auth must be present")
	assert.Equal(t, "oauth2", got.Auth.Type)
	assert.Equal(t, "https://auth.petstore.example.com", got.Auth.ProviderURL)
	assert.Equal(t, []string{"pets:read", "pets:write"}, got.Auth.Scopes)
	assert.Equal(t, "PETSTORE_TOKEN", got.Auth.TokenEnv)
	assert.Equal(t, "--token", got.Auth.TokenFlag)
	assert.Equal(t, "https://api.petstore.example.com", got.Auth.Audience)
	assert.Nil(t, got.Auth.Endpoints, "No manual endpoints in this manifest")
}

func TestParse_FullManifest_Generate(t *testing.T) {
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "go", got.Generate.CLI.Target)
	assert.Equal(t, "typescript", got.Generate.MCP.Target)
	assert.Equal(t, []string{"stdio", "sse"}, got.Generate.MCP.Transport)
}

// ---------------------------------------------------------------------------
// AC-1 extended: ParseFile reads from disk
// ---------------------------------------------------------------------------

func TestParseFile_ReadsFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(fullManifestYAML), 0644)
	require.NoError(t, err)

	got, err := ParseFile(path)
	require.NoError(t, err, "ParseFile should not return an error for a valid file")
	require.NotNil(t, got, "ParseFile must return a non-nil Toolkit")

	want := expectedToolkit()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseFile mismatch (-want +got):\n%s", diff)
	}
}

func TestParseFile_NonexistentFile(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/toolwright.yaml")
	require.Error(t, err, "ParseFile must error for a nonexistent file")
}

// ---------------------------------------------------------------------------
// AC-2: Auth string shorthand unmarshals correctly
// ---------------------------------------------------------------------------

func TestAuth_UnmarshalYAML_StringShorthand(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantType string
	}{
		{
			name:     "none shorthand",
			yaml:     `auth: none`,
			wantType: "none",
		},
		{
			name:     "token shorthand",
			yaml:     `auth: token`,
			wantType: "token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap in a struct to test the auth field unmarshalling.
			type wrapper struct {
				Auth *Auth `yaml:"auth"`
			}
			var w wrapper
			err := yaml.Unmarshal([]byte(tc.yaml), &w)
			require.NoError(t, err, "Unmarshal should handle string shorthand")
			require.NotNil(t, w.Auth, "Auth pointer must not be nil for string shorthand")
			assert.Equal(t, tc.wantType, w.Auth.Type, "Auth.Type should match the shorthand string")

			// Ensure other fields are zero when using shorthand — a naive impl
			// that doesn't clear fields would fail here.
			assert.Empty(t, w.Auth.TokenEnv, "String shorthand should not populate TokenEnv")
			assert.Empty(t, w.Auth.ProviderURL, "String shorthand should not populate ProviderURL")
			assert.Nil(t, w.Auth.Endpoints, "String shorthand should not populate Endpoints")
			assert.Nil(t, w.Auth.Scopes, "String shorthand should not populate Scopes")
			assert.Empty(t, w.Auth.Audience, "String shorthand should not populate Audience")
		})
	}
}

func TestAuth_UnmarshalYAML_FullObject(t *testing.T) {
	input := `auth:
  type: oauth2
  provider_url: https://auth.example.com
  scopes:
    - read
    - write
  token_env: MY_TOKEN
  token_flag: --token
  token_header: "Bearer"
  audience: https://api.example.com
`
	type wrapper struct {
		Auth *Auth `yaml:"auth"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte(input), &w)
	require.NoError(t, err)
	require.NotNil(t, w.Auth)

	assert.Equal(t, "oauth2", w.Auth.Type)
	assert.Equal(t, "https://auth.example.com", w.Auth.ProviderURL)
	assert.Equal(t, []string{"read", "write"}, w.Auth.Scopes)
	assert.Equal(t, "MY_TOKEN", w.Auth.TokenEnv)
	assert.Equal(t, "--token", w.Auth.TokenFlag)
	assert.Equal(t, "Bearer", w.Auth.TokenHeader)
	assert.Equal(t, "https://api.example.com", w.Auth.Audience)
	assert.Nil(t, w.Auth.Endpoints, "No endpoints in this object")
}

func TestAuth_UnmarshalYAML_ShorthandInManifest(t *testing.T) {
	// Auth "none" shorthand used at the toolkit level in a real manifest.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: simple-tool
  version: 0.1.0
  description: A simple tool
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
auth: none
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Auth, "Auth pointer should not be nil for 'auth: none'")
	assert.Equal(t, "none", got.Auth.Type)
}

// ---------------------------------------------------------------------------
// AC-2: ResolvedAuth returns correct auth based on precedence
// ---------------------------------------------------------------------------

func TestToolkit_ResolvedAuth(t *testing.T) {
	toolkitAuth := &Auth{
		Type:        "oauth2",
		ProviderURL: "https://auth.example.com",
		Scopes:      []string{"read"},
		TokenEnv:    "TK_TOKEN",
		TokenFlag:   "--token",
	}
	toolAuth := &Auth{
		Type:      "token",
		TokenEnv:  "TOOL_TOKEN",
		TokenFlag: "--tool-token",
	}

	tests := []struct {
		name        string
		toolkitAuth *Auth
		toolAuth    *Auth
		wantType    string
		wantEnv     string
	}{
		{
			name:        "tool auth overrides toolkit auth",
			toolkitAuth: toolkitAuth,
			toolAuth:    toolAuth,
			wantType:    "token",
			wantEnv:     "TOOL_TOKEN",
		},
		{
			name:        "toolkit auth when tool has no auth",
			toolkitAuth: toolkitAuth,
			toolAuth:    nil,
			wantType:    "oauth2",
			wantEnv:     "TK_TOKEN",
		},
		{
			name:        "defaults to none when neither has auth",
			toolkitAuth: nil,
			toolAuth:    nil,
			wantType:    "none",
			wantEnv:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := &Toolkit{Auth: tc.toolkitAuth}
			tool := Tool{Auth: tc.toolAuth}

			got := tk.ResolvedAuth(tool)

			assert.Equal(t, tc.wantType, got.Type, "ResolvedAuth Type")
			assert.Equal(t, tc.wantEnv, got.TokenEnv, "ResolvedAuth TokenEnv")
		})
	}
}

func TestToolkit_ResolvedAuth_ReturnsValueNotPointer(t *testing.T) {
	// ResolvedAuth returns Auth (value), not *Auth. Verify it returns a full
	// copy that can be compared without pointer concerns.
	toolAuth := &Auth{
		Type:        "token",
		TokenEnv:    "MY_TOKEN",
		TokenFlag:   "--token",
		TokenHeader: "Authorization: Bearer",
	}
	tk := &Toolkit{}
	tool := Tool{Auth: toolAuth}

	got := tk.ResolvedAuth(tool)

	// All fields from the tool auth should be present in the returned value.
	assert.Equal(t, "token", got.Type)
	assert.Equal(t, "MY_TOKEN", got.TokenEnv)
	assert.Equal(t, "--token", got.TokenFlag)
	assert.Equal(t, "Authorization: Bearer", got.TokenHeader)
}

func TestToolkit_ResolvedAuth_ToolkitAuthPreservesAllFields(t *testing.T) {
	// When falling back to toolkit auth, ALL fields should be returned, not
	// just the Type.
	tkAuth := &Auth{
		Type:        "oauth2",
		ProviderURL: "https://auth.example.com",
		Scopes:      []string{"a", "b"},
		TokenEnv:    "E",
		TokenFlag:   "--f",
		Audience:    "aud",
	}
	tk := &Toolkit{Auth: tkAuth}
	tool := Tool{} // no tool auth

	got := tk.ResolvedAuth(tool)

	assert.Equal(t, "oauth2", got.Type)
	assert.Equal(t, "https://auth.example.com", got.ProviderURL)
	assert.Equal(t, []string{"a", "b"}, got.Scopes)
	assert.Equal(t, "E", got.TokenEnv)
	assert.Equal(t, "--f", got.TokenFlag)
	assert.Equal(t, "aud", got.Audience)
}

func TestToolkit_ResolvedAuth_NoneDefault_HasNoExtraFields(t *testing.T) {
	// When defaulting to none, only Type should be set. Nothing else.
	tk := &Toolkit{}
	tool := Tool{}

	got := tk.ResolvedAuth(tool)

	assert.Equal(t, "none", got.Type)
	assert.Empty(t, got.TokenEnv)
	assert.Empty(t, got.TokenFlag)
	assert.Empty(t, got.TokenHeader)
	assert.Empty(t, got.ProviderURL)
	assert.Nil(t, got.Endpoints)
	assert.Nil(t, got.Scopes)
	assert.Empty(t, got.Audience)
}

// ---------------------------------------------------------------------------
// AC-3: Auth with manual endpoints
// ---------------------------------------------------------------------------

func TestAuth_WithEndpoints(t *testing.T) {
	input := `auth:
  type: oauth2
  provider_url: https://auth.example.com
  scopes:
    - read
  token_env: MY_TOKEN
  token_flag: --token
  endpoints:
    authorization: https://auth.example.com/authorize
    token: https://auth.example.com/token
    jwks: https://auth.example.com/.well-known/jwks.json
`
	type wrapper struct {
		Auth *Auth `yaml:"auth"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte(input), &w)
	require.NoError(t, err)
	require.NotNil(t, w.Auth)
	require.NotNil(t, w.Auth.Endpoints, "Endpoints must not be nil when specified")

	assert.Equal(t, "https://auth.example.com/authorize", w.Auth.Endpoints.Authorization)
	assert.Equal(t, "https://auth.example.com/token", w.Auth.Endpoints.Token)
	assert.Equal(t, "https://auth.example.com/.well-known/jwks.json", w.Auth.Endpoints.JWKS)
}

func TestAuth_WithoutEndpoints_PointersNil(t *testing.T) {
	input := `auth:
  type: oauth2
  provider_url: https://auth.example.com
  scopes:
    - read
  token_env: MY_TOKEN
  token_flag: --token
`
	type wrapper struct {
		Auth *Auth `yaml:"auth"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte(input), &w)
	require.NoError(t, err)
	require.NotNil(t, w.Auth)
	assert.Nil(t, w.Auth.Endpoints, "Endpoints must be nil when not specified")
}

func TestParse_ManifestWithEndpoints(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: ep-test
  version: 0.1.0
  description: Endpoints test
tools:
  - name: do-thing
    description: Does a thing
    entrypoint: ./thing.sh
auth:
  type: oauth2
  provider_url: https://auth.example.com
  scopes:
    - read
  token_env: MY_TOKEN
  token_flag: --token
  endpoints:
    authorization: https://auth.example.com/authorize
    token: https://auth.example.com/oauth/token
    jwks: https://auth.example.com/jwks
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Auth)
	require.NotNil(t, got.Auth.Endpoints)
	assert.Equal(t, "https://auth.example.com/authorize", got.Auth.Endpoints.Authorization)
	assert.Equal(t, "https://auth.example.com/oauth/token", got.Auth.Endpoints.Token)
	assert.Equal(t, "https://auth.example.com/jwks", got.Auth.Endpoints.JWKS)
}

// ---------------------------------------------------------------------------
// AC-7: Parser rejects malformed YAML
// ---------------------------------------------------------------------------

func TestParse_MalformedInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "only whitespace",
			input: "   \n\n  \t  \n",
		},
		{
			name:  "missing apiVersion",
			input: "kind: Toolkit\nmetadata:\n  name: test\n  version: 1.0.0\n  description: desc\n",
		},
		{
			name:  "wrong kind",
			input: "apiVersion: toolwright/v1\nkind: NotAToolkit\nmetadata:\n  name: test\n  version: 1.0.0\n  description: desc\n",
		},
		{
			name:  "kind missing entirely",
			input: "apiVersion: toolwright/v1\nmetadata:\n  name: test\n  version: 1.0.0\n  description: desc\n",
		},
		{
			name:  "binary garbage",
			input: "\x00\x01\x02\x03\x04\x05\xff\xfe\xfd",
		},
		{
			name:  "invalid YAML syntax",
			input: "apiVersion: toolwright/v1\n  bad indent: {[}\n",
		},
		{
			name:  "YAML but completely wrong structure (scalar)",
			input: "42",
		},
		{
			name:  "YAML array instead of object",
			input: "- item1\n- item2\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Parse(strings.NewReader(tc.input))
			require.Error(t, err, "Parse should return an error for: %s", tc.name)

			// A returned non-nil toolkit alongside an error would be confusing.
			// The contract should be: error means nil result.
			assert.Nil(t, result, "Parse should return nil toolkit on error")
		})
	}
}

func TestParse_EmptyInput_ErrorMessage(t *testing.T) {
	_, err := Parse(strings.NewReader(""))
	require.Error(t, err)
	// The error should be descriptive, not a generic YAML decode error.
	// We do not prescribe exact wording, but it should not be empty.
	assert.NotEmpty(t, err.Error(), "Error message should be non-empty")
}

func TestParse_WrongKind_ErrorContainsKind(t *testing.T) {
	input := "apiVersion: toolwright/v1\nkind: WrongKind\nmetadata:\n  name: test\n  version: 1.0.0\n  description: d\n"
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	// The error should mention "kind" or "Toolkit" so the user knows what to fix.
	errMsg := strings.ToLower(err.Error())
	assert.True(t,
		strings.Contains(errMsg, "kind") || strings.Contains(errMsg, "toolkit"),
		"Error for wrong kind should mention 'kind' or 'Toolkit', got: %s", err.Error(),
	)
}

func TestParse_MissingAPIVersion_ErrorContainsAPIVersion(t *testing.T) {
	input := "kind: Toolkit\nmetadata:\n  name: test\n  version: 1.0.0\n  description: d\n"
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	errMsg := strings.ToLower(err.Error())
	assert.True(t,
		strings.Contains(errMsg, "apiversion") || strings.Contains(errMsg, "version"),
		"Error for missing apiVersion should mention it, got: %s", err.Error(),
	)
}

// ---------------------------------------------------------------------------
// AC-7 extended: ParseFile rejects malformed files
// ---------------------------------------------------------------------------

func TestParseFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)

	result, err := ParseFile(path)
	require.Error(t, err, "ParseFile should error on empty file")
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// AC-11: Round-trip through YAML
// ---------------------------------------------------------------------------

func TestParse_RoundTrip(t *testing.T) {
	// Step 1: Parse the original YAML.
	original, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err, "First parse should succeed")
	require.NotNil(t, original)

	// Step 2: Marshal back to YAML.
	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err, "Marshal should succeed")
	require.NotEmpty(t, marshalled, "Marshalled YAML must not be empty")

	// Step 3: Parse the marshalled YAML again.
	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err, "Second parse (round-trip) should succeed")
	require.NotNil(t, roundTripped)

	// Step 4: Compare the two parsed structs. They must be deeply equal.
	// The spec says auth string shorthand is NOT preserved (marshals to
	// object form), which is acceptable. Both parses produce the same struct.
	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch (-original +roundTripped):\n%s", diff)
	}
}

func TestParse_RoundTrip_MinimalManifest(t *testing.T) {
	// Test round-trip with a minimal manifest (no optional fields) to ensure
	// omitempty works correctly and doesn't introduce spurious zero values.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: minimal
  version: 0.1.0
  description: Minimal manifest
tools:
  - name: hello
    description: Says hello
    entrypoint: ./hello.sh
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, original)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.NotNil(t, roundTripped)

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Minimal round-trip mismatch (-original +roundTripped):\n%s", diff)
	}
}

func TestParse_RoundTrip_AuthNoneShorthand(t *testing.T) {
	// Auth string shorthand "none" is NOT preserved after marshal (per spec).
	// After round-trip, auth should still be {Type: "none"} regardless of form.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: auth-test
  version: 0.1.0
  description: Auth round-trip test
tools:
  - name: hello
    description: Says hello
    entrypoint: ./hello.sh
auth: none
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, original)
	require.NotNil(t, original.Auth)
	assert.Equal(t, "none", original.Auth.Type)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.NotNil(t, roundTripped)
	require.NotNil(t, roundTripped.Auth)
	assert.Equal(t, "none", roundTripped.Auth.Type)
}

// ---------------------------------------------------------------------------
// Edge cases: Ensure parser handles unusual but valid inputs
// ---------------------------------------------------------------------------

func TestParse_ToolWithNoFlags(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: no-flags
  version: 0.1.0
  description: Tool with no flags
tools:
  - name: simple
    description: A simple tool
    entrypoint: ./run.sh
    args:
      - name: input
        type: string
        required: true
        description: Input value
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	assert.Len(t, got.Tools[0].Args, 1)
	assert.Empty(t, got.Tools[0].Flags, "Flags should be empty when not specified")
	assert.Empty(t, got.Tools[0].Examples, "Examples should be empty when not specified")
	assert.Nil(t, got.Tools[0].ExitCodes, "ExitCodes should be nil when not specified")
}

func TestParse_ToolWithNoArgs(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: no-args
  version: 0.1.0
  description: Tool with no args
tools:
  - name: status
    description: Check status
    entrypoint: ./status.sh
    flags:
      - name: verbose
        type: bool
        required: false
        default: false
        description: Enable verbose output
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	assert.Empty(t, got.Tools[0].Args, "Args should be empty when not specified")
	assert.Len(t, got.Tools[0].Flags, 1)
}

func TestParse_NoAuth(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: no-auth
  version: 0.1.0
  description: No auth at all
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Nil(t, got.Auth, "Auth should be nil when not specified in manifest")
}

func TestParse_NoGenerate(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: no-generate
  version: 0.1.0
  description: No generate section
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	// Generate should be the zero value, not nil (it's not a pointer).
	assert.Empty(t, got.Generate.CLI.Target, "CLI target should be empty when generate not specified")
	assert.Empty(t, got.Generate.MCP.Target, "MCP target should be empty when generate not specified")
}

func TestParse_MultipleToolsSameOrder(t *testing.T) {
	// Verify tools are parsed in the order they appear in the YAML.
	// A parser that uses a map internally might scramble the order.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: ordered
  version: 0.1.0
  description: Order matters
tools:
  - name: alpha
    description: First
    entrypoint: ./alpha.sh
  - name: bravo
    description: Second
    entrypoint: ./bravo.sh
  - name: charlie
    description: Third
    entrypoint: ./charlie.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 3)
	assert.Equal(t, "alpha", got.Tools[0].Name)
	assert.Equal(t, "bravo", got.Tools[1].Name)
	assert.Equal(t, "charlie", got.Tools[2].Name)
}

func TestParse_FlagDefaultTypes(t *testing.T) {
	// Ensure default values maintain their typed nature through parsing.
	// An int default should parse as int, bool as bool, string as string.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: flag-defaults
  version: 0.1.0
  description: Flag default type test
tools:
  - name: typed
    description: Typed flags
    entrypoint: ./typed.sh
    flags:
      - name: count
        type: int
        required: false
        default: 42
        description: A count
      - name: enabled
        type: bool
        required: false
        default: true
        description: A boolean
      - name: label
        type: string
        required: false
        default: hello
        description: A label
      - name: rate
        type: float
        required: false
        default: 3.14
        description: A rate
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	flags := got.Tools[0].Flags
	require.Len(t, flags, 4)

	// int default
	assert.Equal(t, 42, flags[0].Default, "int default should be numeric 42")
	// bool default
	assert.Equal(t, true, flags[1].Default, "bool default should be boolean true")
	// string default
	assert.Equal(t, "hello", flags[2].Default, "string default should be string")
	// float default
	assert.InDelta(t, 3.14, flags[3].Default, 0.001, "float default should be numeric 3.14")
}

func TestParse_ExitCodesWithIntKeys(t *testing.T) {
	// Exit codes use integer keys. Ensure they are not silently converted
	// to string keys or dropped.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: exit-test
  version: 0.1.0
  description: Exit code test
tools:
  - name: exiter
    description: Has exit codes
    entrypoint: ./exit.sh
    exit_codes:
      0: success
      1: general failure
      127: command not found
      255: unknown error
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	ec := got.Tools[0].ExitCodes
	require.Len(t, ec, 4)
	assert.Equal(t, "success", ec[0])
	assert.Equal(t, "general failure", ec[1])
	assert.Equal(t, "command not found", ec[127])
	assert.Equal(t, "unknown error", ec[255])
}

func TestParse_ExampleWithNoArgsOrFlags(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: example-test
  version: 0.1.0
  description: Example test
tools:
  - name: bare
    description: A bare tool
    entrypoint: ./bare.sh
    examples:
      - description: Run with no arguments
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Examples, 1)
	assert.Equal(t, "Run with no arguments", got.Tools[0].Examples[0].Description)
	// Args and Flags should be nil or empty, not cause a panic.
	assert.Empty(t, got.Tools[0].Examples[0].Args)
	assert.Empty(t, got.Tools[0].Examples[0].Flags)
}

func TestParse_ToolLevelAuthWithEndpoints(t *testing.T) {
	// Endpoints at the tool level (not just toolkit level).
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: tool-ep
  version: 0.1.0
  description: Tool-level endpoints
tools:
  - name: secured
    description: A secured tool
    entrypoint: ./secured.sh
    auth:
      type: oauth2
      provider_url: https://tool-auth.example.com
      scopes:
        - admin
      token_env: TOOL_TOKEN
      token_flag: --token
      endpoints:
        authorization: https://tool-auth.example.com/auth
        token: https://tool-auth.example.com/token
        jwks: https://tool-auth.example.com/keys
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.NotNil(t, got.Tools[0].Auth)
	require.NotNil(t, got.Tools[0].Auth.Endpoints)
	assert.Equal(t, "https://tool-auth.example.com/auth", got.Tools[0].Auth.Endpoints.Authorization)
	assert.Equal(t, "https://tool-auth.example.com/token", got.Tools[0].Auth.Endpoints.Token)
	assert.Equal(t, "https://tool-auth.example.com/keys", got.Tools[0].Auth.Endpoints.JWKS)
}

// ---------------------------------------------------------------------------
// Cross-cutting: Parse returns wrapped errors (Constitution rule 4)
// ---------------------------------------------------------------------------

func TestParse_ErrorsAreWrapped(t *testing.T) {
	// The spec says errors are wrapped with context. We verify Parse errors
	// contain useful context, not bare YAML library errors.
	_, err := Parse(strings.NewReader("\x00\x01\x02"))
	require.Error(t, err)
	// The error message should not be just "yaml: ..." — it should have
	// manifest-specific context. This is a soft check: we just verify
	// the error is not empty and is printable.
	assert.NotEmpty(t, err.Error())
}

func TestParseFile_ErrorWrapsPath(t *testing.T) {
	path := "/tmp/nonexistent-toolwright-test-file.yaml"
	_, err := ParseFile(path)
	require.Error(t, err)
	// The error should mention the file path so the user knows which file
	// failed to load.
	assert.Contains(t, err.Error(), path,
		"ParseFile error should contain the file path")
}

// ---------------------------------------------------------------------------
// Robustness: Parse must not panic on adversarial inputs
// ---------------------------------------------------------------------------

func TestParse_DoesNotPanic(t *testing.T) {
	adversarial := []string{
		"",
		"null",
		"~",
		"---",
		"---\n---",
		"[]",
		"42",
		"true",
		`"just a string"`,
		"apiVersion: toolwright/v1", // missing everything else
		strings.Repeat("a: b\n", 10000),
	}

	for i, input := range adversarial {
		t.Run("", func(t *testing.T) {
			// We only care that it doesn't panic. Errors are expected.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Parse panicked on adversarial input %d: %v", i, r)
					}
				}()
				_, _ = Parse(strings.NewReader(input))
			}()
		})
	}
}
