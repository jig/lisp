// Package nsversion loads the version namespace into a jig/lisp
// environment.
package nsversion

import (
	"github.com/jig/lisp/lib/version"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	version.Load(env)
	return nil
}
