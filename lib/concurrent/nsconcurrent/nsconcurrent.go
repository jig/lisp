package nsconcurrent

import (
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	concurrent.Load(env)
	return nil
}
