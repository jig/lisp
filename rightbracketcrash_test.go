package lisp

import (
	_ "embed"
	"testing"

	"github.com/jig/lisp/env"
)

func TestRightBracketCrash(t *testing.T) {
	if _, err := READ(")", nil, env.NewEnv()); err == nil || err.Error() != "unexpected ')'" {
		t.Fatal(err)
	}
}
