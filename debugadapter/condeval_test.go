//go:build debugger

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
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// launchToConfig runs the initialize/launch/setBreakpoints/configurationDone
// handshake for a script whose breakpoints carry conditions or log
// messages, and returns nothing — the caller drives the session from there.
func launchToConfig(t *testing.T, client *Transport, script string, bps []SourceBreakpoint) {
	t.Helper()
	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "initialized" })
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "setBreakpoints", SetBreakpointsArguments{
		Source:      Source{Path: script},
		Breakpoints: bps,
	})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})
}

// TestConditionalBreakpoint verifies a breakpoint with a condition only
// pauses on the iteration where the condition holds.
func TestConditionalBreakpoint(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "cond.lisp")
	// The breakpoint sits on line 2, inside f's body; f is called three
	// times so the line is hit with x = 1, 2, 3.
	src := "(defn f [x]\n" +
		"  (println x))\n" +
		"(f 1)\n" +
		"(f 2)\n" +
		"(f 3)\n"
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

	launchToConfig(t, client, script, []SourceBreakpoint{{Line: 2, Condition: "(= x 2)"}})

	// It must stop exactly once, with x == 2.
	if got := stoppedAt(t, client, 5); got != 2 {
		t.Fatalf("expected stop on line 2, got %d", got)
	}
	sendRequest(t, client, 6, "evaluate", map[string]interface{}{
		"expression": "x", "frameId": 0, "context": "watch",
	})
	resp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "evaluate" && m["type"] == "response"
	})
	if body := resp["body"].(map[string]interface{}); body["result"] != "2" {
		t.Fatalf("expected x=2 at the conditional stop, got %v", body["result"])
	}

	// Continuing runs to completion without another stop (x=3 fails).
	sendRequest(t, client, 7, "continue", map[string]interface{}{"threadId": 1})
	got := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped" || m["event"] == "terminated"
	})
	if got["event"] != "terminated" {
		t.Fatalf("expected termination after continue, got another stop: %v", got)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish")
	}
	sendRequest(t, client, 8, "disconnect", nil)
}

// TestLogpoint verifies a breakpoint with a logMessage emits interpolated
// output instead of pausing.
func TestLogpoint(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "log.lisp")
	src := "(def n 7)\n" +
		"(println n)\n"
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

	launchToConfig(t, client, script, []SourceBreakpoint{{Line: 2, LogMessage: "n is {n}!"}})

	// Read to termination, recording that we never stop and that the
	// interpolated logpoint output arrives.
	var sawLog bool
	for {
		raw, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if m["event"] == "stopped" {
			t.Fatal("logpoint must not pause execution")
		}
		if m["event"] == "output" {
			body, _ := m["body"].(map[string]interface{})
			if out, _ := body["output"].(string); out == "n is 7!\n" {
				sawLog = true
			}
		}
		if m["event"] == "terminated" {
			break
		}
	}
	if !sawLog {
		t.Fatal("did not observe the interpolated logpoint output \"n is 7!\"")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish")
	}
	sendRequest(t, client, 8, "disconnect", nil)
}
