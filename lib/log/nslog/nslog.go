// Package nslog loads the log namespace into a jig/lisp environment.
package nslog

import (
	"github.com/jig/lisp/lib/log"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	log.Load(env)
	return nil
}
