// Package nslazy loads the lazy-sequence namespace into a jig/lisp
// environment. The namespace is implemented entirely in Go, so loading it is
// just a matter of registering the builtins.
package nslazy

import (
	"github.com/jig/lisp/lib/lazy"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	lazy.Load(env)
	return nil
}
