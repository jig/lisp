package nssystem

import (
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	system.Load(env)
	return nil
}
