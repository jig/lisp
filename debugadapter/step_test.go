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
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewCursorHere(script, -3, 1))
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
		_, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewCursorHere(script, -3, 1))
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

// avoid "declared and not used" if json import is otherwise unused
var _ = json.Marshal
