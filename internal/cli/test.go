package cli

import (
	"context"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/Obsidian-Owl/toolwright/internal/tooltest"
)

// suiteRunner abstracts test suite execution so the CLI layer can be tested
// without spawning real child processes. Production code wires
// tooltest.TestRunner here.
type suiteRunner interface {
	Run(ctx context.Context, suite tooltest.TestSuite, toolkit manifest.Toolkit) (*tooltest.TestReport, error)
	RunParallel(ctx context.Context, suite tooltest.TestSuite, toolkit manifest.Toolkit, workers int) (*tooltest.TestReport, error)
}

// testParser abstracts test file discovery so the CLI layer can be tested
// without filesystem access. Production code wires tooltest.ParseTestDir here.
type testParser interface {
	ParseDir(dir string) ([]tooltest.TestSuite, error)
}

// testConfig holds the injectable dependencies for the test command.
type testConfig struct {
	Runner suiteRunner
	Parser testParser
}

// newTestCmd returns the test subcommand. cfg provides the runner and parser
// dependencies; in production these are wired to real implementations.
func newTestCmd(cfg *testConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test scenarios against tools",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTests(cmd, cfg)
		},
	}

	cmd.Flags().StringP("manifest", "m", "toolwright.yaml", "path to manifest file")
	cmd.Flags().StringP("tests", "t", "tests/", "path to tests directory")
	cmd.Flags().StringP("filter", "f", "", "regex filter for test names")
	cmd.Flags().IntP("parallel", "p", 1, "number of parallel workers")

	return cmd
}

// runTests implements the core logic of the test subcommand.
func runTests(cmd *cobra.Command, cfg *testConfig) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	manifestPath, _ := cmd.Flags().GetString("manifest")
	testsDir, _ := cmd.Flags().GetString("tests")
	filter, _ := cmd.Flags().GetString("filter")
	parallel, _ := cmd.Flags().GetInt("parallel")

	w := cmd.OutOrStdout()

	// Load manifest.
	tk, err := loadManifest(manifestPath)
	if err != nil {
		if jsonMode {
			_ = outputError(w, "io_error",
				fmt.Sprintf("cannot load manifest: %s", manifestPath),
				"check that the file exists and is readable")
		}
		return err
	}

	// Parse test suites.
	suites, err := cfg.Parser.ParseDir(testsDir)
	if err != nil {
		if jsonMode {
			_ = outputError(w, "io_error",
				fmt.Sprintf("cannot read tests directory: %s", testsDir),
				"check that the directory exists and is readable")
		}
		return err
	}

	// Apply filter if set.
	if filter != "" {
		re, reErr := regexp.Compile(filter)
		if reErr != nil {
			return fmt.Errorf("invalid filter regex: %w", reErr)
		}
		suites = filterSuites(suites, re)
	}

	// No suites: emit a zero-count TAP plan and return.
	if len(suites) == 0 {
		empty := tooltest.TestReport{}
		if jsonMode {
			_ = tooltest.FormatJSON(empty, w)
		} else {
			_ = tooltest.FormatTAP(empty, w)
		}
		return nil
	}

	// Run suites and collect reports.
	var reports []*tooltest.TestReport
	anyFailed := false

	for _, suite := range suites {
		var report *tooltest.TestReport
		var runErr error

		if parallel > 1 {
			report, runErr = cfg.Runner.RunParallel(cmd.Context(), suite, *tk, parallel)
		} else {
			report, runErr = cfg.Runner.Run(cmd.Context(), suite, *tk)
		}

		if runErr != nil {
			if jsonMode {
				_ = outputError(w, "runner_error", runErr.Error(), "check tool configuration")
			}
			return runErr
		}

		if report.Failed > 0 {
			anyFailed = true
		}
		reports = append(reports, report)
	}

	// Output all reports.
	for _, report := range reports {
		if jsonMode {
			if err := tooltest.FormatJSON(*report, w); err != nil {
				return fmt.Errorf("format JSON: %w", err)
			}
		} else {
			if err := tooltest.FormatTAP(*report, w); err != nil {
				return fmt.Errorf("format TAP: %w", err)
			}
		}
	}

	if anyFailed {
		return fmt.Errorf("one or more tests failed")
	}

	return nil
}

// filterSuites filters each suite's test cases to only those whose Name
// matches re. Suites with no matching cases are dropped.
func filterSuites(suites []tooltest.TestSuite, re *regexp.Regexp) []tooltest.TestSuite {
	var result []tooltest.TestSuite
	for _, suite := range suites {
		var matched []tooltest.TestCase
		for _, tc := range suite.Tests {
			if re.MatchString(tc.Name) {
				matched = append(matched, tc)
			}
		}
		if len(matched) > 0 {
			result = append(result, tooltest.TestSuite{
				Tool:  suite.Tool,
				Tests: matched,
			})
		}
	}
	return result
}
