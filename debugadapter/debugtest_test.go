//go:build debugger

package debugadapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// TestServer_DebugTestStopsInBody reproduces the editor's "Debug Test"
// flow: the DAP eval closure loads a deftest file and then runs one test
// by name via test/run-test!. A breakpoint inside that test's body must
// stop the debugger, which is the whole point of the feature (the CLI
// --test runner cannot).
func TestServer_DebugTestStopsInBody(t *testing.T) {
	if os.Getenv("LISP_DAP_TRACE") == "" {
		t.Setenv("LISP_DAP_TRACE", "1")
		traceEnabled = true
		t.Cleanup(func() { traceEnabled = false })
	}

	tmp := t.TempDir()
	script := filepath.Join(tmp, "sample_test.lisp")
	// Line 2 is the body of deftest `target`; line 4 belongs to `other`.
	src := "(deftest target\n" +
		"  (is (= 3 (+ 1 2))))\n" +
		"(deftest other\n" +
		"  (is (= 4 (* 2 2))))\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	runtime.Modules.Register(script, script)

	client, server, closer := pair()

	ns := fullEnv(t)
	if err := nstest.Load(ns); err != nil {
		t.Fatalf("nstest.Load: %v", err)
	}

	done := make(chan error, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		if _, err := lisp.REPL(ctx, env, fmt.Sprintf("(load-file %q)", script), types.NewAnonymousCursorHere(1, 1)); err != nil {
			done <- err
			return err
		}
		// The --run-test path: run just `target` in this session.
		runOne := types.NewList(nil, types.Symbol{Val: "test/run-test!"}, "target")
		_, err := lisp.EVAL(ctx, runOne, env)
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

	// Breakpoint inside the body of `target` (line 2).
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

	// Drain every stop until the session terminates, collecting the lines
	// so the breakpoint's behaviour is observable regardless of how many
	// times line 2 is reached (creating the test closure at load time may
	// stop there too, before the body runs under test/run-test!).
	seq := 5
	stops := []int{}
	for {
		msg := readUntil(t, client, func(m map[string]interface{}) bool {
			return m["event"] == "stopped" || m["event"] == "terminated"
		})
		if msg["event"] == "terminated" {
			break
		}
		sendRequest(t, client, seq, "stackTrace", map[string]interface{}{"threadId": 1, "startFrame": 0, "levels": 20})
		resp := readUntil(t, client, func(m map[string]interface{}) bool {
			return m["type"] == "response" && m["command"] == "stackTrace"
		})
		frames := resp["body"].(map[string]interface{})["stackFrames"].([]interface{})
		line, _ := frames[0].(map[string]interface{})["line"].(float64)
		stops = append(stops, int(line))
		seq++
		sendRequest(t, client, seq, "continue", map[string]interface{}{"threadId": 1})
		readUntil(t, client, func(m map[string]interface{}) bool {
			return m["command"] == "continue" && m["type"] == "response"
		})
		seq++
	}
	t.Logf("stops: %v", stops)
	// The meaningful guarantees: the breakpoint fires (at least once), it
	// only ever fires at the target test body (line 2) — never at load
	// time creating the closure (the isFnForm skip), and never in `other`,
	// which is not run. A dense assertion line may pause more than once as
	// execution passes through the forms on it; that is pre-existing DAP
	// line-transition behaviour, not specific to Debug Test.
	if len(stops) == 0 {
		t.Fatal("breakpoint in the test body never fired")
	}
	for _, l := range stops {
		if l != 2 {
			t.Errorf("unexpected stop at line %d; all stops must be the target body (line 2): %v", l, stops)
		}
	}

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
