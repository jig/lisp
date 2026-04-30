//go:build lispdebug

package debugadapter

import (
	"path/filepath"
	"sync"

	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// mode is the execution mode of the debuggee.
type mode int

const (
	modeRunning mode = iota
	modeStopOnEntry
	modeStepOver
	modeStepIn
	modeStepOut
	modePaused
)

// varRefKind is the discriminator for a variables reference entry.
type varRefKind int

const (
	varRefScopeLocals varRefKind = iota
	varRefValue
)

type varRef struct {
	kind     varRefKind
	frameIdx int           // for scope-locals
	value    types.MalType // for value
}

// state holds the live debug session: mode, breakpoints, the thread
// being debugged, and the variables reference table consulted by
// `variables` requests.
//
// The mutex protects every field. The condition variable is used by the
// EVAL goroutine (in the hook) to block while the session is paused; the
// server goroutine signals it when the client asks to resume.
type state struct {
	mu   sync.Mutex
	cond *sync.Cond

	mode        mode
	targetDepth int            // for stepOver (entered at this depth) / stepOut
	stepFrame   *runtime.Frame // top frame at the moment the step request arrived;
	//                            EVAL's TCO loop re-enters OnEval on the same frame,
	//                            so we use this to distinguish a "new call" from a
	//                            "next iteration of the same call".

	breakpoints map[string]map[int]bool // abs path → line → set

	thread *runtime.Thread

	server *Server

	varRefs    map[int]varRef
	nextVarRef int

	stopOnEntry bool
	disconnect  bool
	exited      bool
}

func newState(s *Server) *state {
	st := &state{
		mode:        modeRunning,
		breakpoints: map[string]map[int]bool{},
		thread:      runtime.NewThread(),
		server:      s,
		varRefs:     map[int]varRef{},
	}
	st.cond = sync.NewCond(&st.mu)
	return st
}

// setBreakpoints replaces all breakpoints for src.Path with lines.
// Returns the verified Breakpoints array to send back to the client.
func (s *state) setBreakpoints(src Source, requested []SourceBreakpoint) []Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs := absolutize(src.Path)
	if abs == "" {
		// Without a path we cannot match; mark all as unverified.
		out := make([]Breakpoint, len(requested))
		for i, bp := range requested {
			out[i] = Breakpoint{Verified: false, Line: bp.Line, Source: src}
		}
		return out
	}
	lines := map[int]bool{}
	out := make([]Breakpoint, len(requested))
	for i, bp := range requested {
		lines[bp.Line] = true
		out[i] = Breakpoint{Verified: true, Line: bp.Line, Source: Source{Name: src.Name, Path: abs}}
	}
	s.breakpoints[abs] = lines
	return out
}

// isUserCode reports whether the cursor points at a file the client
// could plausibly know about. A Module is considered "user code" only
// when it has been registered with runtime.Modules — that means a real
// filesystem path was associated with it (e.g. via command.ExecuteFile
// or command.startDAP). Library headers (header-load-file, etc.) live
// in the binary as embedded strings; pausing inside them would point
// the client at non-existent files and trigger "Could not load source"
// errors in VSCode.
func (s *state) isUserCode(cursor *types.Position) bool {
	if cursor == nil || cursor.Module == nil {
		return false
	}
	_, ok := runtime.Modules.Lookup(*cursor.Module)
	return ok
}

// matchBreakpoint reports whether cursor is on a line with a registered
// breakpoint.
func (s *state) matchBreakpoint(cursor *types.Position) bool {
	if cursor == nil || cursor.Module == nil {
		return false
	}
	resolved := runtime.Modules.Resolve(*cursor.Module)
	abs := absolutize(resolved)
	if abs == "" {
		return false
	}
	lines := s.breakpoints[abs]
	if lines == nil {
		return false
	}
	return lines[cursor.BeginRow]
}

// pauseAndWait sends a `stopped` event and blocks until the client
// resumes. Caller must hold s.mu.
func (s *state) pauseAndWait(reason, description string) {
	s.mode = modePaused
	s.varRefs = map[int]varRef{}
	s.nextVarRef = 0
	body := StoppedEventBody{
		Reason:            reason,
		Description:       description,
		ThreadID:          1,
		AllThreadsStopped: true,
	}
	go s.server.sendEvent("stopped", body)
	for s.mode == modePaused && !s.disconnect {
		s.cond.Wait()
	}
}

// resume wakes the EVAL goroutine after switching to the given mode.
// stepFrame snapshots the top frame so the hook can ignore TCO loop
// re-entries on the same frame and only react to a real new call.
// Caller must hold s.mu.
func (s *state) resume(m mode, depth int) {
	s.mode = m
	s.targetDepth = depth
	s.stepFrame = s.thread.Top()
	s.cond.Signal()
	go s.server.sendEvent("continued", ContinuedEventBody{
		ThreadID:            1,
		AllThreadsContinued: true,
	})
}

// registerVarRef stores v in the references table and returns a fresh
// reference id. Caller must hold s.mu. Returns 0 for non-composite
// values that have no expandable children.
func (s *state) registerVarRef(v varRef) int {
	s.nextVarRef++
	s.varRefs[s.nextVarRef] = v
	return s.nextVarRef
}

func absolutize(p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
