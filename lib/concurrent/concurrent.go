package concurrent

import (
	"context"

	_ "embed"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/runtime"
	. "github.com/jig/lisp/types"
)

//go:embed header-concurrent.lisp
var headerConcurrent string

func HeaderConcurrent() string { return headerConcurrent }

func Load(env EnvType) {
	call.CallOverrideFN(env, "atom", func(a MalType) (MalType, error) { return &Atom{Val: a}, nil })
	call.CallOverrideFN(env, "new-atom", func(a *Atom) (MalType, error) { return nil, errors.New("atom cannot be deserialized") })
	call.CallOverrideFN(env, "atom?", func(a MalType) (MalType, error) { return Q[*Atom](a), nil })
	call.CallOverrideFN(env, "swap!", swap_BANG)
	call.CallOverrideFN(env, "reset!", reset_BANG)
	call.Call(env, future_call)
	call.Call(env, future_cancel)
	call.CallOverrideFN(env, "future-cancelled?", func(f *Future) (bool, error) { return f.Cancelled.Load(), nil })
	call.CallOverrideFN(env, "future-done?", func(f *Future) (bool, error) { return f.Done.Load(), nil })
	call.CallOverrideFN(env, "future?", func(f MalType) (bool, error) { return Q[*Future](f), nil })
	call.Call(env, new_future_call)

	// Documentation metadata for the atom builtins (see call.Doc). The
	// futures are exercised through the lisp-defined future macro, which
	// carries its own arglist.
	call.Doc(env, "atom", "[value]", "Creates a mutable, thread-safe atom holding value.")
	call.Doc(env, "atom?", "[x]", "Whether x is an atom.")
	call.Doc(env, "reset!", "[atom value]", "Sets the atom to value and returns it.")
	call.Doc(env, "swap!", "[atom f & args]", "Atomically sets the atom to (f current & args).")
	call.Doc(env, "future-call", "[fn]", "Runs fn on a new goroutine, returning a future for its result.")
	call.Doc(env, "future-cancel", "[future]", "Requests cancellation of a running future.")
	call.Doc(env, "future-cancelled?", "[future]", "Whether the future was cancelled.")
	call.Doc(env, "future-done?", "[future]", "Whether the future has finished.")
	call.Doc(env, "future?", "[x]", "Whether x is a future.")
}

func future_call(ctx context.Context, f MalFunc) (*Future, error) {
	return NewFuture(ctx, f), nil
}

func future_cancel(f *Future) (bool, error) {
	return f.Cancel(), nil
}

// Atom functions
func reset_BANG(atomRef, value MalType) (MalType, error) {
	if !Q[*Atom](atomRef) {
		return nil, errors.New("reset! called with non-atom")
	}
	atm := atomRef.(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	atm.Set(value)
	return value, nil
}

func swap_BANG(ctx context.Context, a ...MalType) (MalType, error) {
	if !Q[*Atom](a[0]) {
		return nil, errors.New("swap! called with non-atom")
	}
	atm := a[0].(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	args := []MalType{atm.Val}
	f := a[1]
	args = append(args, a[2:]...)
	res, e := Apply(ctx, f, args)
	if e != nil {
		return nil, e
	}
	atm.Set(res)
	return res, nil
}

// Atoms
type Atom struct {
	Mutex  sync.RWMutex
	Val    MalType
	Meta   MalType
	Cursor *Position
}

func (a *Atom) Type() string {
	return "atom"
}

func (a *Atom) Set(val MalType) MalType {
	a.Val = val
	return a
}

func (a *Atom) Deref(_ context.Context) (MalType, error) {
	a.Mutex.RLock()
	defer a.Mutex.RUnlock()
	return a.Val, nil
}

func (a *Atom) LispPrint(pr_str func(MalType, bool) string) string {
	return "«atom " + pr_str(a.Val, true) + "»"
}

// Future
type Future struct {
	ValChan    chan MalType
	ErrChan    chan error
	CancelFunc context.CancelFunc
	// Done and Cancelled are read from other goroutines (future-done? /
	// future-cancelled?) while the future's goroutine and Cancel write
	// them, so they are atomic. Read with .Load(), not as bare bools.
	Done      atomic.Bool
	Cancelled atomic.Bool

	Fn     MalFunc
	Meta   MalType
	Cursor *Position
}

func new_future_call(fn MalFunc) (*Future, error) {
	return nil, errors.New("atom cannot be deserialized")
}

func NewFuture(ctx context.Context, fn MalFunc) *Future {
	ctx, cancel := context.WithCancel(ctx)
	f := &Future{
		ValChan:    make(chan MalType, 1),
		ErrChan:    make(chan error, 1),
		CancelFunc: cancel,
		Fn:         fn,
	}
	go func() {
		defer func() { f.Done.Store(true) }()
		// The future's goroutine must not share the spawner's debug
		// thread: see runtime.DetachThread. No-op in release builds.
		res, err := Apply(runtime.DetachThread(ctx), fn, nil)
		if err != nil {
			f.ErrChan <- err
			return
		}
		f.ValChan <- res
	}()

	return f
}

func (f *Future) Cancel() bool {
	// CompareAndSwap makes the check-then-act atomic: whoever flips Done
	// false→true owns the cancellation. If the goroutine already
	// finished (Done true), the swap fails and we leave Cancelled false.
	if f.Done.CompareAndSwap(false, true) {
		f.Cancelled.Store(true)
		f.CancelFunc()
	}
	return f.Cancelled.Load()
}

func (f *Future) Deref(ctx context.Context) (MalType, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("timeout while dereferencing future")
	case err := <-f.ErrChan:
		f.ErrChan <- err
		return nil, err
	case res := <-f.ValChan:
		f.ValChan <- res
		return res, nil
	}
}

func (f *Future) LispPrint(_Pr_str func(MalType, bool) string) string {
	return "«futur-call " + _Pr_str(f.Fn.Exp, true) + "»"
}

func (f *Future) Type() string {
	return "future-call"
}

func (f *Future) GetPosition() *Position {
	return f.Cursor
}
