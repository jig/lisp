//go:build lispdebug

package debugadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// fullEnv builds an env that mirrors what cmd/lisp loads, so load-file
// works inside the debugger session.
func fullEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	concurrent.Load(ns)
	ns.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return lisp.EVAL(ctx, a[0], ns)
	}})
	ns.Set(types.Symbol{Val: "*ARGV*"}, types.List{})
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, ns, core.HeaderBasic(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderBasic: %v", err)
	}
	if _, err := lisp.REPL(ctx, ns, core.HeaderLoadFile(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderLoadFile: %v", err)
	}
	if _, err := lisp.REPL(ctx, ns, concurrent.HeaderConcurrent(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderConcurrent: %v", err)
	}
	return ns
}

// stoppedAt returns the line reported by the most recent stackTrace
// after a `stopped` event arrives.
func stoppedAt(t *testing.T, c *Transport, seq int) int {
	t.Helper()
	readUntil(t, c, func(m map[string]interface{}) bool {
		return m["event"] == "stopped"
	})
	sendRequest(t, c, seq, "stackTrace", map[string]interface{}{"threadId": 1, "startFrame": 0, "levels": 20})
	resp := readUntil(t, c, func(m map[string]interface{}) bool {
		return m["type"] == "response" && m["command"] == "stackTrace"
	})
	body := resp["body"].(map[string]interface{})
	frames := body["stackFrames"].([]interface{})
	if len(frames) == 0 {
		t.Fatal("stackTrace returned no frames")
	}
	top := frames[0].(map[string]interface{})
	line, _ := top["line"].(float64)
	return int(line)
}

// TestServer_StepOverThroughLoadFile reproduces the user-reported flow:
// stop on a breakpoint inside a (do (println …) (println …) …) loaded
// via load-file, F10 should land on each subsequent println in turn.
func TestServer_StepOverThroughLoadFile(t *testing.T) {
	if os.Getenv("LISP_DAP_TRACE") == "" {
		// auto-enable trace for this test so failures are diagnosable
		t.Setenv("LISP_DAP_TRACE", "1")
		traceEnabled = true
		t.Cleanup(func() { traceEnabled = false })
	}

	tmp := t.TempDir()
	script := filepath.Join(tmp, "step.lisp")
	src := "(do\n" +
		"    (println 1)\n" +
		"    (println 2)\n" +
		"    (println 3))\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()

	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}

	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})

	// Set a breakpoint at line 2 (the (println 1) form).
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 2}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})

	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// 1) breakpoint stops at line 2
	if got := stoppedAt(t, client, 5); got != 2 {
		t.Errorf("first stop: expected line 2, got %d", got)
	}

	// 2) step over → line 3
	sendRequest(t, client, 6, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "next" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 7); got != 3 {
		t.Errorf("after first next: expected line 3, got %d", got)
	}

	// 3) step over → line 4
	sendRequest(t, client, 8, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "next" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 9); got != 4 {
		t.Errorf("after second next: expected line 4, got %d", got)
	}

	// 4) continue → terminated
	sendRequest(t, client, 10, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "continue" && m["type"] == "response"
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "terminated"
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 11, "disconnect", nil)
}

// TestServer_StepInSkipsAtoms verifies F11 (step in) does not pause on
// atomic sub-forms (the head Symbol of a call, literal arguments). One
// F11 should land directly on the next list-form to evaluate.
func TestServer_StepInSkipsAtoms(t *testing.T) {
	if os.Getenv("LISP_DAP_TRACE") == "" {
		t.Setenv("LISP_DAP_TRACE", "1")
		traceEnabled = true
		t.Cleanup(func() { traceEnabled = false })
	}

	tmp := t.TempDir()
	script := filepath.Join(tmp, "stepin.lisp")
	src := "(do\n" +
		"    (println 1)\n" +
		"    (println 2)\n" +
		"    (println 3))\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}

	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 2}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// First stop: breakpoint at (println 1).
	if got := stoppedAt(t, client, 5); got != 2 {
		t.Errorf("first stop: expected line 2, got %d", got)
	}

	// step in → should land directly on (println 2) (line 3), not on
	// the head Symbol or the integer 1 of (println 1).
	sendRequest(t, client, 6, "stepIn", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "stepIn" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 7); got != 3 {
		t.Errorf("after step in: expected line 3, got %d", got)
	}

	// step in → (println 3)
	sendRequest(t, client, 8, "stepIn", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "stepIn" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 9); got != 4 {
		t.Errorf("after second step in: expected line 4, got %d", got)
	}

	sendRequest(t, client, 10, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "continue" && m["type"] == "response"
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "terminated"
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 11, "disconnect", nil)
}

// TestStepInto_OnDoForm mimics the exact user flow (launch config uses
// stopOnEntry:true):
// 1. Breakpoint at the (do ...) line → one single stop at line 1
// 2. F11 (stepIn) → one press goes into the do body (line 2)
// 3. F10 (next) → walks the siblings within the do line by line
func TestStepInto_OnDoForm(t *testing.T) {
	if os.Getenv("LISP_DAP_TRACE") == "" {
		t.Setenv("LISP_DAP_TRACE", "1")
		traceEnabled = true
		t.Cleanup(func() { traceEnabled = false })
	}

	tmp := t.TempDir()
	script := filepath.Join(tmp, "do.lisp")
	src := "(do\n" +
		"    (println 1)\n" +
		"    (println 2)\n" +
		"    (println 3))\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}

	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": true})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})

	// Set a breakpoint at line 1 (the (do ...) form).
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 1}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})

	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// 1) Breakpoint stops at line 1 (the do form)
	t.Logf("=== Step 1: Breakpoint at line 1 ===")
	if got := stoppedAt(t, client, 5); got != 1 {
		t.Errorf("breakpoint: expected line 1, got %d", got)
	}

	// 2) Step in (F11) → should go into the body (expecting line 2)
	t.Logf("=== Step 2: F11 (stepIn) from breakpoint ===")
	sendRequest(t, client, 6, "stepIn", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "stepIn" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 7); got != 2 {
		t.Errorf("after stepIn: expected line 2, got %d", got)
	}

	// 3) Step over (F10) → should go to line 3
	t.Logf("=== Step 3: F10 (next) within do body ===")
	sendRequest(t, client, 8, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "next" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 9); got != 3 {
		t.Errorf("after next: expected line 3, got %d", got)
	}

	// 4) Step over again → should go to line 4
	t.Logf("=== Step 4: F10 (next) again ===")
	sendRequest(t, client, 10, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "next" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 11); got != 4 {
		t.Errorf("after next: expected line 4, got %d", got)
	}

	// Continue to exit
	sendRequest(t, client, 12, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "continue" && m["type"] == "response"
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "terminated"
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 13, "disconnect", nil)
}

// avoid "declared and not used" if json import is otherwise unused
var _ = json.Marshal

// TestUserFlatFile mirrors the user's real test.lisp: three separate
// top-level (println) forms loaded via load-file. A breakpoint on line 1
// must land on (println 1) itself — not on the synthetic load-file call
// or the `(do …)` wrapper it injects — and F10 (next) must then walk
// (println 2) and (println 3) line by line.
func TestUserFlatFile(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "flat.lisp")
	src := "(println 1)\n(println 2)\n(println 3)\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}
	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": true})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 1}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// 1) Breakpoint lands directly on (println 1) at line 1.
	if got := stoppedAt(t, client, 5); got != 1 {
		t.Errorf("breakpoint: expected line 1, got %d", got)
	}
	// 2) F10 → line 2.
	sendRequest(t, client, 6, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["command"] == "next" && m["type"] == "response" })
	if got := stoppedAt(t, client, 7); got != 2 {
		t.Errorf("after first next: expected line 2, got %d", got)
	}
	// 3) F10 → line 3.
	sendRequest(t, client, 8, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["command"] == "next" && m["type"] == "response" })
	if got := stoppedAt(t, client, 9); got != 3 {
		t.Errorf("after second next: expected line 3, got %d", got)
	}

	sendRequest(t, client, 10, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 11, "disconnect", nil)
}

// TestServer_Evaluate exercises the `evaluate` request while paused on a
// breakpoint: resolving a def'd symbol in the paused frame's env,
// computing a compound expression, and reporting errors.
func TestServer_Evaluate(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "eval.lisp")
	src := "(def answer 41)\n(println answer)\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}
	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 2}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	if got := stoppedAt(t, client, 5); got != 2 {
		t.Fatalf("breakpoint: expected line 2, got %d", got)
	}

	evaluate := func(seq int, expr string) map[string]interface{} {
		sendRequest(t, client, seq, "evaluate", map[string]interface{}{
			"expression": expr, "frameId": 0, "context": "repl",
		})
		return readUntil(t, client, func(m map[string]interface{}) bool {
			return m["command"] == "evaluate" && m["type"] == "response"
		})
	}

	// def'd symbol resolves in the paused frame's env
	resp := evaluate(6, "answer")
	if ok, _ := resp["success"].(bool); !ok {
		t.Fatalf("evaluate answer failed: %v", resp)
	}
	if body := resp["body"].(map[string]interface{}); body["result"] != "41" {
		t.Errorf("evaluate answer: expected 41, got %v", body["result"])
	}

	// compound expression
	resp = evaluate(7, "(+ answer 1)")
	if body := resp["body"].(map[string]interface{}); body["result"] != "42" {
		t.Errorf("evaluate (+ answer 1): expected 42, got %v", body["result"])
	}

	// error case: unknown symbol → success=false
	resp = evaluate(8, "no-such-symbol")
	if ok, _ := resp["success"].(bool); ok {
		t.Errorf("evaluate no-such-symbol: expected failure, got %v", resp)
	}

	// stepping still works after console evaluations (BP state intact)
	sendRequest(t, client, 9, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 10, "disconnect", nil)
}

// TestStepInto_ThreadingMacro verifies that F11 visits every stage of a
// threading macro at its own source line. Macro expansions are built at
// runtime by cons/concat and used to carry no cursors, so the debugger
// skipped every stage; macroexpand now fills the missing positions from
// the original subforms and EVAL re-runs the hook on the expanded form.
func TestStepInto_ThreadingMacro(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "thread.lisp")
	src := "(def a3 (-> {}\n" + // line 1
		"    (assoc :a \"1980\")\n" + // line 2
		"    (assoc :b \"1981\")\n" + // line 3
		"    (assoc :c \"1982\")))\n" + // line 4
		"(println a3)\n" // line 5
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	if err := nscoreextended.Load(ns); err != nil {
		t.Fatalf("nscoreextended.Load: %v", err)
	}
	if _, err := lisp.REPL(context.Background(), ns, coreextented.HeaderCoreExtended(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderCoreExtended: %v", err)
	}
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}
	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	// Breakpoint directly on a threading stage: it must fire even though
	// the stage only exists inside the macro expansion.
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 3}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// BP on stage (assoc :b …) at line 3.
	if got := stoppedAt(t, client, 5); got != 3 {
		t.Errorf("breakpoint on stage: expected line 3, got %d", got)
	}
	// F11 → inner stage (assoc :a …) at line 2 (evaluation goes inwards).
	sendRequest(t, client, 6, "stepIn", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["command"] == "stepIn" && m["type"] == "response" })
	if got := stoppedAt(t, client, 7); got != 2 {
		t.Errorf("after stepIn: expected line 2, got %d", got)
	}
	// F11 → the {} literal back on line 1.
	sendRequest(t, client, 8, "stepIn", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["command"] == "stepIn" && m["type"] == "response" })
	if got := stoppedAt(t, client, 9); got != 1 {
		t.Errorf("after second stepIn: expected line 1, got %d", got)
	}

	sendRequest(t, client, 10, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 11, "disconnect", nil)
}

// TestServer_StepOut verifies Shift-F11: from a breakpoint inside a
// function body, stepOut runs the rest of the function and stops at the
// next form of the caller's level.
func TestServer_StepOut(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "stepout.lisp")
	src := "(defn helper [x]\n" + // line 1
		"    (* x 2))\n" + // line 2
		"(println (helper 3))\n" + // line 3
		"(println \"after\")\n" // line 4
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}
	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: []SourceBreakpoint{{Line: 2}},
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// Breakpoint inside helper's body.
	if got := stoppedAt(t, client, 5); got != 2 {
		t.Fatalf("breakpoint: expected line 2, got %d", got)
	}
	// Shift-F11 → finish helper, stop at the caller level: next
	// top-level statement (println "after") on line 4.
	sendRequest(t, client, 6, "stepOut", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["command"] == "stepOut" && m["type"] == "response" })
	if got := stoppedAt(t, client, 7); got != 4 {
		t.Errorf("after stepOut: expected line 4, got %d", got)
	}

	sendRequest(t, client, 8, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 9, "disconnect", nil)
}

// TestServer_Pause verifies the pause request: a running program (no
// breakpoints) is interrupted at the next evaluated form.
func TestServer_Pause(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "pause.lisp")
	src := "(sleep 300)\n" + // line 1: long enough to send pause meanwhile
		"(println 1)\n" // line 2
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()
	ns := fullEnv(t)
	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1))
		done <- err
		return err
	}
	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// Let the program enter the (sleep 300) and pause it.
	time.Sleep(100 * time.Millisecond)
	sendRequest(t, client, 4, "pause", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "pause" && m["type"] == "response"
	})
	// The pause lands on the next evaluated form: (println 1) at line 2.
	if got := stoppedAt(t, client, 5); got != 2 {
		t.Errorf("after pause: expected line 2, got %d", got)
	}

	sendRequest(t, client, 6, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not return")
	}
	sendRequest(t, client, 7, "disconnect", nil)
}
