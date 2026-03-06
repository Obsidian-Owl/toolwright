package cli

import "errors"

// UsageError wraps an error that represents a CLI usage mistake (e.g. missing
// positional argument). Commands that detect usage errors return this type so
// that main.go can map it to ExitUsage (exit code 2).
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// IOError wraps an error that represents an I/O failure (e.g. file not found,
// permission denied). Commands that detect I/O errors return this type so that
// main.go can map it to ExitIO (exit code 3).
type IOError struct {
	Err error
}

func (e *IOError) Error() string { return e.Err.Error() }
func (e *IOError) Unwrap() error { return e.Err }

// ExitCodeForError returns the appropriate exit code for the given error.
// nil -> ExitSuccess (0)
// *UsageError -> ExitUsage (2)
// *IOError -> ExitIO (3)
// anything else -> ExitError (1)
func ExitCodeForError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var usageErr *UsageError
	var ioErr *IOError
	switch {
	case errors.As(err, &usageErr):
		return ExitUsage
	case errors.As(err, &ioErr):
		return ExitIO
	default:
		return ExitError
	}
}
