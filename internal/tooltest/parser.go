package tooltest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// yamlAssertion mirrors Assertion with yaml tags for unmarshaling.
// Pointer fields ensure zero-value vs explicit-false are distinguishable.
type yamlAssertion struct {
	Path     string `yaml:"path"`
	Equals   any    `yaml:"equals"`
	Contains any    `yaml:"contains"`
	Matches  string `yaml:"matches"`
	Exists   *bool  `yaml:"exists"`
	Length   *int   `yaml:"length"`
}

// yamlExpectation mirrors Expectation with yaml tags for unmarshaling.
type yamlExpectation struct {
	ExitCode       *int            `yaml:"exit_code"`
	StdoutIsJSON   *bool           `yaml:"stdout_is_json"`
	StdoutSchema   string          `yaml:"stdout_schema"`
	StdoutContains []yamlAssertion `yaml:"stdout_contains"`
	StderrContains []string        `yaml:"stderr_contains"`
}

// yamlTestCase mirrors TestCase with yaml tags for unmarshaling.
type yamlTestCase struct {
	Name      string            `yaml:"name"`
	Args      []string          `yaml:"args"`
	Flags     map[string]string `yaml:"flags"`
	AuthToken string            `yaml:"auth_token"`
	Expect    yamlExpectation   `yaml:"expect"`
	Timeout   string            `yaml:"timeout"`
}

// yamlTestSuite mirrors TestSuite with yaml tags for unmarshaling.
type yamlTestSuite struct {
	Tool  string         `yaml:"tool"`
	Tests []yamlTestCase `yaml:"tests"`
}

// expandAuthToken resolves auth token env-var references.
// If raw matches "${VAR_NAME}", it returns os.Getenv("VAR_NAME").
// If raw is empty, it checks TOOLWRIGHT_TEST_TOKEN as a fallback.
// Otherwise it returns raw unchanged.
func expandAuthToken(raw string) string {
	if strings.HasPrefix(raw, "${") && strings.HasSuffix(raw, "}") {
		varName := raw[2 : len(raw)-1]
		return os.Getenv(varName)
	}
	if raw == "" {
		return os.Getenv("TOOLWRIGHT_TEST_TOKEN")
	}
	return raw
}

// convertAssertion maps a yamlAssertion to the public Assertion type.
func convertAssertion(ya yamlAssertion) Assertion {
	return Assertion(ya)
}

// convertExpectation maps a yamlExpectation to the public Expectation type.
func convertExpectation(ye yamlExpectation) Expectation {
	ex := Expectation{
		ExitCode:       ye.ExitCode,
		StdoutIsJSON:   ye.StdoutIsJSON,
		StdoutSchema:   ye.StdoutSchema,
		StderrContains: ye.StderrContains,
	}
	if len(ye.StdoutContains) > 0 {
		ex.StdoutContains = make([]Assertion, len(ye.StdoutContains))
		for i, ya := range ye.StdoutContains {
			ex.StdoutContains[i] = convertAssertion(ya)
		}
	}
	return ex
}

// convertTestCase maps a yamlTestCase to the public TestCase type.
func convertTestCase(yc yamlTestCase) (TestCase, error) {
	tc := TestCase{
		Name:      yc.Name,
		Args:      yc.Args,
		Flags:     yc.Flags,
		AuthToken: expandAuthToken(yc.AuthToken),
		Expect:    convertExpectation(yc.Expect),
	}
	if yc.Timeout != "" {
		d, err := time.ParseDuration(yc.Timeout)
		if err != nil {
			return TestCase{}, fmt.Errorf("parse timeout %q: %w", yc.Timeout, err)
		}
		tc.Timeout = d
	}
	return tc, nil
}

// ParseTestFile parses a single .test.yaml file into a TestSuite.
func ParseTestFile(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from controlled inputs (glob or caller-provided)
	if err != nil {
		return nil, fmt.Errorf("read test file %q: %w", path, err)
	}

	var raw yamlTestSuite
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse test file %q: %w", path, err)
	}

	if raw.Tool == "" {
		return nil, fmt.Errorf("test file %q: missing or empty required field 'tool'", path)
	}

	suite := &TestSuite{
		Tool: raw.Tool,
	}
	if len(raw.Tests) > 0 {
		suite.Tests = make([]TestCase, len(raw.Tests))
		for i, yc := range raw.Tests {
			tc, err := convertTestCase(yc)
			if err != nil {
				return nil, fmt.Errorf("test file %q test case %d: %w", path, i, err)
			}
			suite.Tests[i] = tc
		}
	}

	return suite, nil
}

// ParseTestDir globs *.test.yaml from a directory and parses each into a TestSuite.
func ParseTestDir(dir string) ([]TestSuite, error) {
	// Verify the directory exists by attempting to read it.
	if _, err := os.ReadDir(dir); err != nil {
		return nil, fmt.Errorf("read test directory %q: %w", dir, err)
	}

	pattern := filepath.Join(dir, "*.test.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob test files in %q: %w", dir, err)
	}

	suites := make([]TestSuite, 0, len(matches))
	for _, path := range matches {
		suite, err := ParseTestFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse test dir %q: %w", dir, err)
		}
		suites = append(suites, *suite)
	}

	return suites, nil
}
