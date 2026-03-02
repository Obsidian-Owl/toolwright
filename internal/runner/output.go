package runner

import "time"

// Result holds the output of a completed tool execution.
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
}
