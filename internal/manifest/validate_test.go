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
// Task 2: Unknown flag types are rejected (rule: "unknown-flag-type")
// ---------------------------------------------------------------------------

func TestValidateFlag_UnknownFlagType(t *testing.T) {
	tests := []struct {
		name      string
		flagType  string
		wantError bool
	}{
		// Known scalar types
		{name: "string is known", flagType: "string", wantError: false},
		{name: "int is known", flagType: "int", wantError: false},
		{name: "float is known", flagType: "float", wantError: false},
		{name: "bool is known", flagType: "bool", wantError: false},
		// Known array types
		{name: "string[] is known", flagType: "string[]", wantError: false},
		{name: "int[] is known", flagType: "int[]", wantError: false},
		{name: "float[] is known", flagType: "float[]", wantError: false},
		{name: "bool[] is known", flagType: "bool[]", wantError: false},
		// Unknown types
		{name: "unknown scalar", flagType: "bytes", wantError: true},
		{name: "unknown array", flagType: "bytes[]", wantError: true},
		{name: "empty string", flagType: "", wantError: true},
		{name: "number is not a type", flagType: "number", wantError: true},
		{name: "array is not a type", flagType: "array", wantError: true},
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
						{Name: "myflag", Type: tc.flagType, Description: "A flag"},
					},
				},
			}

			errs := Validate(tk)

			if tc.wantError {
				ve := findErrorByRule(errs, "unknown-flag-type")
				require.NotNil(t, ve,
					"Flag type %q should produce unknown-flag-type error, got rules: %v",
					tc.flagType, errRules(errs))
				assert.Contains(t, ve.Path, "tools[0].flags[0]",
					"Error path should reference the flag")
			} else {
				ve := findErrorByRule(errs, "unknown-flag-type")
				assert.Nil(t, ve,
					"Flag type %q should be valid, got error: %v", tc.flagType, ve)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Task 2: Array default type checking
// ---------------------------------------------------------------------------

func TestCheckDefaultType_ArrayTypes(t *testing.T) {
	tests := []struct {
		name      string
		flagType  string
		value     any
		wantError bool
	}{
		// string[] valid cases
		{name: "string[] with string slice", flagType: "string[]", value: []interface{}{"a", "b"}, wantError: false},
		{name: "string[] with empty slice", flagType: "string[]", value: []interface{}{}, wantError: false},
		// string[] invalid cases
		{name: "string[] with scalar string", flagType: "string[]", value: "scalar", wantError: true},
		{name: "string[] with int element", flagType: "string[]", value: []interface{}{1, 2}, wantError: true},

		// int[] valid cases
		{name: "int[] with int slice", flagType: "int[]", value: []interface{}{1, 2, 3}, wantError: false},
		{name: "int[] with empty slice", flagType: "int[]", value: []interface{}{}, wantError: false},
		// int[] invalid cases
		{name: "int[] with string element", flagType: "int[]", value: []interface{}{"a"}, wantError: true},
		{name: "int[] with scalar", flagType: "int[]", value: 42, wantError: true},

		// float[] valid cases
		{name: "float[] with float slice", flagType: "float[]", value: []interface{}{1.1, 2.2}, wantError: false},
		{name: "float[] with int elements (acceptable)", flagType: "float[]", value: []interface{}{1, 2}, wantError: false},
		{name: "float[] with empty slice", flagType: "float[]", value: []interface{}{}, wantError: false},
		// float[] invalid cases
		{name: "float[] with string element", flagType: "float[]", value: []interface{}{"abc"}, wantError: true},

		// bool[] valid cases
		{name: "bool[] with bool slice", flagType: "bool[]", value: []interface{}{true, false}, wantError: false},
		{name: "bool[] with empty slice", flagType: "bool[]", value: []interface{}{}, wantError: false},
		// bool[] invalid cases
		{name: "bool[] with string element", flagType: "bool[]", value: []interface{}{"yes"}, wantError: true},
		{name: "bool[] with scalar", flagType: "bool[]", value: true, wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDefaultType(tc.flagType, tc.value)
			if tc.wantError {
				require.Error(t, err,
					"checkDefaultType(%q, %v) should return an error", tc.flagType, tc.value)
			} else {
				require.NoError(t, err,
					"checkDefaultType(%q, %v) should not return an error", tc.flagType, tc.value)
			}
		})
	}
}

func TestValidate_ArrayFlagDefault_Integration(t *testing.T) {
	// Verify array default type errors surface as ValidationErrors via Validate.
	tests := []struct {
		name      string
		flagType  string
		dflt      any
		wantError bool
	}{
		{name: "string[] valid default", flagType: "string[]", dflt: []interface{}{"a", "b"}, wantError: false},
		{name: "string[] scalar default is invalid", flagType: "string[]", dflt: "scalar", wantError: true},
		{name: "int[] with string element", flagType: "int[]", dflt: []interface{}{"not-an-int"}, wantError: true},
		{name: "int[] empty slice is valid", flagType: "int[]", dflt: []interface{}{}, wantError: false},
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
						{Name: "myflag", Type: tc.flagType, Default: tc.dflt, Description: "A flag"},
					},
				},
			}

			errs := Validate(tk)
			flagPath := "tools[0].flags[0].default"

			if tc.wantError {
				ve := findErrorByPath(errs, flagPath)
				require.NotNil(t, ve,
					"Expected error at %s for type %q with default %v, got: %v",
					flagPath, tc.flagType, tc.dflt, errPaths(errs))
				assert.Equal(t, "type-mismatch", ve.Rule)
			} else {
				ve := findError(errs, flagPath, "type-mismatch")
				assert.Nil(t, ve,
					"Expected no type-mismatch at %s for type %q with default %v",
					flagPath, tc.flagType, tc.dflt)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Task 2: Array enum type checking
// ---------------------------------------------------------------------------

func TestCheckEnumType_ArrayTypes(t *testing.T) {
	tests := []struct {
		name      string
		flagType  string
		enum      []string
		wantError bool
	}{
		// string[] dispatches to string scalar — all string values valid
		{name: "string[] enum valid", flagType: "string[]", enum: []string{"a", "b"}, wantError: false},

		// int[] dispatches to int scalar — values must be parseable as int
		{name: "int[] enum valid ints", flagType: "int[]", enum: []string{"1", "2", "3"}, wantError: false},
		{name: "int[] enum with non-int", flagType: "int[]", enum: []string{"1", "not-int"}, wantError: true},

		// float[] dispatches to float scalar
		{name: "float[] enum valid floats", flagType: "float[]", enum: []string{"1.1", "2.2"}, wantError: false},
		{name: "float[] enum with non-float", flagType: "float[]", enum: []string{"1.0", "abc"}, wantError: true},

		// bool[] dispatches to bool scalar
		{name: "bool[] enum valid bools", flagType: "bool[]", enum: []string{"true", "false"}, wantError: false},
		{name: "bool[] enum with non-bool", flagType: "bool[]", enum: []string{"yes", "no"}, wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkEnumType(tc.flagType, tc.enum)
			if tc.wantError {
				require.Error(t, err,
					"checkEnumType(%q, %v) should return an error", tc.flagType, tc.enum)
			} else {
				require.NoError(t, err,
					"checkEnumType(%q, %v) should not return an error", tc.flagType, tc.enum)
			}
		})
	}
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

// findWarningByRule returns the first ValidationError with the given rule and
// severity "warning", or nil if none found.
func findWarningByRule(errs []ValidationError, rule string) *ValidationError {
	for i := range errs {
		if errs[i].Rule == rule && errs[i].Severity == SeverityWarning {
			return &errs[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// AC3: itemSchema compilation — valid and invalid JSON Schema on object types
// ---------------------------------------------------------------------------

func TestValidateFlag_ItemSchema_ValidSchema_ObjectType(t *testing.T) {
	tests := []struct {
		name       string
		flagType   string
		itemSchema map[string]any
	}{
		{
			name:     "object with simple properties schema",
			flagType: "object",
			itemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
		{
			name:     "object with required fields schema",
			flagType: "object",
			itemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
				},
				"required": []any{"name"},
			},
		},
		{
			name:     "object with minimal schema",
			flagType: "object",
			itemSchema: map[string]any{
				"type": "object",
			},
		},
		{
			name:     "object[] with valid schema",
			flagType: "object[]",
			itemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   map[string]any{"type": "integer"},
					"name": map[string]any{"type": "string"},
				},
			},
		},
		{
			name:     "object[] with nested object schema",
			flagType: "object[]",
			itemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"street": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
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
							Name:        "config",
							Type:        tc.flagType,
							Description: "Config flag",
							ItemSchema:  tc.itemSchema,
						},
					},
				},
			}

			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-item-schema")
			assert.Nil(t, ve,
				"Flag type %q with valid itemSchema should not produce invalid-item-schema error, got: %v",
				tc.flagType, errs)

			// Also ensure no item-schema-not-allowed error (object types allow itemSchema)
			ve2 := findErrorByRule(errs, "item-schema-not-allowed")
			assert.Nil(t, ve2,
				"Flag type %q should allow itemSchema, got: %v",
				tc.flagType, errs)
		})
	}
}

func TestValidateFlag_ItemSchema_InvalidSchema(t *testing.T) {
	tests := []struct {
		name       string
		flagType   string
		itemSchema map[string]any
	}{
		{
			name:     "object with invalid type value",
			flagType: "object",
			itemSchema: map[string]any{
				"type": "nonexistent",
			},
		},
		{
			name:     "object with invalid property type",
			flagType: "object",
			itemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "nonexistent"},
				},
			},
		},
		{
			name:     "object[] with invalid type value",
			flagType: "object[]",
			itemSchema: map[string]any{
				"type": "nonexistent",
			},
		},
		{
			name:     "object[] with invalid property type",
			flagType: "object[]",
			itemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"count": map[string]any{"type": "imaginary"},
				},
			},
		},
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
							Name:        "config",
							Type:        tc.flagType,
							Description: "Config flag",
							ItemSchema:  tc.itemSchema,
						},
					},
				},
			}

			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-item-schema")
			require.NotNil(t, ve,
				"Flag type %q with invalid itemSchema %v should produce invalid-item-schema error, got rules: %v",
				tc.flagType, tc.itemSchema, errRules(errs))
			assert.Contains(t, ve.Path, "tools[0].flags[0]",
				"Error path should reference the flag")
			assert.NotEmpty(t, ve.Message,
				"Error should have a human-readable message")
			// Invalid schema errors must be severity "error", not "warning"
			assert.NotEqual(t, SeverityWarning, ve.Severity,
				"Invalid item schema should be an error, not a warning")
		})
	}
}

// ---------------------------------------------------------------------------
// AC4: itemSchema not allowed on non-object types
// ---------------------------------------------------------------------------

func TestValidateFlag_ItemSchema_NotAllowedOnNonObjectTypes(t *testing.T) {
	nonObjectTypes := []struct {
		name     string
		flagType string
	}{
		{name: "string", flagType: "string"},
		{name: "int", flagType: "int"},
		{name: "float", flagType: "float"},
		{name: "bool", flagType: "bool"},
		{name: "string[]", flagType: "string[]"},
		{name: "int[]", flagType: "int[]"},
		{name: "float[]", flagType: "float[]"},
		{name: "bool[]", flagType: "bool[]"},
	}

	someSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{"type": "string"},
		},
	}

	for _, tc := range nonObjectTypes {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{
				{
					Name:        "my-tool",
					Description: "A tool",
					Entrypoint:  "./tool.sh",
					Flags: []Flag{
						{
							Name:        "bad-flag",
							Type:        tc.flagType,
							Description: "Should not have itemSchema",
							ItemSchema:  someSchema,
						},
					},
				},
			}

			errs := Validate(tk)
			ve := findErrorByRule(errs, "item-schema-not-allowed")
			require.NotNil(t, ve,
				"Flag type %q with itemSchema should produce item-schema-not-allowed error, got rules: %v",
				tc.flagType, errRules(errs))
			assert.Contains(t, ve.Path, "tools[0].flags[0]",
				"Error path should reference the flag")
			assert.NotEmpty(t, ve.Message,
				"Error should have a human-readable message explaining itemSchema is not allowed for type %q",
				tc.flagType)
		})
	}
}

func TestValidateFlag_ItemSchema_AllowedOnObjectTypes(t *testing.T) {
	// Verify that object and object[] do NOT produce "item-schema-not-allowed"
	objectTypes := []struct {
		name     string
		flagType string
	}{
		{name: "object", flagType: "object"},
		{name: "object[]", flagType: "object[]"},
	}

	someSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{"type": "string"},
		},
	}

	for _, tc := range objectTypes {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{
				{
					Name:        "my-tool",
					Description: "A tool",
					Entrypoint:  "./tool.sh",
					Flags: []Flag{
						{
							Name:        "config",
							Type:        tc.flagType,
							Description: "Object flag with schema",
							ItemSchema:  someSchema,
						},
					},
				},
			}

			errs := Validate(tk)
			ve := findErrorByRule(errs, "item-schema-not-allowed")
			assert.Nil(t, ve,
				"Flag type %q should allow itemSchema, got error: %v", tc.flagType, ve)
		})
	}
}

func TestValidateFlag_ItemSchema_NilOnNonObject_NoError(t *testing.T) {
	// Non-object types with NO itemSchema should NOT trigger item-schema-not-allowed.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{Name: "name", Type: "string", Description: "A string flag"},
				{Name: "count", Type: "int", Description: "An int flag"},
				{Name: "tags", Type: "string[]", Description: "A string array flag"},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "item-schema-not-allowed")
	assert.Nil(t, ve,
		"Non-object types without itemSchema should produce no item-schema-not-allowed error")
}

// ---------------------------------------------------------------------------
// AC5: missing itemSchema warning for object types
// ---------------------------------------------------------------------------

func TestValidateFlag_MissingItemSchema_ObjectType_Warning(t *testing.T) {
	tests := []struct {
		name     string
		flagType string
	}{
		{name: "object without itemSchema", flagType: "object"},
		{name: "object[] without itemSchema", flagType: "object[]"},
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
							Name:        "data",
							Type:        tc.flagType,
							Description: "Object flag without schema",
							// ItemSchema deliberately omitted
						},
					},
				},
			}

			errs := Validate(tk)
			ve := findWarningByRule(errs, "missing-item-schema")
			require.NotNil(t, ve,
				"Flag type %q without itemSchema should produce a warning with rule missing-item-schema, got: %v",
				tc.flagType, errs)
			assert.Equal(t, SeverityWarning, ve.Severity,
				"missing-item-schema should be a warning, not an error")
			assert.Contains(t, ve.Path, "tools[0].flags[0]",
				"Warning path should reference the flag")
			assert.NotEmpty(t, ve.Message,
				"Warning should have a human-readable message")
		})
	}
}

func TestValidateFlag_MissingItemSchema_NonObjectType_NoWarning(t *testing.T) {
	// Non-object types without itemSchema should NOT produce a missing-item-schema warning.
	nonObjectTypes := []string{"string", "int", "float", "bool", "string[]", "int[]", "float[]", "bool[]"}

	for _, flagType := range nonObjectTypes {
		t.Run(flagType, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{
				{
					Name:        "my-tool",
					Description: "A tool",
					Entrypoint:  "./tool.sh",
					Flags: []Flag{
						{
							Name:        "myflag",
							Type:        flagType,
							Description: "A non-object flag",
							// No itemSchema — but that's expected for non-object types
						},
					},
				},
			}

			errs := Validate(tk)
			ve := findErrorByRule(errs, "missing-item-schema")
			assert.Nil(t, ve,
				"Non-object type %q without itemSchema should NOT produce missing-item-schema, got: %v",
				flagType, errs)
		})
	}
}

func TestValidateFlag_MissingItemSchema_IsWarningNotError(t *testing.T) {
	// Explicitly verify the severity is warning, not error or empty string.
	// This prevents a lazy implementation that sets all issues to error severity.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "payload",
					Type:        "object",
					Description: "Object without schema",
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "missing-item-schema")
	require.NotNil(t, ve,
		"object type without itemSchema must produce missing-item-schema result")

	// Must be exactly "warning" — not "" (empty), not "error", not "info"
	assert.Equal(t, SeverityWarning, ve.Severity,
		"missing-item-schema severity must be exactly %q, got %q", SeverityWarning, ve.Severity)
}

func TestValidateFlag_ObjectWithItemSchema_NoMissingWarning(t *testing.T) {
	// When itemSchema IS present on an object flag, no missing-item-schema warning.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "config",
					Type:        "object",
					Description: "Object with schema",
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"key": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "missing-item-schema")
	assert.Nil(t, ve,
		"object with itemSchema should NOT produce missing-item-schema warning")
}

// ---------------------------------------------------------------------------
// AC6: object default validation with and without itemSchema
// ---------------------------------------------------------------------------

func TestCheckDefaultType_Object_MapDefault_NoSchema(t *testing.T) {
	// type: "object" with a map default and no itemSchema should pass.
	err := checkDefaultType("object", map[string]any{"key": "val"})
	assert.NoError(t, err,
		"checkDefaultType(\"object\", map{key:val}) should pass when no schema context")
}

func TestCheckDefaultType_Object_NonMapDefault(t *testing.T) {
	// type: "object" with a non-map default should fail.
	tests := []struct {
		name  string
		value any
	}{
		{name: "string default", value: "not-a-map"},
		{name: "int default", value: 42},
		{name: "bool default", value: true},
		{name: "slice default", value: []interface{}{"a", "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDefaultType("object", tc.value)
			require.Error(t, err,
				"checkDefaultType(\"object\", %v (%T)) should fail for non-map default",
				tc.value, tc.value)
		})
	}
}

func TestValidateFlag_ObjectDefault_MatchesItemSchema_Passes(t *testing.T) {
	// An object flag whose default matches the itemSchema should produce no type-mismatch error.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "config",
					Type:        "object",
					Description: "Config with valid default",
					Default:     map[string]any{"name": "test", "count": 5},
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":  map[string]any{"type": "string"},
							"count": map[string]any{"type": "integer"},
						},
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	assert.Nil(t, ve,
		"Object default matching itemSchema should not produce type-mismatch error, got: %v", errs)
}

func TestValidateFlag_ObjectDefault_ViolatesItemSchema_Error(t *testing.T) {
	// An object flag whose default violates the itemSchema should produce a type-mismatch error.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "config",
					Type:        "object",
					Description: "Config with invalid default",
					Default:     map[string]any{"name": 12345}, // name should be string, not int
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	require.NotNil(t, ve,
		"Object default violating itemSchema should produce type-mismatch at tools[0].flags[0].default, got rules: %v",
		errRules(errs))
	assert.Contains(t, ve.Message, "name",
		"Error message should indicate which field violated the schema")
}

func TestValidateFlag_ObjectDefault_MissingRequiredField_Error(t *testing.T) {
	// An object default missing a required field should produce a type-mismatch error.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "config",
					Type:        "object",
					Description: "Config with missing required field",
					Default:     map[string]any{"optional": "value"}, // missing "name"
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
						"required": []any{"name"},
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	require.NotNil(t, ve,
		"Object default missing required field should produce type-mismatch error, got rules: %v",
		errRules(errs))
}

func TestValidateFlag_ObjectDefault_NoItemSchema_MapPasses(t *testing.T) {
	// AC6: type: "object" with no itemSchema accepts any JSON object as default.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "data",
					Type:        "object",
					Description: "Schemaless object flag",
					Default:     map[string]any{"anything": "goes", "nested": map[string]any{"deep": true}},
					// No ItemSchema — accepts any map
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	assert.Nil(t, ve,
		"Object default with no itemSchema should accept any map, got error: %v", ve)
}

func TestValidateFlag_ObjectDefault_NoItemSchema_NonMapFails(t *testing.T) {
	// Even without itemSchema, a non-map default for type:"object" should fail.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "data",
					Type:        "object",
					Description: "Object flag with string default",
					Default:     "not-a-map",
					// No ItemSchema
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	require.NotNil(t, ve,
		"Object flag with string default should produce type-mismatch, got rules: %v",
		errRules(errs))
}

func TestValidateFlag_ObjectArrayDefault_ValidArray_Passes(t *testing.T) {
	// type: "object[]" with a valid array of maps should pass.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "items",
					Type:        "object[]",
					Description: "Array of objects",
					Default:     []interface{}{map[string]any{"id": 1}, map[string]any{"id": 2}},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	assert.Nil(t, ve,
		"object[] with valid array-of-maps default should pass, got error: %v", ve)
}

func TestValidateFlag_ObjectArrayDefault_NotArray_Fails(t *testing.T) {
	// type: "object[]" with a non-array default should fail.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "items",
					Type:        "object[]",
					Description: "Array of objects",
					Default:     map[string]any{"id": 1}, // single map, not array
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	require.NotNil(t, ve,
		"object[] with non-array default should produce type-mismatch, got rules: %v",
		errRules(errs))
}

func TestValidateFlag_ObjectArrayDefault_ElementViolatesSchema_Fails(t *testing.T) {
	// type: "object[]" with itemSchema, one element violates it.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "items",
					Type:        "object[]",
					Description: "Array of typed objects",
					Default: []interface{}{
						map[string]any{"name": "valid"},
						map[string]any{"name": 999}, // name should be string
					},
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findError(errs, "tools[0].flags[0].default", "type-mismatch")
	require.NotNil(t, ve,
		"object[] with element violating itemSchema should produce type-mismatch, got rules: %v",
		errRules(errs))
}

// ---------------------------------------------------------------------------
// Adversarial: prevent hardcoded rule checks and implementation shortcuts
// ---------------------------------------------------------------------------

func TestValidateFlag_ItemSchema_MultipleFlags_IndependentValidation(t *testing.T) {
	// Two flags on the same tool: one valid itemSchema, one invalid.
	// Both should be independently validated (not short-circuit).
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "good-config",
					Type:        "object",
					Description: "Valid schema",
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
					},
				},
				{
					Name:        "bad-config",
					Type:        "object",
					Description: "Invalid schema",
					ItemSchema: map[string]any{
						"type": "nonexistent",
					},
				},
			},
		},
	}

	errs := Validate(tk)

	// First flag should be clean
	goodFlagErrs := findError(errs, "tools[0].flags[0].itemSchema", "invalid-item-schema")
	assert.Nil(t, goodFlagErrs,
		"First flag with valid itemSchema should not produce error")

	// Second flag should have error
	ve := findErrorByRule(errs, "invalid-item-schema")
	require.NotNil(t, ve,
		"Second flag with invalid itemSchema should produce error, got rules: %v", errRules(errs))
	assert.Contains(t, ve.Path, "tools[0].flags[1]",
		"Error path should reference flags[1], the second flag")
}

func TestValidateFlag_ItemSchema_EmptyMap_NotAllowedAsSchema(t *testing.T) {
	// An empty map {} is technically valid JSON Schema (matches anything).
	// This should NOT produce an invalid-item-schema error.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "config",
					Type:        "object",
					Description: "Empty schema",
					ItemSchema:  map[string]any{},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "invalid-item-schema")
	assert.Nil(t, ve,
		"Empty itemSchema {} is valid JSON Schema (matches anything), should not error")
}

func TestValidateFlag_ItemSchema_SecondToolSecondFlag_PathCorrect(t *testing.T) {
	// Verify error path is correct for deeply-indexed flags.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "tool-a",
			Description: "A",
			Entrypoint:  "./a.sh",
			Flags: []Flag{
				{Name: "ok", Type: "string", Description: "Fine"},
			},
		},
		{
			Name:        "tool-b",
			Description: "B",
			Entrypoint:  "./b.sh",
			Flags: []Flag{
				{Name: "also-ok", Type: "int", Description: "Fine"},
				{
					Name:        "bad",
					Type:        "string",
					Description: "Should not have itemSchema",
					ItemSchema: map[string]any{
						"type": "object",
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "item-schema-not-allowed")
	require.NotNil(t, ve,
		"String flag with itemSchema should produce item-schema-not-allowed, got rules: %v",
		errRules(errs))
	assert.Contains(t, ve.Path, "tools[1].flags[1]",
		"Error path should be tools[1].flags[1], got %q", ve.Path)
}

func TestValidateFlag_ExistingErrors_SeverityDefaultsToError(t *testing.T) {
	// Existing validation errors (like type-mismatch, unknown-flag-type) should
	// have Severity of "error" or "" (backwards compatible). This test ensures
	// the severity distinction between warnings and errors is meaningful.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{Name: "bad", Type: "badtype", Description: "Unknown type"},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "unknown-flag-type")
	require.NotNil(t, ve, "Should produce unknown-flag-type error")

	// An existing error's severity should be "error" (not warning, not empty).
	// This ensures warning severity is genuinely different from error severity.
	assert.Equal(t, SeverityError, ve.Severity,
		"Existing validation errors should have severity %q, got %q",
		SeverityError, ve.Severity)
}

func TestValidateFlag_ItemSchema_BothInvalidAndNotAllowed_OnlyNotAllowed(t *testing.T) {
	// If a non-object type has itemSchema with invalid JSON Schema, should the
	// error be "item-schema-not-allowed" (since it's on the wrong type)?
	// The not-allowed check should take precedence — we don't need to validate
	// a schema that shouldn't be there in the first place.
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "my-tool",
			Description: "A tool",
			Entrypoint:  "./tool.sh",
			Flags: []Flag{
				{
					Name:        "bad",
					Type:        "string",
					Description: "String with invalid schema",
					ItemSchema: map[string]any{
						"type": "nonexistent",
					},
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "item-schema-not-allowed")
	require.NotNil(t, ve,
		"String flag with itemSchema should produce item-schema-not-allowed, got rules: %v",
		errRules(errs))

	// Should NOT also produce invalid-item-schema (would be redundant)
	ve2 := findErrorByRule(errs, "invalid-item-schema")
	assert.Nil(t, ve2,
		"When item-schema-not-allowed fires, invalid-item-schema should not also fire")
}

// ---------------------------------------------------------------------------
// Security boundary tests: property names, flag/arg names, depth limit
// ---------------------------------------------------------------------------

func TestValidateFlag_ItemSchema_InvalidPropertyName_Rejected(t *testing.T) {
	tests := []struct {
		name     string
		propName string
	}{
		{"injection via closing brace", `}); process.exit(1); //`},
		{"injection via quote", `": z.string(), }); import("child_process"); const x = { "`},
		{"starts with digit", "123bad"},
		{"contains space", "foo bar"},
		{"contains hyphen", "foo-bar"},
		{"contains dot", "foo.bar"},
		{"empty string", ""},
		{"contains semicolon", "foo;bar"},
		{"contains newline", "foo\nbar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{{
				Name:        "my-tool",
				Description: "A tool",
				Entrypoint:  "./tool.sh",
				Flags: []Flag{{
					Name:        "data",
					Type:        "object",
					Description: "Object flag",
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							tt.propName: map[string]any{"type": "string"},
						},
					},
				}},
			}}
			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-property-name")
			require.NotNil(t, ve,
				"Property name %q should be rejected as invalid-property-name, got rules: %v",
				tt.propName, errRules(errs))
			assert.Equal(t, SeverityError, ve.Severity)
		})
	}
}

func TestValidateFlag_ItemSchema_ValidPropertyName_Accepted(t *testing.T) {
	validNames := []string{"name", "firstName", "_private", "$ref", "x1", "A", "camelCase"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{{
				Name:        "my-tool",
				Description: "A tool",
				Entrypoint:  "./tool.sh",
				Flags: []Flag{{
					Name:        "data",
					Type:        "object",
					Description: "Object flag",
					ItemSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							name: map[string]any{"type": "string"},
						},
					},
				}},
			}}
			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-property-name")
			assert.Nil(t, ve,
				"Property name %q should be accepted, but got invalid-property-name", name)
		})
	}
}

func TestValidateFlag_ItemSchema_NestedInvalidPropertyName_Rejected(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{{
		Name:        "my-tool",
		Description: "A tool",
		Entrypoint:  "./tool.sh",
		Flags: []Flag{{
			Name:        "data",
			Type:        "object",
			Description: "Object flag",
			ItemSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"safe": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"bad name": map[string]any{"type": "string"},
						},
					},
				},
			},
		}},
	}}
	errs := Validate(tk)
	ve := findErrorByRule(errs, "invalid-property-name")
	require.NotNil(t, ve,
		"Nested invalid property name should be rejected")
}

func TestValidateTool_FlagName_InvalidFormat_Rejected(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"starts with digit", "1bad"},
		{"contains space", "foo bar"},
		{"injection via quote", `foo"; os.Exit(1); //`},
		{"empty string", ""},
		{"starts with hyphen", "-flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{{
				Name:        "my-tool",
				Description: "A tool",
				Entrypoint:  "./tool.sh",
				Flags: []Flag{{
					Name:        tt.flagName,
					Type:        "string",
					Description: "A flag",
				}},
			}}
			errs := Validate(tk)
			ve := findError(errs, "tools[0].flags[0].name", "name-format")
			require.NotNil(t, ve,
				"Flag name %q should be rejected with name-format, got rules: %v",
				tt.flagName, errRules(errs))
			assert.Equal(t, SeverityError, ve.Severity)
		})
	}
}

func TestValidateTool_ArgName_InvalidFormat_Rejected(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{{
		Name:        "my-tool",
		Description: "A tool",
		Entrypoint:  "./tool.sh",
		Args: []Arg{{
			Name:        "123bad",
			Type:        "string",
			Description: "An arg",
		}},
	}}
	errs := Validate(tk)
	ve := findError(errs, "tools[0].args[0].name", "name-format")
	require.NotNil(t, ve,
		"Arg name %q should be rejected with name-format", "123bad")
}

func TestValidateTool_FlagName_ValidFormat_Accepted(t *testing.T) {
	validNames := []string{"verbose", "dry-run", "output_format", "V", "x1"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			tk := validToolkit()
			tk.Tools = []Tool{{
				Name:        "my-tool",
				Description: "A tool",
				Entrypoint:  "./tool.sh",
				Flags: []Flag{{
					Name:        name,
					Type:        "string",
					Description: "A flag",
				}},
			}}
			errs := Validate(tk)
			ve := findError(errs, "tools[0].flags[0].name", "name-format")
			assert.Nil(t, ve,
				"Flag name %q should be accepted, but got name-format error", name)
		})
	}
}

func TestValidateFlag_ItemSchema_ExceedsMaxDepth_Rejected(t *testing.T) {
	// Build a schema nested beyond maxItemSchemaDepth.
	schema := map[string]any{"type": "string"}
	for i := 0; i < maxItemSchemaDepth+5; i++ {
		schema = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"nested": schema,
			},
		}
	}
	tk := validToolkit()
	tk.Tools = []Tool{{
		Name:        "my-tool",
		Description: "A tool",
		Entrypoint:  "./tool.sh",
		Flags: []Flag{{
			Name:        "data",
			Type:        "object",
			Description: "Deep object",
			ItemSchema:  schema,
		}},
	}}
	errs := Validate(tk)
	// Should hit either invalid-property-name (depth limit) or invalid-item-schema (depth limit).
	hasDepthError := false
	for _, e := range errs {
		if strings.Contains(e.Message, "maximum nesting depth") {
			hasDepthError = true
			break
		}
	}
	assert.True(t, hasDepthError,
		"Schema exceeding maxItemSchemaDepth should produce a depth error, got: %v", errRules(errs))
}
