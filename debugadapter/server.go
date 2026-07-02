//go:build lispdebug

package debugadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// EvalFunc evaluates the given source under the given context. The DAP
// server invokes it once `configurationDone` arrives. The function
// should respect the context for cancellation.
type EvalFunc func(ctx context.Context, env types.EnvType) error

// Server is the DAP server. One instance handles one debug session.
type Server struct {
	t *Transport

	state *state

	seq atomic.Int64

	cfgDone chan struct{}

	evalFn   EvalFunc
	evalEnv  types.EnvType
	evalDone chan error

	closeOnce sync.Once
}

// NewServer constructs a server that will drive `eval` once the client
// finishes configuration. `env` is passed to eval; the caller has
// already populated it with the desired libraries.
func NewServer(t *Transport, eval EvalFunc, env types.EnvType) *Server {
	s := &Server{
		t:        t,
		cfgDone:  make(chan struct{}),
		evalFn:   eval,
		evalEnv:  env,
		evalDone: make(chan error, 1),
	}
	s.state = newState(s)
	return s
}

// StreamOutput reads r until EOF and forwards each chunk to the client
// as an `output` event with the given category ("stdout" or "stderr").
// Run it on its own goroutine, typically fed by an os.Pipe that replaces
// the process's real stdout while the debuggee runs: the interpreter's
// println/prn write to os.Stdout, which in stdio DAP mode is the
// protocol channel itself — raw prints there would corrupt the message
// framing and never reach the client's Debug Console.
func (s *Server) StreamOutput(r io.Reader, category string) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.sendEvent("output", OutputEventBody{Category: category, Output: string(buf[:n])})
		}
		if err != nil {
			return
		}
	}
}

// Run blocks reading and dispatching DAP messages until the client
// disconnects or the debuggee exits.
func (s *Server) Run(ctx context.Context) error {
	// Install our hook for the duration of this session.
	prevHook := runtime.Hook
	runtime.Hook = &StepHook{st: s.state}
	defer func() { runtime.Hook = prevHook }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go s.runEval(ctx)

	for {
		raw, err := s.t.ReadMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.markDisconnect()
				<-s.evalDone
				return nil
			}
			s.markDisconnect()
			return err
		}
		var hdr Message
		if err := json.Unmarshal(raw, &hdr); err != nil {
			continue
		}
		if hdr.Type != "request" {
			continue
		}
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}
		s.dispatch(ctx, &req)
		if req.Command == "disconnect" || req.Command == "terminate" {
			s.markDisconnect()
			<-s.evalDone
			return nil
		}
	}
}

// runEval waits for `configurationDone`, then runs the program.
func (s *Server) runEval(ctx context.Context) {
	select {
	case <-s.cfgDone:
	case <-ctx.Done():
		s.evalDone <- ctx.Err()
		return
	}
	evalCtx := runtime.WithThread(ctx, s.state.thread)
	err := s.evalFn(evalCtx, s.evalEnv)

	if err != nil && !errors.Is(err, errDisconnected) {
		s.sendEvent("output", OutputEventBody{Category: "stderr", Output: err.Error() + "\n"})
	}
	exitCode := 0
	if err != nil && !errors.Is(err, errDisconnected) {
		exitCode = 1
	}
	s.sendEvent("exited", ExitedEventBody{ExitCode: exitCode})
	s.sendEvent("terminated", struct{}{})
	s.evalDone <- err
}

// markDisconnect signals the EVAL goroutine to abort at the next hook.
func (s *Server) markDisconnect() {
	s.state.mu.Lock()
	s.state.disconnect = true
	s.state.cond.Broadcast()
	s.state.mu.Unlock()
}

// dispatch routes a request to its handler. Each handler builds and
// sends the response.
func (s *Server) dispatch(_ context.Context, req *Request) {
	switch req.Command {
	case "initialize":
		s.respond(req, true, "", Capabilities{
			SupportsConfigurationDoneRequest: true,
			SupportsTerminateRequest:         true,
			SupportsEvaluateForHovers:        true,
		})
		s.sendEvent("initialized", struct{}{})
	case "launch":
		var args LaunchArguments
		_ = json.Unmarshal(req.Arguments, &args)
		if args.Cwd != "" {
			if err := os.Chdir(args.Cwd); err != nil {
				s.respond(req, false, fmt.Sprintf("launch: cannot chdir to %q: %v", args.Cwd, err), nil)
				return
			}
		}
		s.state.mu.Lock()
		if args.StopOnEntry {
			s.state.stopOnEntry = true
			s.state.mode = modeStopOnEntry
		}
		s.state.mu.Unlock()
		s.respond(req, true, "", nil)
	case "setBreakpoints":
		var args SetBreakpointsArguments
		_ = json.Unmarshal(req.Arguments, &args)
		bps := s.state.setBreakpoints(args.Source, args.Breakpoints)
		s.respond(req, true, "", map[string]interface{}{"breakpoints": bps})
	case "configurationDone":
		s.respond(req, true, "", nil)
		select {
		case <-s.cfgDone:
		default:
			close(s.cfgDone)
		}
	case "threads":
		s.respond(req, true, "", map[string]interface{}{
			"threads": []Thread{{ID: 1, Name: "lisp"}},
		})
	case "stackTrace":
		s.handleStackTrace(req)
	case "scopes":
		s.handleScopes(req)
	case "variables":
		s.handleVariables(req)
	case "evaluate":
		s.handleEvaluate(req)
	// For the four resume-style requests the response is written while
	// still holding the state mutex: the EVAL goroutine is blocked on the
	// condition variable and cannot wake (and emit the next `stopped`
	// event) until the mutex is released, which guarantees the client
	// always observes response → stopped in that order.
	case "continue":
		s.state.mu.Lock()
		s.respond(req, true, "", map[string]interface{}{"allThreadsContinued": true})
		s.state.resume(modeRunning, 0)
		s.state.mu.Unlock()
	case "next":
		s.state.mu.Lock()
		s.respond(req, true, "", nil)
		s.state.resume(modeStepOver, s.state.thread.Depth())
		s.state.mu.Unlock()
	case "stepIn":
		s.state.mu.Lock()
		s.respond(req, true, "", nil)
		s.state.resume(modeStepIn, 0)
		s.state.mu.Unlock()
	case "stepOut":
		s.state.mu.Lock()
		s.respond(req, true, "", nil)
		s.state.resume(modeStepOut, s.state.thread.Depth())
		s.state.mu.Unlock()
	case "pause":
		// Cooperative pause: flip mode so the next OnEval blocks. The
		// goroutine is currently running; OnEval will pick this up.
		s.state.mu.Lock()
		s.state.mode = modeStepIn
		s.state.mu.Unlock()
		s.respond(req, true, "", nil)
	case "disconnect":
		s.respond(req, true, "", nil)
	case "terminate":
		s.respond(req, true, "", nil)
	default:
		s.respond(req, false, "unsupported command: "+req.Command, nil)
	}
}

func (s *Server) handleStackTrace(req *Request) {
	frames := s.state.thread.Snapshot()
	out := make([]StackFrame, 0, len(frames))
	for i := len(frames) - 1; i >= 0; i-- {
		f := frames[i]
		var src Source
		line := 0
		col := 0
		if f.Cursor != nil {
			line = f.Cursor.BeginRow
			col = f.Cursor.BeginCol
			if f.Cursor.Module != nil {
				// Only emit a Source when we know a real filesystem
				// path. Otherwise the client would fire the `source`
				// request for an embedded library header.
				if path, ok := runtime.Modules.Lookup(*f.Cursor.Module); ok {
					src = Source{Name: *f.Cursor.Module, Path: path}
				}
			}
		}
		name := f.FunctionName
		if name == "" {
			name = "<eval>"
		}
		out = append(out, StackFrame{
			ID:     len(frames) - 1 - i,
			Name:   name,
			Line:   line,
			Column: col,
			Source: src,
		})
	}
	s.respond(req, true, "", map[string]interface{}{
		"stackFrames": out,
		"totalFrames": len(out),
	})
}

// handleEvaluate serves `evaluate` requests: Debug Console input, watch
// expressions and hovers. The expression is evaluated in the environment
// of the requested stack frame (top frame by default) so local bindings
// resolve as the user expects.
//
// The evaluation runs on the server goroutine with a fresh context that
// carries no runtime.Thread: EVAL therefore pushes no frames onto the
// debuggee's stack (stepOver depths stay intact), and the hook ignores
// the module-less cursor, so the paused session is not disturbed. The
// only state the hook touches is lastObservedLine, which is saved and
// restored so the breakpoint line-transition detection cannot re-fire
// because of a console evaluation.
func (s *Server) handleEvaluate(req *Request) {
	var args EvaluateArguments
	_ = json.Unmarshal(req.Arguments, &args)

	s.state.mu.Lock()
	frames := s.state.thread.Snapshot()
	env := s.evalEnv
	// FrameID was assigned in handleStackTrace as `len(frames)-1-i`
	// (top frame → 0). Resolve it back to a slice index.
	idx := len(frames) - 1 - args.FrameID
	if idx >= 0 && idx < len(frames) && frames[idx].Env != nil {
		env = frames[idx].Env
	}
	savedLine := s.state.lastObservedLine
	s.state.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var res types.MalType
	ast, err := lisp.READ(args.Expression, types.NewAnonymousCursorHere(1, 1), env)
	if err == nil {
		res, err = lisp.EVAL(ctx, ast, env)
	}

	s.state.mu.Lock()
	s.state.lastObservedLine = savedLine
	ref := 0
	if err == nil {
		switch res.(type) {
		case types.List, types.Vector, types.HashMap, types.Set:
			ref = s.state.registerVarRef(varRef{kind: varRefValue, value: res})
		}
	}
	s.state.mu.Unlock()

	if err != nil {
		s.respond(req, false, err.Error(), nil)
		return
	}
	s.respond(req, true, "", map[string]interface{}{
		"result":             printer.Pr_str(res, true),
		"type":               fmt.Sprintf("%T", res),
		"variablesReference": ref,
	})
}

func (s *Server) handleScopes(req *Request) {
	var args ScopesArguments
	_ = json.Unmarshal(req.Arguments, &args)

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	frames := s.state.thread.Snapshot()
	// args.FrameID was assigned in handleStackTrace as `len(frames)-1-i`
	// (top frame → 0). Resolve it back to a slice index.
	idx := len(frames) - 1 - args.FrameID
	if idx < 0 || idx >= len(frames) {
		s.respond(req, true, "", map[string]interface{}{"scopes": []Scope{}})
		return
	}
	ref := s.state.registerVarRef(varRef{kind: varRefScopeLocals, frameIdx: idx})
	scopes := []Scope{{Name: "Locals", VariablesReference: ref, Expensive: false}}
	s.respond(req, true, "", map[string]interface{}{"scopes": scopes})
}

func (s *Server) handleVariables(req *Request) {
	var args VariablesArguments
	_ = json.Unmarshal(req.Arguments, &args)

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	entry, ok := s.state.varRefs[args.VariablesReference]
	if !ok {
		s.respond(req, true, "", map[string]interface{}{"variables": []Variable{}})
		return
	}
	var vars []Variable
	switch entry.kind {
	case varRefScopeLocals:
		vars = s.localsForFrame(entry.frameIdx)
	case varRefValue:
		vars = s.childrenOf(entry.value)
	}
	s.respond(req, true, "", map[string]interface{}{"variables": vars})
}

// localsForFrame returns the bindings local to the frame's environment
// (not the outer chain). We surface only the immediate scope to keep
// the variables view manageable.
func (s *Server) localsForFrame(frameIdx int) []Variable {
	frames := s.state.thread.Snapshot()
	if frameIdx < 0 || frameIdx >= len(frames) {
		return nil
	}
	env := frames[frameIdx].Env
	if env == nil {
		return nil
	}
	// Use the prefix-completion API to enumerate local symbols. With an
	// empty prefix it returns symbols from the current scope (and outers
	// after that — we slice on the first scope by reading via Get).
	syms := env.Symbols(nil, "")
	seen := map[string]bool{}
	out := make([]Variable, 0, len(syms))
	for _, r := range syms {
		name := string(r)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		v, err := env.Get(types.Symbol{Val: name})
		if err != nil {
			continue
		}
		out = append(out, s.formatVariable(name, v))
	}
	return out
}

func (s *Server) childrenOf(v types.MalType) []Variable {
	switch v := v.(type) {
	case types.List:
		out := make([]Variable, len(v.Val))
		for i, e := range v.Val {
			out[i] = s.formatVariable(fmt.Sprintf("[%d]", i), e)
		}
		return out
	case types.Vector:
		out := make([]Variable, len(v.Val))
		for i, e := range v.Val {
			out[i] = s.formatVariable(fmt.Sprintf("[%d]", i), e)
		}
		return out
	case types.HashMap:
		out := make([]Variable, 0, len(v.Val))
		for k, val := range v.Val {
			displayKey := k
			if strings.HasPrefix(k, "ʞ") {
				displayKey = ":" + strings.TrimPrefix(k, "ʞ")
			}
			out = append(out, s.formatVariable(displayKey, val))
		}
		return out
	case types.Set:
		out := make([]Variable, 0, len(v.Val))
		for k := range v.Val {
			out = append(out, s.formatVariable(k, k))
		}
		return out
	}
	return nil
}

// formatVariable produces a DAP Variable for the given name/value pair.
// Composite values get a fresh variablesReference so the client can
// expand them lazily.
func (s *Server) formatVariable(name string, v types.MalType) Variable {
	value := printer.Pr_str(v, true)
	typeName := fmt.Sprintf("%T", v)
	ref := 0
	switch v.(type) {
	case types.List, types.Vector, types.HashMap, types.Set:
		ref = s.state.registerVarRef(varRef{kind: varRefValue, value: v})
	}
	return Variable{Name: name, Value: value, Type: typeName, VariablesReference: ref}
}

// respond marshals a Response and writes it.
func (s *Server) respond(req *Request, success bool, msg string, body interface{}) {
	resp := Response{
		Message:    Message{Seq: int(s.seq.Add(1)), Type: "response"},
		RequestSeq: req.Seq,
		Success:    success,
		Command:    req.Command,
		Message_:   msg,
		Body:       body,
	}
	if err := s.t.WriteMessage(resp); err != nil {
		// best-effort; the client will likely time out
		_ = err
	}
}

// sendEvent is the main goroutine-friendly event sender used by the
// state to notify the client of pause/resume/output.
func (s *Server) sendEvent(name string, body interface{}) {
	ev := Event{
		Message: Message{Seq: int(s.seq.Add(1)), Type: "event"},
		Event:   name,
		Body:    body,
	}
	_ = s.t.WriteMessage(ev)
}
