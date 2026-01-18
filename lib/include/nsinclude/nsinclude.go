package nsinclude

import (
	"github.com/jig/lisp/lib/include"
	"github.com/jig/lisp/types"
)

func Load(binary string) func(types.EnvType) error {
	return include.Load(binary)
}
