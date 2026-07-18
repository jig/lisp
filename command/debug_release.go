//go:build !debugger

package command

import "errors"

// setupDebugHook fails in release builds. Rebuild with `-tags debugger`
// to enable the --debug flag.
func setupDebugHook() error {
	return errors.New("--debug requires a debug build: rebuild with -tags debugger")
}
