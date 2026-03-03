package manifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// validToolkit returns a minimal but fully valid Toolkit. Every field is set to
// a value that should pass validation. Tests mutate a copy of this to introduce
// exactly one defect at a time, so any failure is attributable to that defect.
func validToolkit() *Toolkit {
	return &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: Metadata{
			Name:        "my-tool",
			Version:     "1.0.0",
			Description: "A valid description",
		},
		Tools: []Tool{
			{
				Name:        "do-thing",
				Description: "Does a thing",
				Entrypoint:  "./thing.sh",
			},
		},
	}
}

// findError returns the first ValidationError matching the given path prefix
// and rule, or nil if none found. This is used to assert that a specific error
// was reported without being order-dependent.
func findError(errs []ValidationError, path, rule string) *ValidationError {
	for i := range errs {
		if errs[i].Path == path && errs[i].Rule == rule {
			return &errs[i]
		}
	}
	return nil
}

// findErrorByRule returns the first ValidationError with the given rule.
func findErrorByRule(errs []ValidationError, rule string) *ValidationError {
	for i := range errs {
		if errs[i].Rule == rule {
			return &errs[i]
		}
	}
	return nil
}

// findErrorByPath returns the first ValidationError with the given path.
func findErrorByPath(errs []ValidationError, path string) *ValidationError {
	for i := range errs {
		if errs[i].Path == path {
			return &errs[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Baseline: Valid manifest produces zero errors
// ---------------------------------------------------------------------------

func TestValidate_ValidMinimalToolkit_NoErrors(t *testing.T) {
	tk := validToolkit()
	errs := Validate(tk)
	assert.Empty(t, errs, "A valid minimal toolkit should produce no validation errors, got: %v", errs)
}

func TestValidate_ValidFullToolkit_NoErrors(t *testing.T) {
	// The full manifest from parser_test.go should also pass validation.
	tk, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err)
	errs := Validate(tk)
	assert.Empty(t, errs, "The full example toolkit should produce no validation errors, got: %v", errs)
}

func TestValidate_ValidToolkitWithAuthNone_NoErrors(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{Type: "none"}
	errs := Validate(tk)
	assert.Empty(t, errs, "auth.type: none with no other fields should be valid, got: %v", errs)
}

func TestValidate_ValidTokenAuth_NoErrors(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:      "token",
		TokenEnv:  "MY_TOKEN",
		TokenFlag: "--token",
	}
	errs := Validate(tk)
	assert.Empty(t, errs, "Valid token auth with all required fields should pass, got: %v", errs)
}

func TestValidate_ValidOAuth2Auth_NoErrors(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:        "oauth2",
		ProviderURL: "https://auth.example.com",
		Scopes:      []string{"read", "write"},
	}
	errs := Validate(tk)
	assert.Empty(t, errs, "Valid oauth2 auth with all required fields should pass, got: %v", errs)
}

// ---------------------------------------------------------------------------
// AC-4: Validation catches invalid manifests — metadata.name
// ---------------------------------------------------------------------------

func TestValidate_MissingName(t *testing.T) {
	tk := validToolkit()
	tk.Metadata.Name = ""

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Missing metadata.name must produce an error")

	ve := findErrorByPath(errs, "metadata.name")
	require.NotNil(t, ve, "Error must be reported at path 'metadata.name', got paths: %v", errPaths(errs))
	assert.NotEmpty(t, ve.Message, "Error must have a human-readable message")
	assert.NotEmpty(t, ve.Rule, "Error must have a machine-readable rule")
}

func TestValidate_NameFormat(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		wantError bool
	}{
		// Valid names
		{name: "lowercase letters", inputName: "mytool", wantError: false},
		{name: "lowercase with hyphens", inputName: "my-tool", wantError: false},
		{name: "lowercase with numbers", inputName: "tool123", wantError: false},
		{name: "numbers and hyphens", inputName: "123-tool", wantError: false},
		{name: "single char", inputName: "a", wantError: false},
		{name: "all digits", inputName: "123", wantError: false},

		// Invalid names
		{name: "uppercase letters", inputName: "MyTool", wantError: true},
		{name: "all uppercase", inputName: "MYTOOL", wantError: true},
		{name: "mixed case", inputName: "myTool", wantError: true},
		{name: "spaces", inputName: "my tool", wantError: true},
		{name: "underscores", inputName: "my_tool", wantError: true},
		{name: "dots", inputName: "my.tool", wantError: true},
		{name: "at sign", inputName: "my@tool", wantError: true},
		{name: "exclamation", inputName: "tool!", wantError: true},
		{name: "slash", inputName: "my/tool", wantError: true},
		{name: "leading hyphen", inputName: "-tool", wantError: false},  // regex allows it
		{name: "trailing hyphen", inputName: "tool-", wantError: false}, // regex allows it
		{name: "unicode", inputName: "werkzeug-\u00fc", wantError: true},
		{name: "emoji", inputName: "tool-\U0001F680", wantError: true},
		{name: "tab character", inputName: "my\ttool", wantError: true},
		{name: "newline", inputName: "my\ntool", wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Metadata.Name = tc.inputName
			errs := Validate(tk)

			if tc.wantError {
				ve := findErrorByPath(errs, "metadata.name")
				require.NotNil(t, ve,
					"Name %q should be rejected but no error at metadata.name was found (all errors: %v)",
					tc.inputName, errs)
				assert.Equal(t, "name-format", ve.Rule,
					"Name format error should use rule 'name-format'")
			} else {
				ve := findError(errs, "metadata.name", "name-format")
				assert.Nil(t, ve,
					"Name %q should be valid but got error: %v", tc.inputName, ve)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-4: Validation catches invalid manifests — metadata.version (SemVer)
// ---------------------------------------------------------------------------

func TestValidate_Version(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantError bool
	}{
		// Valid SemVer
		{name: "standard semver", version: "1.0.0", wantError: false},
		{name: "zero version", version: "0.0.1", wantError: false},
		{name: "high numbers", version: "100.200.300", wantError: false},
		{name: "prerelease beta", version: "1.0.0-beta.1", wantError: false},
		{name: "prerelease alpha", version: "1.0.0-alpha", wantError: false},
		{name: "prerelease rc", version: "2.0.0-rc.1", wantError: false},
		{name: "build metadata", version: "0.0.1+build.123", wantError: false},
		{name: "prerelease and build", version: "1.0.0-beta.1+build.456", wantError: false},

		// Invalid versions
		{name: "two parts only", version: "1.0", wantError: true},
		{name: "one part only", version: "1", wantError: true},
		{name: "alphabetic", version: "abc", wantError: true},
		{name: "empty string", version: "", wantError: true},
		{name: "v prefix", version: "v1.0.0", wantError: true},
		{name: "four parts", version: "1.0.0.0", wantError: true},
		{name: "negative number", version: "-1.0.0", wantError: true},
		{name: "leading zero major", version: "01.0.0", wantError: true},
		{name: "leading zero minor", version: "1.01.0", wantError: true},
		{name: "leading zero patch", version: "1.0.01", wantError: true},
		{name: "spaces", version: "1. 0. 0", wantError: true},
		{name: "trailing text", version: "1.0.0abc", wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Metadata.Version = tc.version
			errs := Validate(tk)

			if tc.wantError {
				ve := findErrorByPath(errs, "metadata.version")
				require.NotNil(t, ve,
					"Version %q should be rejected but no error at metadata.version was found (all errors: %v)",
					tc.version, errs)
				assert.Equal(t, "semver", ve.Rule,
					"SemVer error should use rule 'semver'")
			} else {
				ve := findError(errs, "metadata.version", "semver")
				assert.Nil(t, ve,
					"Version %q should be valid but got error: %v", tc.version, ve)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-4: Validation catches invalid manifests — metadata.description
// ---------------------------------------------------------------------------

func TestValidate_DescriptionEmpty(t *testing.T) {
	tk := validToolkit()
	tk.Metadata.Description = ""

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Empty description must produce an error")

	ve := findErrorByPath(errs, "metadata.description")
	require.NotNil(t, ve, "Error must be reported at path 'metadata.description'")
}

func TestValidate_DescriptionExactly200Chars_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Metadata.Description = strings.Repeat("a", 200)

	errs := Validate(tk)
	ve := findErrorByPath(errs, "metadata.description")
	assert.Nil(t, ve,
		"Description of exactly 200 chars should be valid, got error: %v", ve)
}

func TestValidate_DescriptionOver200Chars(t *testing.T) {
	tk := validToolkit()
	tk.Metadata.Description = strings.Repeat("a", 201)

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Description over 200 chars must produce an error")

	ve := findErrorByPath(errs, "metadata.description")
	require.NotNil(t, ve, "Error must be reported at path 'metadata.description'")
	assert.NotEmpty(t, ve.Message)
}

func TestValidate_Description199Chars_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Metadata.Description = strings.Repeat("b", 199)

	errs := Validate(tk)
	ve := findErrorByPath(errs, "metadata.description")
	assert.Nil(t, ve,
		"Description of 199 chars should be valid")
}

func TestValidate_Description1Char_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Metadata.Description = "x"

	errs := Validate(tk)
	ve := findErrorByPath(errs, "metadata.description")
	assert.Nil(t, ve,
		"Description of 1 char should be valid")
}

// ---------------------------------------------------------------------------
// AC-4: Duplicate tool names
// ---------------------------------------------------------------------------

func TestValidate_DuplicateToolNames(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{Name: "list-pets", Description: "List pets", Entrypoint: "./list.sh"},
		{Name: "add-pet", Description: "Add pet", Entrypoint: "./add.sh"},
		{Name: "list-pets", Description: "List pets again", Entrypoint: "./list2.sh"},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Duplicate tool names must produce an error")

	found := findErrorByRule(errs, "unique-tool-name")
	require.NotNil(t, found, "Duplicate tool name error should use rule 'unique-tool-name', got rules: %v", errRules(errs))
	assert.Contains(t, found.Message, "list-pets",
		"Error message should mention the duplicate name")
}

func TestValidate_DuplicateToolNames_AllUnique_NoError(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{Name: "alpha", Description: "A", Entrypoint: "./a.sh"},
		{Name: "bravo", Description: "B", Entrypoint: "./b.sh"},
		{Name: "charlie", Description: "C", Entrypoint: "./c.sh"},
	}

	errs := Validate(tk)
	found := findErrorByRule(errs, "unique-tool-name")
	assert.Nil(t, found, "All unique tool names should not produce a unique-tool-name error")
}

func TestValidate_DuplicateToolNames_ThreeOfSame(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{Name: "dupe", Description: "First", Entrypoint: "./a.sh"},
		{Name: "dupe", Description: "Second", Entrypoint: "./b.sh"},
		{Name: "dupe", Description: "Third", Entrypoint: "./c.sh"},
	}

	errs := Validate(tk)
	found := findErrorByRule(errs, "unique-tool-name")
	require.NotNil(t, found, "Three tools with same name must produce error")
}

// ---------------------------------------------------------------------------
// AC-4: Duplicate arg names within a tool
// ---------------------------------------------------------------------------

func TestValidate_DuplicateArgNames(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Args: []Arg{
				{Name: "input", Type: "string", Description: "First"},
				{Name: "output", Type: "string", Description: "Second"},
				{Name: "input", Type: "string", Description: "Duplicate"},
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Duplicate arg names within a tool must produce an error")

	found := findErrorByRule(errs, "unique-arg-name")
	require.NotNil(t, found, "Duplicate arg name error should use rule 'unique-arg-name', got rules: %v", errRules(errs))
	assert.Contains(t, found.Path, "tools[0]",
		"Error path should reference the tool index")
}

func TestValidate_DuplicateArgNames_AcrossTools_NoProblem(t *testing.T) {
	// Same arg name in different tools is fine.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "tool-a",
			Description: "A",
			Entrypoint:  "./a.sh",
			Args: []Arg{
				{Name: "input", Type: "string", Description: "Input"},
			},
		},
		{
			Name:        "tool-b",
			Description: "B",
			Entrypoint:  "./b.sh",
			Args: []Arg{
				{Name: "input", Type: "string", Description: "Input"},
			},
		},
	}

	errs := Validate(tk)
	found := findErrorByRule(errs, "unique-arg-name")
	assert.Nil(t, found,
		"Same arg name across different tools should not trigger unique-arg-name error")
}

// ---------------------------------------------------------------------------
// AC-4: Duplicate flag names within a tool
// ---------------------------------------------------------------------------

func TestValidate_DuplicateFlagNames(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{Name: "verbose", Type: "bool", Description: "Verbose"},
				{Name: "output", Type: "string", Description: "Output"},
				{Name: "verbose", Type: "bool", Description: "Duplicate verbose"},
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Duplicate flag names within a tool must produce an error")

	found := findErrorByRule(errs, "unique-flag-name")
	require.NotNil(t, found, "Duplicate flag name error should use rule 'unique-flag-name', got rules: %v", errRules(errs))
	assert.Contains(t, found.Path, "tools[0]",
		"Error path should reference the tool index")
}

func TestValidate_DuplicateFlagNames_AcrossTools_NoProblem(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "tool-a",
			Description: "A",
			Entrypoint:  "./a.sh",
			Flags: []Flag{
				{Name: "verbose", Type: "bool", Description: "Verbose"},
			},
		},
		{
			Name:        "tool-b",
			Description: "B",
			Entrypoint:  "./b.sh",
			Flags: []Flag{
				{Name: "verbose", Type: "bool", Description: "Verbose"},
			},
		},
	}

	errs := Validate(tk)
	found := findErrorByRule(errs, "unique-flag-name")
	assert.Nil(t, found,
		"Same flag name across different tools should not trigger unique-flag-name error")
}

// ---------------------------------------------------------------------------
// AC-5: Validation catches invalid auth config — token auth
// ---------------------------------------------------------------------------

func TestValidate_TokenAuth_MissingTokenEnv(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:      "token",
		TokenEnv:  "", // missing
		TokenFlag: "--token",
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "token auth without token_env must error")

	ve := findErrorByPath(errs, "auth.token_env")
	require.NotNil(t, ve, "Error must be at path 'auth.token_env', got: %v", errPaths(errs))
}

func TestValidate_TokenAuth_MissingTokenFlag(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:      "token",
		TokenEnv:  "MY_TOKEN",
		TokenFlag: "", // missing
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "token auth without token_flag must error")

	ve := findErrorByPath(errs, "auth.token_flag")
	require.NotNil(t, ve, "Error must be at path 'auth.token_flag', got: %v", errPaths(errs))
}

func TestValidate_TokenAuth_MissingBothTokenFields(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type: "token",
		// Both token_env and token_flag missing
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "token auth without both required fields must error")

	// Both fields should produce separate errors (not fail-fast).
	veEnv := findErrorByPath(errs, "auth.token_env")
	veFlag := findErrorByPath(errs, "auth.token_flag")
	assert.NotNil(t, veEnv, "Should report missing token_env")
	assert.NotNil(t, veFlag, "Should report missing token_flag")
}

// ---------------------------------------------------------------------------
// AC-5: Validation catches invalid auth config — oauth2 auth
// ---------------------------------------------------------------------------

func TestValidate_OAuth2Auth_MissingProviderURL(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:   "oauth2",
		Scopes: []string{"read"},
		// provider_url missing
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "oauth2 auth without provider_url must error")

	ve := findErrorByPath(errs, "auth.provider_url")
	require.NotNil(t, ve, "Error must be at path 'auth.provider_url', got: %v", errPaths(errs))
}

func TestValidate_OAuth2Auth_MissingScopes(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:        "oauth2",
		ProviderURL: "https://auth.example.com",
		// scopes missing
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "oauth2 auth without scopes must error")

	ve := findErrorByPath(errs, "auth.scopes")
	require.NotNil(t, ve, "Error must be at path 'auth.scopes', got: %v", errPaths(errs))
}

func TestValidate_OAuth2Auth_EmptyScopes(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:        "oauth2",
		ProviderURL: "https://auth.example.com",
		Scopes:      []string{}, // empty, not nil
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "oauth2 auth with empty scopes must error")

	ve := findErrorByPath(errs, "auth.scopes")
	require.NotNil(t, ve, "Error must be at path 'auth.scopes'")
}

func TestValidate_OAuth2Auth_HTTPProviderURL(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:        "oauth2",
		ProviderURL: "http://auth.example.com", // HTTP, not HTTPS
		Scopes:      []string{"read"},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "provider_url with HTTP must error")

	ve := findErrorByPath(errs, "auth.provider_url")
	require.NotNil(t, ve, "Error must be at path 'auth.provider_url'")
}

func TestValidate_OAuth2Auth_HTTPSProviderURL_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type:        "oauth2",
		ProviderURL: "https://auth.example.com",
		Scopes:      []string{"read"},
	}

	errs := Validate(tk)
	ve := findErrorByPath(errs, "auth.provider_url")
	assert.Nil(t, ve, "HTTPS provider_url should be valid")
}

func TestValidate_OAuth2Auth_MissingBothProviderAndScopes(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{
		Type: "oauth2",
		// Both provider_url and scopes missing
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "oauth2 without required fields must error")

	// Both should be reported (non-fail-fast).
	veURL := findErrorByPath(errs, "auth.provider_url")
	veScopes := findErrorByPath(errs, "auth.scopes")
	assert.NotNil(t, veURL, "Should report missing provider_url")
	assert.NotNil(t, veScopes, "Should report missing scopes")
}

// ---------------------------------------------------------------------------
// AC-5: auth.type: none passes
// ---------------------------------------------------------------------------

func TestValidate_AuthTypeNone_Passes(t *testing.T) {
	tk := validToolkit()
	tk.Auth = &Auth{Type: "none"}

	errs := Validate(tk)
	assert.Empty(t, errs, "auth.type: none should produce no errors, got: %v", errs)
}

// ---------------------------------------------------------------------------
// AC-5: Unknown auth type
// ---------------------------------------------------------------------------

func TestValidate_UnknownAuthType(t *testing.T) {
	tests := []struct {
		name     string
		authType string
	}{
		{name: "basic", authType: "basic"},
		{name: "apikey", authType: "apikey"},
		{name: "digest", authType: "digest"},
		{name: "empty string", authType: ""},
		{name: "random string", authType: "foobar"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Auth = &Auth{Type: tc.authType}

			errs := Validate(tk)
			require.NotEmpty(t, errs,
				"Unknown auth type %q must produce an error", tc.authType)

			ve := findErrorByPath(errs, "auth.type")
			require.NotNil(t, ve,
				"Error for unknown auth type must be at path 'auth.type', got: %v", errPaths(errs))
		})
	}
}

// ---------------------------------------------------------------------------
// AC-5: Toolkit-level auth is validated (not just tool-level)
// ---------------------------------------------------------------------------

func TestValidate_ToolkitLevelAuth_IsValidated(t *testing.T) {
	// Toolkit-level auth with invalid config must be caught.
	tk := validToolkit()
	tk.Auth = &Auth{
		Type: "token",
		// Missing token_env and token_flag at toolkit level
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Invalid toolkit-level auth must be caught")

	veEnv := findErrorByPath(errs, "auth.token_env")
	veFlag := findErrorByPath(errs, "auth.token_flag")
	assert.NotNil(t, veEnv, "Toolkit auth.token_env should be validated")
	assert.NotNil(t, veFlag, "Toolkit auth.token_flag should be validated")
}

func TestValidate_ToolLevelAuth_IsValidated(t *testing.T) {
	// Tool-level auth with invalid config must also be caught.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Auth: &Auth{
				Type: "token",
				// Missing token_env and token_flag
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Invalid tool-level auth must be caught")

	// Path should reference the tool index
	veEnv := findErrorByPath(errs, "tools[0].auth.token_env")
	veFlag := findErrorByPath(errs, "tools[0].auth.token_flag")
	assert.NotNil(t, veEnv,
		"Tool-level auth.token_env should be validated at path tools[0].auth.token_env, got: %v", errPaths(errs))
	assert.NotNil(t, veFlag,
		"Tool-level auth.token_flag should be validated at path tools[0].auth.token_flag, got: %v", errPaths(errs))
}

func TestValidate_ToolLevelAuth_SecondTool(t *testing.T) {
	// Verify the tool index is correct for the second tool.
	tk := validToolkit()
	tk.Tools = []Tool{
		{Name: "good-tool", Description: "Good", Entrypoint: "./good.sh"},
		{
			Name:        "bad-tool",
			Description: "Bad",
			Entrypoint:  "./bad.sh",
			Auth: &Auth{
				Type:        "oauth2",
				ProviderURL: "http://insecure.example.com", // HTTP
				Scopes:      []string{"read"},
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs)

	ve := findErrorByPath(errs, "tools[1].auth.provider_url")
	require.NotNil(t, ve,
		"Error for tool[1]'s auth should be at 'tools[1].auth.provider_url', got: %v", errPaths(errs))
}

// ---------------------------------------------------------------------------
// AC-6: Validation catches type mismatches — flag default vs type
// ---------------------------------------------------------------------------

func TestValidate_FlagTypeMismatch(t *testing.T) {
	tests := []struct {
		name      string
		flagType  string
		dflt      any
		wantError bool
	}{
		// Mismatches
		{name: "int type string default", flagType: "int", dflt: "abc", wantError: true},
		{name: "int type bool default", flagType: "int", dflt: true, wantError: true},
		{name: "bool type string default", flagType: "bool", dflt: "yes", wantError: true},
		{name: "bool type int default", flagType: "bool", dflt: 42, wantError: true},
		{name: "float type string default", flagType: "float", dflt: "abc", wantError: true},

		// Valid matches
		{name: "string type string default", flagType: "string", dflt: "hello", wantError: false},
		{name: "int type int default", flagType: "int", dflt: 42, wantError: false},
		{name: "int type zero default", flagType: "int", dflt: 0, wantError: false},
		{name: "bool type true default", flagType: "bool", dflt: true, wantError: false},
		{name: "bool type false default", flagType: "bool", dflt: false, wantError: false},
		{name: "float type float default", flagType: "float", dflt: 3.14, wantError: false},
		{name: "float type int default", flagType: "float", dflt: 3, wantError: false}, // int is acceptable for float
		{name: "no default", flagType: "int", dflt: nil, wantError: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{
				{
					Name:        "my-tool",
					Description: "A tool",
					Entrypoint:  "./tool.sh",
					Flags: []Flag{
						{
							Name:        "myflag",
							Type:        tc.flagType,
							Default:     tc.dflt,
							Description: "A flag",
						},
					},
				},
			}

			errs := Validate(tk)
			flagPath := "tools[0].flags[0].default"

			if tc.wantError {
				ve := findErrorByPath(errs, flagPath)
				require.NotNil(t, ve,
					"Flag type %q with default %v (%T) should be rejected at path %s, got: %v",
					tc.flagType, tc.dflt, tc.dflt, flagPath, errPaths(errs))
				assert.Equal(t, "type-mismatch", ve.Rule,
					"Type mismatch error should use rule 'type-mismatch'")
			} else {
				ve := findError(errs, flagPath, "type-mismatch")
				assert.Nil(t, ve,
					"Flag type %q with default %v should be valid, got error: %v",
					tc.flagType, tc.dflt, ve)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-6: Validation catches type mismatches — bool enum
// ---------------------------------------------------------------------------

func TestValidate_BoolFlagWithEnum(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "toggle",
					Type:        "bool",
					Enum:        []string{"yes", "no"},
					Description: "A toggle",
				},
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Bool flag with enum must produce an error")

	// The error should be on the enum or the flag.
	var found bool
	for _, ve := range errs {
		if strings.Contains(ve.Path, "tools[0].flags[0]") {
			found = true
			break
		}
	}
	assert.True(t, found,
		"Error should reference tools[0].flags[0], got: %v", errPaths(errs))
}

func TestValidate_IntFlagWithIntEnum_Valid(t *testing.T) {
	// int enum with int values should be valid. Note: YAML parses these
	// as strings in the enum field ([]string), so the values "1", "2", "3"
	// are strings that represent ints.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "level",
					Type:        "int",
					Enum:        []string{"1", "2", "3"},
					Description: "Level",
				},
			},
		},
	}

	errs := Validate(tk)
	// Should not have a type-mismatch or bool-enum error for int enum with int-parseable values.
	for _, ve := range errs {
		if strings.Contains(ve.Path, "tools[0].flags[0]") && ve.Rule == "type-mismatch" {
			t.Errorf("int flag with int enum values should be valid, got error: %v", ve)
		}
	}
}

func TestValidate_StringFlagWithEnum_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "format",
					Type:        "string",
					Enum:        []string{"json", "csv", "table"},
					Description: "Output format",
				},
			},
		},
	}

	errs := Validate(tk)
	for _, ve := range errs {
		if strings.Contains(ve.Path, "tools[0].flags[0]") {
			t.Errorf("string flag with string enum should be valid, got error: %v", ve)
		}
	}
}

// ---------------------------------------------------------------------------
// AC-8: ValidationError structure
// ---------------------------------------------------------------------------

func TestValidate_ErrorHasPathMessageRule(t *testing.T) {
	// Introduce a known invalid field and check the error structure.
	tk := validToolkit()
	tk.Metadata.Name = "INVALID NAME"

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Invalid name should produce errors")

	for _, ve := range errs {
		assert.NotEmpty(t, ve.Path,
			"Every ValidationError must have a non-empty Path")
		assert.NotEmpty(t, ve.Message,
			"Every ValidationError must have a non-empty Message")
		assert.NotEmpty(t, ve.Rule,
			"Every ValidationError must have a non-empty Rule")
	}
}

func TestValidate_PathUsesBracketNotation(t *testing.T) {
	// Create a manifest with a duplicate arg in the second tool's third arg.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "tool-a",
			Description: "A",
			Entrypoint:  "./a.sh",
		},
		{
			Name:        "tool-b",
			Description: "B",
			Entrypoint:  "./b.sh",
			Args: []Arg{
				{Name: "first", Type: "string", Description: "First"},
				{Name: "second", Type: "string", Description: "Second"},
				{Name: "first", Type: "string", Description: "Duplicate of first"},
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Duplicate arg names should produce errors")

	// The path should use bracket notation with the correct tool index.
	found := findErrorByRule(errs, "unique-arg-name")
	require.NotNil(t, found, "Should have unique-arg-name error")
	assert.Contains(t, found.Path, "tools[1]",
		"Path should reference tool index 1 using bracket notation")
}

func TestValidate_RuleIsMachineReadable(t *testing.T) {
	// Rules should be kebab-case identifiers, not sentences or codes with spaces.
	tk := validToolkit()
	tk.Metadata.Name = ""                              // triggers required + name-format
	tk.Metadata.Version = "bad"                        // triggers semver
	tk.Metadata.Description = strings.Repeat("x", 201) // triggers description length
	tk.Tools = []Tool{
		{Name: "a", Description: "A", Entrypoint: "./a.sh"},
		{Name: "a", Description: "B", Entrypoint: "./b.sh"}, // triggers unique-tool-name
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Multiple invalid fields should produce errors")

	knownRules := map[string]bool{
		"name-format":      false,
		"semver":           false,
		"unique-tool-name": false,
	}

	for _, ve := range errs {
		// Check no whitespace in rules.
		assert.False(t, strings.ContainsAny(ve.Rule, " \t\n"),
			"Rule %q should not contain whitespace", ve.Rule)

		if _, ok := knownRules[ve.Rule]; ok {
			knownRules[ve.Rule] = true
		}
	}

	// At least the semver and unique-tool-name rules should be present.
	assert.True(t, knownRules["semver"],
		"Expected rule 'semver' to be present in errors")
	assert.True(t, knownRules["unique-tool-name"],
		"Expected rule 'unique-tool-name' to be present in errors")
}

// ---------------------------------------------------------------------------
// AC-8: Multiple errors returned in single call (not fail-fast)
// ---------------------------------------------------------------------------

func TestValidate_MultipleErrors_NotFailFast(t *testing.T) {
	// Introduce multiple independent errors. All should be reported.
	tk := &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: Metadata{
			Name:        "INVALID",                // bad name
			Version:     "not-a-version",          // bad version
			Description: strings.Repeat("z", 300), // too long
		},
		Tools: []Tool{
			{Name: "dup", Description: "A", Entrypoint: "./a.sh"},
			{Name: "dup", Description: "B", Entrypoint: "./b.sh"},
		},
		Auth: &Auth{
			Type: "token",
			// Missing token_env and token_flag
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Multiple issues must produce errors")

	// We expect at least 5 distinct errors:
	//   1. name-format
	//   2. semver
	//   3. description too long
	//   4. unique-tool-name
	//   5. auth.token_env missing
	//   6. auth.token_flag missing
	assert.GreaterOrEqual(t, len(errs), 5,
		"Should report at least 5 errors for this manifest, got %d: %v", len(errs), errs)

	// Verify specific independent errors are all present.
	assert.NotNil(t, findErrorByPath(errs, "metadata.name"),
		"Should report metadata.name error")
	assert.NotNil(t, findErrorByPath(errs, "metadata.version"),
		"Should report metadata.version error")
	assert.NotNil(t, findErrorByPath(errs, "metadata.description"),
		"Should report metadata.description error")
	assert.NotNil(t, findErrorByRule(errs, "unique-tool-name"),
		"Should report unique-tool-name error")
	assert.NotNil(t, findErrorByPath(errs, "auth.token_env"),
		"Should report auth.token_env error")
}

func TestValidate_MultipleErrorsOnDifferentTools(t *testing.T) {
	// Errors on different tools should all be reported.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "tool-a",
			Description: "A",
			Entrypoint:  "./a.sh",
			Flags: []Flag{
				{Name: "bad", Type: "int", Default: "not-an-int", Description: "Bad"},
			},
		},
		{
			Name:        "tool-b",
			Description: "B",
			Entrypoint:  "./b.sh",
			Flags: []Flag{
				{Name: "worse", Type: "bool", Default: "maybe", Description: "Worse"},
			},
		},
	}

	errs := Validate(tk)
	require.NotEmpty(t, errs)

	tool0Err := findErrorByPath(errs, "tools[0].flags[0].default")
	tool1Err := findErrorByPath(errs, "tools[1].flags[0].default")
	assert.NotNil(t, tool0Err, "Should report error for tool[0] flag default")
	assert.NotNil(t, tool1Err, "Should report error for tool[1] flag default")
}

// ---------------------------------------------------------------------------
// AC-6 additional: float-specific tests
// ---------------------------------------------------------------------------

func TestValidate_FloatFlagDefaultFloat_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{Name: "rate", Type: "float", Default: 3.14, Description: "Rate"},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	assert.Nil(t, ve, "float flag with float default should be valid")
}

// ---------------------------------------------------------------------------
// Edge: nil toolkit pointer
// ---------------------------------------------------------------------------

func TestValidate_NilToolkit(t *testing.T) {
	// Validate should not panic on nil input.
	assert.NotPanics(t, func() {
		_ = Validate(nil)
	}, "Validate(nil) should not panic")
}

// ---------------------------------------------------------------------------
// Edge: toolkit with no tools
// ---------------------------------------------------------------------------

func TestValidate_NoTools(t *testing.T) {
	tk := validToolkit()
	tk.Tools = nil

	// Whether this is an error or valid depends on spec, but it should not panic.
	assert.NotPanics(t, func() {
		_ = Validate(tk)
	}, "Validate with no tools should not panic")
}

func TestValidate_EmptyToolsSlice(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{}

	// Similar to nil, should not panic.
	assert.NotPanics(t, func() {
		_ = Validate(tk)
	}, "Validate with empty tools slice should not panic")
}

// ---------------------------------------------------------------------------
// Edge: flag at second index
// ---------------------------------------------------------------------------

func TestValidate_FlagAtSecondIndex_PathCorrect(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{Name: "good", Type: "string", Default: "ok", Description: "Good"},
				{Name: "bad", Type: "int", Default: "not-int", Description: "Bad"},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByPath(errs, "tools[0].flags[1].default")
	require.NotNil(t, ve,
		"Error for second flag should be at 'tools[0].flags[1].default', got: %v", errPaths(errs))
	assert.Equal(t, "type-mismatch", ve.Rule)
}

// ---------------------------------------------------------------------------
// Edge: auth on nil (no auth set)
// ---------------------------------------------------------------------------

func TestValidate_NilAuth_Valid(t *testing.T) {
	tk := validToolkit()
	tk.Auth = nil // no auth is valid

	errs := Validate(tk)
	// Should have no auth-related errors.
	for _, ve := range errs {
		if strings.HasPrefix(ve.Path, "auth") {
			t.Errorf("No auth set should not produce auth errors, got: %v", ve)
		}
	}
}

// ---------------------------------------------------------------------------
// Adversarial: hardcoded return value detection
// ---------------------------------------------------------------------------

func TestValidate_DifferentValidToolkits_AllPass(t *testing.T) {
	// A hardcoded implementation that returns nil for a specific toolkit
	// would fail on these varied valid inputs.
	toolkits := []*Toolkit{
		{
			APIVersion: "toolwright/v1",
			Kind:       "Toolkit",
			Metadata:   Metadata{Name: "alpha", Version: "0.1.0", Description: "Alpha tool"},
			Tools:      []Tool{{Name: "run", Description: "Run", Entrypoint: "./run.sh"}},
		},
		{
			APIVersion: "toolwright/v1",
			Kind:       "Toolkit",
			Metadata:   Metadata{Name: "beta-123", Version: "99.99.99", Description: "Beta tool"},
			Tools:      []Tool{{Name: "exec", Description: "Exec", Entrypoint: "./exec.sh"}},
			Auth:       &Auth{Type: "none"},
		},
		{
			APIVersion: "toolwright/v1",
			Kind:       "Toolkit",
			Metadata:   Metadata{Name: "z", Version: "0.0.1-rc.1+build.999", Description: "Z"},
			Tools: []Tool{
				{Name: "a", Description: "A", Entrypoint: "./a.sh"},
				{Name: "b", Description: "B", Entrypoint: "./b.sh"},
			},
			Auth: &Auth{
				Type:        "oauth2",
				ProviderURL: "https://auth.z.com",
				Scopes:      []string{"all"},
			},
		},
	}

	for i, tk := range toolkits {
		errs := Validate(tk)
		assert.Empty(t, errs,
			"Valid toolkit %d (%s) should produce no errors, got: %v", i, tk.Metadata.Name, errs)
	}
}

func TestValidate_DifferentInvalidNames_AllFail(t *testing.T) {
	// Prevent hardcoded checks for specific invalid names.
	invalidNames := []string{"AB", "Hello", "foo bar", "a_b", "x.y", "CamelCase", "SHOUT"}

	for _, name := range invalidNames {
		tk := validToolkit()
		tk.Metadata.Name = name

		errs := Validate(tk)
		ve := findErrorByPath(errs, "metadata.name")
		assert.NotNil(t, ve,
			"Name %q should be rejected but was not", name)
	}
}

func TestValidate_DifferentInvalidVersions_AllFail(t *testing.T) {
	// Prevent hardcoded checks for specific invalid versions.
	invalidVersions := []string{"x", "1.0", "v2.0.0", "1.2.3.4", "abc.def.ghi"}

	for _, ver := range invalidVersions {
		tk := validToolkit()
		tk.Metadata.Version = ver

		errs := Validate(tk)
		ve := findErrorByPath(errs, "metadata.version")
		assert.NotNil(t, ve,
			"Version %q should be rejected but was not", ver)
	}
}

// ---------------------------------------------------------------------------
// Ensure the return type is correct (compile-time check + runtime)
// ---------------------------------------------------------------------------

func TestValidate_ReturnType(t *testing.T) {
	tk := validToolkit()
	errs := Validate(tk)
	// This is also a compile-time check that the return type is []ValidationError.
	_ = errs
}

func TestValidationError_FieldsAccessible(t *testing.T) {
	// Compile-time and runtime check that ValidationError has the right fields.
	ve := ValidationError{
		Path:    "metadata.name",
		Message: "name is required",
		Rule:    "required",
	}
	assert.Equal(t, "metadata.name", ve.Path)
	assert.Equal(t, "name is required", ve.Message)
	assert.Equal(t, "required", ve.Rule)
}

// ---------------------------------------------------------------------------
// Helper: extract paths/rules from error slices for diagnostic messages
// ---------------------------------------------------------------------------

func errPaths(errs []ValidationError) []string {
	paths := make([]string, len(errs))
	for i, ve := range errs {
		paths[i] = ve.Path
	}
	return paths
}

func errRules(errs []ValidationError) []string {
	rules := make([]string, len(errs))
	for i, ve := range errs {
		rules[i] = ve.Rule
	}
	return rules
}
