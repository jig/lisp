//go:build lispdebug

package debugadapter

import (
	"context"

	"github.com/jig/lisp/runtime"
)

// StepHook is the runtime.EvalHook installed by the DAP server. It is
// invoked once per EVAL iteration; it consults the session state to
// decide whether to pause execution.
type StepHook struct {
	st *state
}

// OnEval implements runtime.EvalHook.
func (h *StepHook) OnEval(ctx context.Context, ev runtime.EvalEvent) error {
	s := h.st
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.disconnect {
		return errDisconnected
	}

	t := runtime.ThreadFromContext(ctx)
	depth := 0
	var top *runtime.Frame
	if t != nil {
		depth = t.Depth()
		top = t.Top()
	}

	// Breakpoint check first: a breakpoint always wins over step mode.
	if s.matchBreakpoint(ev.Cursor) {
		s.pauseAndWait("breakpoint", "")
		return nil
	}

	// Don't pause inside library code: stop-on-entry and step modes
	// only fire on forms whose source file the client can actually
	// open. Otherwise VSCode tries to fetch the source via the
	// `source` request and errors out.
	if !s.isUserCode(ev.Cursor) {
		return nil
	}

	// EVAL's TCO loop calls OnEval repeatedly on the *same* frame for
	// constructs like do/let/if/fn-body. A "step" should only react to a
	// genuinely new call (push) or return to a different frame (pop), so
	// we ignore re-entries on the frame we were on at step time.
	sameFrame := top != nil && top == s.stepFrame

	switch s.mode {
	case modeStopOnEntry:
		s.pauseAndWait("entry", "")
	case modeStepIn:
		if sameFrame {
			break
		}
		s.pauseAndWait("step", "")
	case modeStepOver:
		if sameFrame {
			break
		}
		if depth <= s.targetDepth {
			s.pauseAndWait("step", "")
		}
	case modeStepOut:
		if depth < s.targetDepth {
			s.pauseAndWait("step", "")
		}
	default:
		// modeRunning, modePaused → keep running (paused is impossible
		// here because the goroutine is unblocked).
	}

	if s.disconnect {
		return errDisconnected
	}
	return nil
}

// errDisconnected is returned by OnEval to abort EVAL when the client
// disconnects.
var errDisconnected = &disconnectError{}

type disconnectError struct{}

func (*disconnectError) Error() string { return "DAP client disconnected" }
