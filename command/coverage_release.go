//go:build !debugger

package command

import "errors"

// startCoverage is unavailable in release builds: the eval hook that
// feeds the collector is compiled out. Build with -tags debugger (the
// a binary built with -tags debugger) to record coverage.
func startCoverage(_ string) (func() error, error) {
	return nil, errors.New("--coverage requires a build with -tags debugger")
}
