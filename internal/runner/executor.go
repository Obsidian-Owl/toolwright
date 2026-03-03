package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// MaxOutputBytes is the maximum number of bytes captured from stdout or stderr.
// Prevents memory exhaustion from runaway tool output (constitution rule 26).
const MaxOutputBytes = 10 << 20 // 10 MiB

// BuildArgs assembles the argument slice to pass to a tool's entrypoint.
// Order: positional args, then flags (in tool.Flags definition order), then token.
func BuildArgs(tool manifest.Tool, positionalArgs []string, flags map[string]string, token string) []string {
	result := make([]string, 0, len(positionalArgs))

	// 1. Positional args in order.
	result = append(result, positionalArgs...)

	// 2. Flags in tool.Flags slice order (deterministic, not map iteration order).
	for _, f := range tool.Flags {
		val, ok := flags[f.Name]
		if !ok || val == "" {
			continue
		}
		if f.Type == "bool" {
			if val == "true" {
				result = append(result, fmt.Sprintf("--%s", f.Name))
			}
			// val == "false" → skip entirely
			continue
		}
		result = append(result, fmt.Sprintf("--%s", f.Name), val)
	}

	// 3. Token last, using the exact TokenFlag string (already includes --).
	if tool.Auth != nil && tool.Auth.TokenFlag != "" && token != "" {
		result = append(result, tool.Auth.TokenFlag, token)
	}

	return result
}

// Executor runs tools as child processes.
type Executor struct {
	WorkDir string
}

// Run executes the tool's entrypoint with the given args, flags, and token.
// Non-zero exit codes are captured in Result and do NOT cause an error return.
// Only true execution failures (entrypoint not found, context cancellation) return an error.
func (e *Executor) Run(ctx context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*Result, error) {
	// Validate WorkDir exists and is a directory (defense-in-depth, rule 26).
	if e.WorkDir != "" {
		info, err := os.Stat(e.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("invalid working directory: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("invalid working directory: %s is not a directory", e.WorkDir)
		}
	}

	// Sanitize entrypoint path (defense-in-depth, rule 26).
	entrypoint := filepath.Clean(tool.Entrypoint)

	argv := BuildArgs(tool, args, flags, token)

	// Use exec.Command (not CommandContext) so we control process group killing ourselves.
	cmd := exec.Command(entrypoint, argv...)
	cmd.Dir = e.WorkDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Cap output buffers to prevent memory exhaustion (rule 26).
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdoutBuf, remaining: MaxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderrBuf, remaining: MaxOutputBytes}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Watch the context in a goroutine; kill the entire process group on cancellation.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		case <-done:
		}
	}()

	start := time.Now()
	err := cmd.Wait()
	duration := time.Since(start)
	close(done)

	if err != nil {
		// Context was cancelled or timed out.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tool execution interrupted: %w", ctx.Err())
		}

		// Non-zero exit code — not an error, capture in Result.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &Result{
				ExitCode: exitErr.ExitCode(),
				Stdout:   stdoutBuf.Bytes(),
				Stderr:   stderrBuf.Bytes(),
				Duration: duration,
			}, nil
		}

		// True execution failure (unexpected wait error).
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return &Result{
		ExitCode: 0,
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		Duration: duration,
	}, nil
}

// limitedWriter wraps an io.Writer and silently discards bytes beyond a cap.
// Prevents memory exhaustion from runaway tool output (constitution rule 26).
type limitedWriter struct {
	w         io.Writer
	remaining int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.remaining <= 0 {
		return len(p), nil // discard silently
	}
	if int64(len(p)) > lw.remaining {
		p = p[:lw.remaining]
	}
	n, err := lw.w.Write(p)
	lw.remaining -= int64(n)
	return n, err
}
