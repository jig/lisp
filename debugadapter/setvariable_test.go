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
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// TestSetVariable pauses inside a function and rebinds a local via
// setVariable, then confirms the new value is visible in the frame.
func TestSetVariable(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "setvar.lisp")
	src := "(defn f [x]\n" +
		"  (println x)\n" +
		"  (println x))\n" +
		"(f 10)\n"
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

	launchToConfig(t, client, script, []SourceBreakpoint{{Line: 2}})

	if got := stoppedAt(t, client, 5); got != 2 {
		t.Fatalf("expected stop on line 2, got %d", got)
	}

	// Find the Locals scope of the top frame.
	sendRequest(t, client, 6, "scopes", map[string]interface{}{"frameId": 0})
	scopesResp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "scopes" && m["type"] == "response"
	})
	scopes := scopesResp["body"].(map[string]interface{})["scopes"].([]interface{})
	localsRef := -1
	for _, sc := range scopes {
		scope := sc.(map[string]interface{})
		if scope["name"] == "Locals" {
			localsRef = int(scope["variablesReference"].(float64))
		}
	}
	if localsRef < 0 {
		t.Fatalf("no Locals scope in %v", scopes)
	}

	// Rebind x to 99.
	sendRequest(t, client, 7, "setVariable", SetVariableArguments{
		VariablesReference: localsRef, Name: "x", Value: "99",
	})
	setResp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "setVariable" && m["type"] == "response"
	})
	if ok, _ := setResp["success"].(bool); !ok {
		t.Fatalf("setVariable failed: %v", setResp)
	}
	if body := setResp["body"].(map[string]interface{}); body["value"] != "99" {
		t.Fatalf("setVariable returned value %v, want 99", body["value"])
	}

	// The rebinding is visible when the expression is evaluated in the frame.
	sendRequest(t, client, 8, "evaluate", map[string]interface{}{
		"expression": "x", "frameId": 0, "context": "watch",
	})
	evalResp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "evaluate" && m["type"] == "response"
	})
	if body := evalResp["body"].(map[string]interface{}); body["result"] != "99" {
		t.Fatalf("after setVariable, x evaluated to %v, want 99", body["result"])
	}

	sendRequest(t, client, 9, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish")
	}
	sendRequest(t, client, 10, "disconnect", nil)
}
