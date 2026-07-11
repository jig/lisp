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

// scopeNames returns the scope names reported for a frame, and the
// variablesReference of the named scope (or -1).
func scopeNames(t *testing.T, client *Transport, seq int, frameID int, want string) ([]string, int) {
	t.Helper()
	sendRequest(t, client, seq, "scopes", map[string]interface{}{"frameId": frameID})
	resp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "scopes" && m["type"] == "response"
	})
	scopes := resp["body"].(map[string]interface{})["scopes"].([]interface{})
	names := make([]string, 0, len(scopes))
	ref := -1
	for _, sc := range scopes {
		scope := sc.(map[string]interface{})
		name := scope["name"].(string)
		names = append(names, name)
		if name == want {
			ref = int(scope["variablesReference"].(float64))
		}
	}
	return names, ref
}

// TestReturnValueAfterStep verifies that after stepping over a form, its
// result appears as a synthetic "Return value" scope — and that a plain
// breakpoint stop has no such scope.
func TestReturnValueAfterStep(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "ret.lisp")
	src := "(+ 1 2)\n" +
		"(println 9)\n"
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

	launchToConfig(t, client, script, []SourceBreakpoint{{Line: 1}})

	// Breakpoint stop on line 1: no return value yet.
	if got := stoppedAt(t, client, 5); got != 1 {
		t.Fatalf("expected stop on line 1, got %d", got)
	}
	if names, ref := scopeNames(t, client, 6, 0, "Return value"); ref != -1 {
		t.Fatalf("breakpoint stop should have no Return value scope, got %v", names)
	}

	// Step over (+ 1 2): land on line 2 with its result surfaced.
	sendRequest(t, client, 7, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "next" && m["type"] == "response"
	})
	if got := stoppedAt(t, client, 8); got != 2 {
		t.Fatalf("expected step to land on line 2, got %d", got)
	}
	_, ref := scopeNames(t, client, 9, 0, "Return value")
	if ref == -1 {
		t.Fatal("expected a Return value scope after step-over")
	}
	sendRequest(t, client, 10, "variables", map[string]interface{}{"variablesReference": ref})
	varsResp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "variables" && m["type"] == "response"
	})
	vars := varsResp["body"].(map[string]interface{})["variables"].([]interface{})
	if len(vars) != 1 {
		t.Fatalf("expected one return variable, got %v", vars)
	}
	v := vars[0].(map[string]interface{})
	if v["name"] != "(return)" || v["value"] != "3" {
		t.Fatalf("expected (return)=3, got name=%v value=%v", v["name"], v["value"])
	}

	sendRequest(t, client, 11, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool { return m["event"] == "terminated" })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish")
	}
	sendRequest(t, client, 12, "disconnect", nil)
}
