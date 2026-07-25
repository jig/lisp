package log

import "io"

// SetOutput redirects the package logger for tests.
func SetOutput(w io.Writer) {
	logger = newLogger(w)
}
