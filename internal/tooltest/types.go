package tooltest

import "time"

type TestSuite struct {
	Tool  string
	Tests []TestCase
}

type TestCase struct {
	Name      string
	Args      []string
	Flags     map[string]string
	AuthToken string
	Expect    Expectation
	Timeout   time.Duration
}

type Expectation struct {
	ExitCode       *int
	StdoutIsJSON   *bool
	StdoutSchema   string
	StdoutContains []Assertion
	StderrContains []string
}

type Assertion struct {
	Path     string
	Equals   any
	Contains any
	Matches  string
	Exists   *bool
	Length   *int
}

type TestResult struct {
	Name     string
	Status   string
	Duration time.Duration
	Error    string
	Stdout   []byte
	Stderr   []byte
}

type TestReport struct {
	Tool    string
	Total   int
	Passed  int
	Failed  int
	Results []TestResult
}
