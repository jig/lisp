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
	if t != nil {
		depth = t.Depth()
	}

	// Breakpoint check first: a breakpoint always wins over step mode.
	if s.matchBreakpoint(ev.Cursor) {
		s.pauseAndWait("breakpoint", "")
		return nil
	}

	switch s.mode {
	case modeStopOnEntry:
		s.pauseAndWait("entry", "")
	case modeStepIn:
		s.pauseAndWait("step", "")
	case modeStepOver:
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
