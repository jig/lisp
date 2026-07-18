// Package coreextended provides the extended standard library: a large
// pure-lisp part (header-coreextended.lisp) and a few Go builtins that
// need the Go runtime, like format.
package coreextended

import (
	"fmt"

	_ "embed"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/printer"
	. "github.com/jig/lisp/types"
)

//go:embed header-coreextended.lisp
var headerCoreExtended string

func HeaderCoreExtended() string { return headerCoreExtended }

// Load registers the Go-implemented builtins of the namespace; the lisp
// part is evaluated separately by nscoreextended.
func Load(env EnvType) {
	call.CallOverrideFN(env, "format", format, 1)

	call.Doc(env, "format", "[fmt & args]",
		"Formats args into fmt using Go verbs (%s %d %f %v %q %x ...); collections and keywords render in their lisp form. Returns the string.")
}

// lispArg defers an argument's rendering to the lisp printer, so %s and
// %v on collections and keywords show lisp forms instead of Go internals.
type lispArg struct{ v MalType }

func (l lispArg) String() string { return printer.Pr_str(l.v, false) }

// formatArg adapts a lisp value for Go's fmt: primitives pass through so
// numeric and boolean verbs keep working, values with their own fmt
// support (e.g. big numbers) pass through too, and everything else is
// rendered by the lisp printer.
func formatArg(v MalType) any {
	switch t := v.(type) {
	case nil, int, float32, float64, bool:
		// float32 is the reader's float type; float64 arrives from Go libs
		return t
	case string:
		if Keyword_Q(t) {
			return lispArg{t}
		}
		return t
	default:
		if _, ok := t.(fmt.Formatter); ok {
			return t
		}
		return lispArg{t}
	}
}

func format(f string, args ...MalType) (MalType, error) {
	conv := make([]any, len(args))
	for i, a := range args {
		conv[i] = formatArg(a)
	}
	return fmt.Sprintf(f, conv...), nil
}
