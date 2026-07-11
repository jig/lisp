//go:build lispdebug

package debugadapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// TestExceptionBreakpoint verifies that, with the "all" exception filter
// enabled, execution pauses at the point a Lisp error is raised — with
// the raising frame on top of the stack — and then runs to completion
// when resumed.
func TestExceptionBreakpoint(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "boom.lisp")
	src := "(defn boom []\n" +
		"  (throw {:code 99}))\n" +
		"(boom)\n"
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
	sendRequest(t, client, 3, "setExceptionBreakpoints", SetExceptionBreakpointsArguments{Filters: []string{"all"}})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setExceptionBreakpoints" && m["type"] == "response"
	})
	sendRequest(t, client, 4, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// Pause at the raise site: reason "exception", top frame on line 2
	// (the (throw …) form inside boom).
	stopped := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped"
	})
	body := stopped["body"].(map[string]interface{})
	if body["reason"] != "exception" {
		t.Fatalf("expected stopped reason=exception, got %v", body["reason"])
	}
	sendRequest(t, client, 5, "stackTrace", map[string]interface{}{"threadId": 1, "startFrame": 0, "levels": 20})
	st := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "stackTrace" && m["type"] == "response"
	})
	frames := st["body"].(map[string]interface{})["stackFrames"].([]interface{})
	top := frames[0].(map[string]interface{})
	if line, _ := top["line"].(float64); int(line) != 2 {
		t.Fatalf("expected exception raised on line 2, got %v", top["line"])
	}

	// Resume: the error keeps unwinding (no second stop) and the program
	// terminates.
	sendRequest(t, client, 6, "continue", map[string]interface{}{"threadId": 1})
	got := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped" || m["event"] == "terminated"
	})
	if got["event"] != "terminated" {
		t.Fatalf("expected termination after continue, got another stop: %v", got)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the thrown error to propagate out of eval")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish")
	}
	sendRequest(t, client, 7, "disconnect", nil)
}

// TestExceptionBreakpointDisabledByDefault verifies no pause happens when
// the client never enables the exception filter.
func TestExceptionBreakpointDisabledByDefault(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "boom2.lisp")
	if err := os.WriteFile(script, []byte("(throw {:code 1})\n"), 0o644); err != nil {
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

	got := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped" || m["event"] == "terminated"
	})
	if got["event"] != "terminated" {
		t.Fatalf("expected no exception pause when the filter is off, got: %v", got)
	}
	<-done
	sendRequest(t, client, 4, "disconnect", nil)
}
