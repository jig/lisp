//go:build debugger

package debugadapter

import (
	"context"
	"strings"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

// hookEvalTimeout bounds a breakpoint condition or logpoint expression so
// a runaway expression cannot wedge the debuggee (the hook holds s.mu
// while it runs).
const hookEvalTimeout = 2 * time.Second

// evalInHook evaluates expr in env from inside the hook, i.e. while the
// caller holds s.mu on the debuggee goroutine. It sets evalGuard so the
// nested EVAL's OnEval returns immediately instead of re-entering the
// locked state, and runs on a fresh background context carrying no
// runtime.Thread so it pushes no frames onto the debuggee's stack.
func (s *state) evalInHook(expr string, env types.EnvType) (types.MalType, error) {
	s.evalGuard.Store(true)
	defer s.evalGuard.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), hookEvalTimeout)
	defer cancel()

	ast, err := lisp.READ(expr, types.NewAnonymousCursorHere(1, 1), env)
	if err != nil {
		return nil, err
	}
	return lisp.EVAL(ctx, ast, env)
}

// evalCondition reports whether a breakpoint condition holds. A condition
// that fails to evaluate is treated as met, so a mistyped condition stops
// (and surfaces) rather than silently disabling the breakpoint.
func (s *state) evalCondition(_ context.Context, expr string, env types.EnvType) bool {
	v, err := s.evalInHook(expr, env)
	if err != nil {
		return true
	}
	return truthy(v)
}

// emitLog interpolates a logpoint message and sends it to the client as
// output, without pausing. Unlike the `stopped` event (which is sent from
// a goroutine because pauseAndWait then blocks under s.mu), the send is
// synchronous: a logpoint does not pause, so ordering the output before
// execution continues is both safe and what the user expects. Transport
// writes are serialised by their own mutex.
func (s *state) emitLog(_ context.Context, msg string, env types.EnvType) {
	out := s.interpolate(msg, env)
	s.server.sendEvent("output", OutputEventBody{Category: "console", Output: out + "\n"})
}

// interpolate expands `{expr}` placeholders in a logpoint message by
// evaluating each expression in env. `{{` and `}}` are literal braces.
// An expression that errors is rendered inline as `{error: …}` so the
// logpoint still produces output.
func (s *state) interpolate(msg string, env types.EnvType) string {
	var b strings.Builder
	for i := 0; i < len(msg); {
		switch c := msg[i]; c {
		case '{':
			if i+1 < len(msg) && msg[i+1] == '{' {
				b.WriteByte('{')
				i += 2
				continue
			}
			rel := strings.IndexByte(msg[i+1:], '}')
			if rel < 0 {
				// Unbalanced brace: emit the remainder verbatim.
				b.WriteString(msg[i:])
				return b.String()
			}
			expr := msg[i+1 : i+1+rel]
			if v, err := s.evalInHook(expr, env); err != nil {
				b.WriteString("{error: " + err.Error() + "}")
			} else {
				b.WriteString(printer.Pr_str(v, false))
			}
			i += rel + 2
		case '}':
			if i+1 < len(msg) && msg[i+1] == '}' {
				b.WriteByte('}')
				i += 2
				continue
			}
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// truthy applies Lisp truthiness: everything except nil and boolean
// false is truthy.
func truthy(v types.MalType) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return true
}
