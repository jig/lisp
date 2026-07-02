// Package nsrequire exposes the require library loader following the
// same convention as the other lib/* namespaces (nscore, nsconcurrent…).
package nsrequire

import (
	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/types"
)

// Load returns a loader that installs `require` and `resolve-require`,
// reading include directories from the command line (-i/--include).
// binary names the per-binary standard search directories.
func Load(binary string) func(types.EnvType) error {
	return require.Load(binary)
}
