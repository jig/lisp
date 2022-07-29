package repl

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

func TestMultiline(t *testing.T) {
	for _, partialLine := range []string{"(", "{", "["} {
		ns, _ := env.NewEnv(nil, nil, nil)
		_, err := lisp.REPL(context.Background(), ns, partialLine, types.NewCursorFile("REPL TEST"))
		if err == nil {
			t.Fatal("test failed")
		}
		if err, ok := err.(interface {
			Error() string
			GetEncapsulated() types.MalType
		}); ok && err.GetEncapsulated() != nil {
			if err.Error() == "expected ')', got EOF" ||
				err.Error() == "expected ']', got EOF" ||
				err.Error() == "expected '}', got EOF" {
				continue
			}
			t.Fatalf("test failed for %s", partialLine)
		}
	}
}
