package nsintegrity

import (
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/types"
)

func Load(env types.EnvType) error {
	integrity.Load(env)
	return nil
}
