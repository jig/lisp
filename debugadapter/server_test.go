//go:build debugger

package debugadapter

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
)

// pair returns two Transports that talk to each other over an in-memory
// net.Pipe. The first is for the test "client", the second for the
// server under test.
func pair() (*Transport, *Transport, func()) {
	a, b := net.Pipe()
	client := NewTransport(a, a, a)
	server := NewTransport(b, b, b)
	return client, server, func() {
		_ = a.Close()
		_ = b.Close()
	}
}

// runServer launches srv in its own goroutine and returns a teardown
// closure that closes the transports and waits for Run to return. This
// matters because Server.Run mutates the global runtime.Hook on entry
// and restores it on exit; without waiting, sequential tests race on
// that variable.
func runServer(t *testing.T, srv *Server, closeTransports func()) func() {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Run(context.Background())
	}()
	return func() {
		closeTransports()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("server did not stop within 3s")
		}
	}
}

// readUntil reads messages until one matching the predicate is seen.
// Returns the matched payload and any earlier messages.
func readUntil(t *testing.T, c *Transport, predicate func(map[string]interface{}) bool) map[string]interface{} {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for matching message")
		default:
		}
		raw, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if predicate(m) {
			return m
		}
	}
}

func sendRequest(t *testing.T, c *Transport, seq int, command string, args interface{}) {
	t.Helper()
	req := map[string]interface{}{
		"seq":     seq,
		"type":    "request",
		"command": command,
	}
	if args != nil {
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("encode args: %v", err)
		}
		req["arguments"] = json.RawMessage(raw)
	}
	if err := c.WriteMessage(req); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func newTestEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	return ns
}

func TestServer_InitializeLaunchHandshake(t *testing.T) {
	client, server, closer := pair()

	ns := newTestEnv(t)
	eval := func(_ context.Context, _ types.EnvType) error { return nil }

	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", map[string]interface{}{})
	resp := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["type"] == "response" && m["command"] == "initialize"
	})
	if ok, _ := resp["success"].(bool); !ok {
		t.Fatalf("initialize failed: %v", resp)
	}

	// initialized event arrives after the response
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["type"] == "event" && m["event"] == "initialized"
	})

	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": false})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["type"] == "response" && m["command"] == "launch"
	})

	sendRequest(t, client, 3, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["type"] == "response" && m["command"] == "configurationDone"
	})

	// Empty eval → terminated event soon after.
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["type"] == "event" && m["event"] == "terminated"
	})

	sendRequest(t, client, 4, "disconnect", nil)
}

func TestServer_StepOverSkipsTCOContinuation(t *testing.T) {
	client, server, closer := pair()

	ns := newTestEnv(t)
	called := make(chan struct{}, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := evalSimple(ctx, env, "(do 1 2 3 4)")
		called <- struct{}{}
		return err
	}

	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "initialized"
	})
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": true})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// First stop: entry pause on the outer (do …) form.
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped"
	})

	// Step over should walk past every TCO continuation of the outer
	// `do` and land at the next *new* call. Since `(do 1 2 3 4)` has
	// only literal subforms (no nested calls), it should run to
	// completion without another stop.
	sendRequest(t, client, 4, "next", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "next" && m["type"] == "response"
	})

	// Expect terminated without an intervening stopped.
	got := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped" || m["event"] == "terminated"
	})
	if got["event"] != "terminated" {
		t.Fatalf("expected terminated next, got another stopped: %v", got)
	}

	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish")
	}
	sendRequest(t, client, 5, "disconnect", nil)
}

func TestServer_StopOnEntryAndContinue(t *testing.T) {
	client, server, closer := pair()

	ns := newTestEnv(t)
	called := make(chan struct{}, 1)
	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := evalSimple(ctx, env, "(do 1 2 3)")
		called <- struct{}{}
		return err
	}

	srv := NewServer(server, eval, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	sendRequest(t, client, 1, "initialize", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "initialized"
	})
	sendRequest(t, client, 2, "launch", map[string]interface{}{"stopOnEntry": true})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "launch" && m["type"] == "response"
	})
	sendRequest(t, client, 3, "configurationDone", nil)
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "configurationDone" && m["type"] == "response"
	})

	// stopOnEntry → first stopped event
	stopped := readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "stopped"
	})
	body := stopped["body"].(map[string]interface{})
	if body["reason"] != "entry" {
		t.Fatalf("expected stopped reason=entry, got %v", body["reason"])
	}

	sendRequest(t, client, 4, "continue", map[string]interface{}{"threadId": 1})
	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["command"] == "continue" && m["type"] == "response"
	})

	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Fatal("eval did not finish after continue")
	}

	readUntil(t, client, func(m map[string]interface{}) bool {
		return m["event"] == "terminated"
	})
	sendRequest(t, client, 5, "disconnect", nil)
}

// TestServer_StreamOutput verifies that bytes fed into StreamOutput
// arrive at the client as `output` events with the right category.
func TestServer_StreamOutput(t *testing.T) {
	client, server, closer := pair()

	ns := newTestEnv(t)
	eval := func(_ context.Context, _ types.EnvType) error { return nil }
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

	pr, pw := net.Pipe()
	go srv.StreamOutput(pr, "stdout")
	go func() {
		_, _ = pw.Write([]byte("hello from println\n"))
		_ = pw.Close()
	}()

	got := readUntil(t, client, func(m map[string]interface{}) bool {
		if m["event"] != "output" {
			return false
		}
		body, _ := m["body"].(map[string]interface{})
		return body["category"] == "stdout" && body["output"] == "hello from println\n"
	})
	if got == nil {
		t.Fatal("output event not received")
	}
	sendRequest(t, client, 4, "disconnect", nil)
}
