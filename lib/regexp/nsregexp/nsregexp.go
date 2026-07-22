// Package nsregexp loads the regexp namespace into a jig/lisp environment.
package nsregexp

import (
	"github.com/jig/lisp/lib/regexp"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	regexp.Load(env)
	return nil
}
