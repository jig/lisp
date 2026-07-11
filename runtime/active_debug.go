//go:build lispdebug

package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

// Enabled reports whether hook dispatching is compiled in.
// Always true in debug builds (`-tags lispdebug`).
const Enabled = true

// Hook is the active EvalHook. nil disables hooking.
//
// Set this once before kicking off EVAL goroutines. Concurrent reads are
// safe; concurrent writes are not synchronised — callers must coordinate.
var Hook EvalHook

// Dispatch invokes the active hook if any. EVAL calls this once per
// iteration when Enabled is true.
func Dispatch(ctx context.Context, ast types.MalType, env types.EnvType, cursor *types.Position) error {
	h := Hook
	if h == nil {
		return nil
	}
	return h.OnEval(ctx, EvalEvent{AST: ast, Env: env, Cursor: cursor})
}

// DispatchError notifies the active hook of an error at its raise point,
// if the hook implements ErrorHook. EVAL calls this from the innermost
// frame the error passes through (stack still intact). Returns nothing:
// an ErrorHook observes, it does not change the propagating error.
func DispatchError(ctx context.Context, err error, ast types.MalType, env types.EnvType, cursor *types.Position, functionName string) {
	eh, ok := Hook.(ErrorHook)
	if !ok {
		return
	}
	eh.OnError(ctx, ErrorEvent{
		Err:          err,
		AST:          ast,
		Env:          env,
		Cursor:       cursor,
		FunctionName: functionName,
	})
}

// PrintEvalHook reproduces the legacy DEBUG-EVAL printing behaviour:
// when the symbol DEBUG-EVAL is bound to true in the active env, it prints
// the source position followed by the form being evaluated.
type PrintEvalHook struct{}

// OnEval implements EvalHook.
func (PrintEvalHook) OnEval(_ context.Context, ev EvalEvent) error {
	if ev.Env == nil {
		return nil
	}
	dbg, err := ev.Env.Get(types.Symbol{Val: "DEBUG-EVAL"})
	if err != nil {
		return nil
	}
	b, ok := dbg.(bool)
	if !ok || !b {
		return nil
	}
	if ev.Cursor != nil {
		fmt.Printf("\033[38;5;208m%s\033[0m: %s\n", ev.Cursor, printer.Pr_str(ev.AST, true))
	}
	return nil
}

// Frame is a single entry in the live execution stack.
//
// EVAL pushes a Frame on entry and mutates AST/Env/Cursor as the TCO loop
// progresses. A Frame is popped when the EVAL call returns.
//
// ID is a process-wide monotonically increasing identifier assigned by
// MakeFrame. The DAP server uses it (instead of a *Frame pointer) to
// recognise "same call" vs "new call" — pointer identity is unreliable
// because the GC can reuse the memory of a popped frame.
type Frame struct {
	ID           int64
	FunctionName string
	AST          types.MalType
	Env          types.EnvType
	Cursor       *types.Position
}

var nextFrameID atomic.Int64

// Thread is the live call stack of a single Lisp evaluation goroutine.
//
// Operations are safe for concurrent use, but a Thread is conceptually
// owned by one goroutine (the one running EVAL). External readers
// (e.g. the DAP server) snapshot it under the mutex.
type Thread struct {
	mu     sync.Mutex
	frames []*Frame

	// lastResult holds the value of the most recently completed EVAL
	// frame on this thread. The debugger reads it after a step-over or
	// step-out to show the stepped form's return value: because a form's
	// own EVAL returns after all its sub-forms, the outermost stepped
	// form is the last to record, so this is exactly its result.
	lastResult    types.MalType
	hasLastResult bool
}

// NewThread returns an empty Thread.
func NewThread() *Thread { return &Thread{} }

// MakeFrame allocates a Frame. Helper used from EVAL so the release-build
// stub can hide the Frame layout entirely.
func MakeFrame(functionName string, ast types.MalType, env types.EnvType, cursor *types.Position) *Frame {
	return &Frame{
		ID:           nextFrameID.Add(1),
		FunctionName: functionName,
		AST:          ast,
		Env:          env,
		Cursor:       cursor,
	}
}

// FrameID returns the unique identifier of a Frame, or 0 for nil. The
// release-build stub also returns 0; consumers can compare IDs
// portably across builds.
func FrameID(f *Frame) int64 {
	if f == nil {
		return 0
	}
	return f.ID
}

// UpdateFrame mutates an existing Frame in place. Used by the TCO loop in
// EVAL to keep the top frame in sync with the form currently being
// evaluated.
func UpdateFrame(f *Frame, ast types.MalType, env types.EnvType, cursor *types.Position) {
	if f == nil {
		return
	}
	f.AST = ast
	f.Env = env
	f.Cursor = cursor
}

// PushFrame pushes a Frame onto the Thread carried by ctx, if any.
// Reports whether a Thread was found.
func PushFrame(ctx context.Context, f *Frame) bool {
	t := ThreadFromContext(ctx)
	if t == nil {
		return false
	}
	t.Push(f)
	return true
}

// PopFrame pops the top Frame from the Thread carried by ctx, if any.
func PopFrame(ctx context.Context) {
	if t := ThreadFromContext(ctx); t != nil {
		t.Pop()
	}
}

// Push appends a Frame to the stack.
func (t *Thread) Push(f *Frame) {
	t.mu.Lock()
	t.frames = append(t.frames, f)
	t.mu.Unlock()
}

// Pop removes the top frame. Returns nil if empty.
func (t *Thread) Pop() *Frame {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(t.frames)
	if n == 0 {
		return nil
	}
	f := t.frames[n-1]
	t.frames = t.frames[:n-1]
	return f
}

// Depth returns the number of frames on the stack.
func (t *Thread) Depth() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.frames)
}

// Top returns the deepest frame (or nil if empty). The pointer is the
// live Frame; readers should not mutate its fields. Use Snapshot for a
// stable copy.
func (t *Thread) Top() *Frame {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(t.frames)
	if n == 0 {
		return nil
	}
	return t.frames[n-1]
}

// Snapshot returns a value copy of all frames, deepest last. The copy is
// stable for the caller even if EVAL keeps mutating its frames.
func (t *Thread) Snapshot() []Frame {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Frame, len(t.frames))
	for i, f := range t.frames {
		out[i] = *f
	}
	return out
}

// RecordResult stores res as the thread's most-recently-completed value.
func (t *Thread) RecordResult(res types.MalType) {
	t.mu.Lock()
	t.lastResult = res
	t.hasLastResult = true
	t.mu.Unlock()
}

// LastResult returns the most-recently-recorded value and whether one is
// available.
func (t *Thread) LastResult() (types.MalType, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastResult, t.hasLastResult
}

// ResetLastResult forgets any recorded result. The debugger calls it when
// resuming so a stale value is not shown at the next stop.
func (t *Thread) ResetLastResult() {
	t.mu.Lock()
	t.lastResult = nil
	t.hasLastResult = false
	t.mu.Unlock()
}

// RecordResult stores res on the Thread carried by ctx, if any. EVAL
// calls it as each frame completes successfully.
func RecordResult(ctx context.Context, res types.MalType) {
	if t := ThreadFromContext(ctx); t != nil {
		t.RecordResult(res)
	}
}

type ctxKey struct{}

var threadKey ctxKey

// WithThread returns a context carrying the given Thread. EVAL calls
// invoked with this context push/pop frames on it.
func WithThread(ctx context.Context, t *Thread) context.Context {
	return context.WithValue(ctx, threadKey, t)
}

// ThreadFromContext returns the Thread carried by ctx, or nil if absent.
func ThreadFromContext(ctx context.Context) *Thread {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(threadKey).(*Thread)
	return t
}

// DetachThread returns a context that carries no Thread. Futures use it
// for their goroutines: inheriting the spawning goroutine's Thread would
// let two goroutines push/pop frames on the same live stack, corrupting
// the debugger's view, and would allow the debugger to pause a goroutine
// the client does not know about.
func DetachThread(ctx context.Context) context.Context {
	if ThreadFromContext(ctx) == nil {
		return ctx
	}
	return context.WithValue(ctx, threadKey, (*Thread)(nil))
}

// ModuleResolver maps Lisp module names (as carried in Position.Module)
// to absolute filesystem paths. The DAP server consumes this to populate
// `source.path` on stack frames and to match `setBreakpoints` requests
// against incoming Cursors.
type ModuleResolver struct {
	mu    sync.RWMutex
	paths map[string]string
}

// Register associates a module identifier with an absolute path.
// Both arguments are stored verbatim; the caller is responsible for
// resolving relative paths beforehand.
func (r *ModuleResolver) Register(module, absPath string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paths == nil {
		r.paths = map[string]string{}
	}
	r.paths[module] = absPath
}

// Resolve returns the registered absolute path for module, or the
// module name itself when no mapping exists. This makes the resolver
// safe to consult unconditionally.
func (r *ModuleResolver) Resolve(module string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.paths[module]; ok {
		return p
	}
	return module
}

// Lookup returns the registered absolute path for module and a boolean
// reporting whether a mapping was found.
func (r *ModuleResolver) Lookup(module string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.paths[module]
	return p, ok
}

// Modules is the global module-to-path resolver.
var Modules = &ModuleResolver{}
