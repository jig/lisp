//go:build lispdebug

package debugadapter

import (
	"context"
	"fmt"
	"os"

	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// traceEnabled is set by the LISP_DAP_TRACE environment variable. When
// set to a non-empty value the hook prints one line per OnEval call to
// stderr, useful when investigating step-mode bugs without a real DAP
// client. The check is per-process and read once at init time.
var traceEnabled = os.Getenv("LISP_DAP_TRACE") != ""

func tracef(format string, args ...interface{}) {
	if !traceEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "[dap] "+format+"\n", args...)
}

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

	// Atomic forms (Symbol, Number, String, Keyword, …) carry no
	// program structure the user would meaningfully step into. EVAL
	// pushes a frame for them so error stacks stay accurate, but the
	// debugger should not treat them as stops. Lists / Vectors /
	// HashMaps still flow through.
	switch ev.AST.(type) {
	case types.List, types.Vector, types.HashMap:
		// fall through
	default:
		return nil
	}

	t := runtime.ThreadFromContext(ctx)
	depth := 0
	var top *runtime.Frame
	if t != nil {
		depth = t.Depth()
		top = t.Top()
	}

	// Track the line of the previous OnEval before doing the BP check —
	// matchBreakpoint uses the *previous* line to detect a transition.
	prevLine := s.lastObservedLine
	if ev.Cursor != nil {
		s.lastObservedLine = ev.Cursor.BeginRow
	} else {
		s.lastObservedLine = 0
	}

	// Breakpoint check first: a breakpoint always wins over step mode.
	if s.matchBreakpoint(ev.Cursor, prevLine) {
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
	// We compare by frame ID rather than pointer because the GC may
	// reuse the memory of a popped frame.
	sameFrame := s.stepFrameID != 0 && runtime.FrameID(top) == s.stepFrameID

	if traceEnabled {
		mod := "<nil>"
		row := -1
		if ev.Cursor != nil {
			if ev.Cursor.Module != nil {
				mod = *ev.Cursor.Module
			}
			row = ev.Cursor.BeginRow
		}
		tracef("OnEval mode=%d depth=%d topID=%d stepFrameID=%d sameFrame=%v target=%d module=%s row=%d ast=%s",
			s.mode, depth, runtime.FrameID(top), s.stepFrameID, sameFrame, s.targetDepth, mod, row, printer.Pr_str(ev.AST, true))
	}

	switch s.mode {
	case modeStopOnEntry:
		s.pauseAndWait("entry", "")
	case modeStepIn:
		// step-in stops on the next list-form, even if it's a TCO
		// continuation of the same frame: that's exactly how the user
		// "enters" a fn body or a let body.
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
