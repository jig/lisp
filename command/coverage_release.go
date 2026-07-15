//go:build !lispdebug

package command

import "errors"

// startCoverage is unavailable in release builds: the eval hook that
// feeds the collector is compiled out. Build with -tags lispdebug (the
// lisp-debug binary) to record coverage.
func startCoverage(_ string) (func() error, error) {
	return nil, errors.New("--coverage requires the lispdebug build (use the lisp-debug binary)")
}
