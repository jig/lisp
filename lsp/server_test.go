package lsp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/types"
)

// pair returns two Transports talking over an in-memory net.Pipe: one
// for the test client, one for the server under test.
func pair() (*Transport, *Transport, func()) {
	a, b := net.Pipe()
	return NewTransport(a, a, a), NewTransport(b, b, b), func() {
		_ = a.Close()
		_ = b.Close()
	}
}

// testEnv builds an interpreter environment with the core library and
// the lisp-defined headers (defn, …), mirroring what cmd/lisp loads.
func testEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	if _, err := lisp.REPL(context.Background(), ns, core.HeaderBasic(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderBasic: %v", err)
	}
	return ns
}

func runServer(t *testing.T, srv *Server, closer func()) func() {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Run(context.Background())
	}()
	return func() {
		closer()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("server did not stop within 3s")
		}
	}
}

func send(t *testing.T, c *Transport, id int, method string, params interface{}) {
	t.Helper()
	msg := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if id != 0 {
		msg["id"] = id
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		msg["params"] = json.RawMessage(raw)
	}
	if err := c.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

// readUntil reads messages until one matches the predicate.
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

func response(id int) func(map[string]interface{}) bool {
	return func(m map[string]interface{}) bool {
		got, ok := m["id"].(float64)
		return ok && int(got) == id
	}
}

func notification(method string) func(map[string]interface{}) bool {
	return func(m map[string]interface{}) bool {
		return m["method"] == method && m["id"] == nil
	}
}

// startSession spins up a server with the standard handshake done.
func startSession(t *testing.T) (*Transport, func()) {
	t.Helper()
	client, server, closer := pair()
	srv := NewServer(server, testEnv(t))
	stop := runServer(t, srv, closer)

	send(t, client, 1, "initialize", map[string]interface{}{})
	resp := readUntil(t, client, response(1))
	caps := resp["result"].(map[string]interface{})["capabilities"].(map[string]interface{})
	if caps["hoverProvider"] != true {
		t.Fatalf("expected hoverProvider capability, got %v", caps)
	}
	send(t, client, 0, "initialized", map[string]interface{}{})
	return client, stop
}

func didOpen(t *testing.T, client *Transport, uri, text string) map[string]interface{} {
	t.Helper()
	send(t, client, 0, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "lisp", Version: 1, Text: text},
	})
	return readUntil(t, client, notification("textDocument/publishDiagnostics"))
}

func TestServer_DiagnosticsOnParseError(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	// missing closing paren on line 2 (zero-based line 1)
	diags := didOpen(t, client, "file:///broken.lisp", "(def a 1)\n(defn foo [x]\n")
	params := diags["params"].(map[string]interface{})
	list := params["diagnostics"].([]interface{})
	if len(list) == 0 {
		t.Fatal("expected a parse diagnostic, got none")
	}
	d := list[0].(map[string]interface{})
	if d["severity"].(float64) != 1 {
		t.Errorf("expected severity 1, got %v", d["severity"])
	}
	if d["message"] == "" {
		t.Error("expected a non-empty message")
	}
}

func TestServer_DiagnosticsClearOnFix(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///fix.lisp"
	diags := didOpen(t, client, uri, "(def a\n")
	if l := diags["params"].(map[string]interface{})["diagnostics"].([]interface{}); len(l) == 0 {
		t.Fatal("expected initial diagnostic")
	}

	send(t, client, 0, "textDocument/didChange", map[string]interface{}{
		"textDocument":   map[string]interface{}{"uri": uri, "version": 2},
		"contentChanges": []map[string]interface{}{{"text": "(def a 1)\n"}},
	})
	diags = readUntil(t, client, notification("textDocument/publishDiagnostics"))
	if l := diags["params"].(map[string]interface{})["diagnostics"].([]interface{}); len(l) != 0 {
		t.Fatalf("expected diagnostics cleared after fix, got %v", l)
	}
}

func TestServer_DocumentSymbols(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///syms.lisp"
	didOpen(t, client, uri, "(def answer 42)\n(defn add [a b]\n    (+ a b))\n")

	send(t, client, 2, "textDocument/documentSymbol", DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	resp := readUntil(t, client, response(2))
	syms := resp["result"].([]interface{})
	if len(syms) != 2 {
		t.Fatalf("expected 2 symbols, got %d: %v", len(syms), syms)
	}
	first := syms[0].(map[string]interface{})
	if first["name"] != "answer" {
		t.Errorf("expected first symbol answer, got %v", first["name"])
	}
	second := syms[1].(map[string]interface{})
	if second["name"] != "add" {
		t.Errorf("expected second symbol add, got %v", second["name"])
	}
	// (defn add …) starts on line 2 → zero-based line 1
	rng := second["range"].(map[string]interface{})["start"].(map[string]interface{})
	if rng["line"].(float64) != 1 {
		t.Errorf("expected add at line 1, got %v", rng["line"])
	}
}

func TestServer_Completion(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///comp.lisp"
	didOpen(t, client, uri, "(defn my-local-fn [x] x)\n")

	send(t, client, 3, "textDocument/completion", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 0},
	})
	resp := readUntil(t, client, response(3))
	items := resp["result"].([]interface{})
	labels := map[string]bool{}
	for _, it := range items {
		labels[it.(map[string]interface{})["label"].(string)] = true
	}
	for _, want := range []string{"my-local-fn", "str", "println"} {
		if !labels[want] {
			t.Errorf("expected completion %q, missing (got %d items)", want, len(items))
		}
	}
}

func TestServer_Hover(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///hover.lisp"
	didOpen(t, client, uri, "(defn add [a b]\n    (+ a b))\n(add 1 2)\n")

	// hover over `add` on line 3 (zero-based 2), character 1
	send(t, client, 4, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 1},
	})
	resp := readUntil(t, client, response(4))
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected hover result, got %v", resp["result"])
	}
	value := result["contents"].(map[string]interface{})["value"].(string)
	if want := "(defn add [a b])"; !strings.Contains(value, want) {
		t.Errorf("expected hover to contain %q, got %q", want, value)
	}

	// hover over a core builtin
	send(t, client, 5, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 5}, // the `+`
	})
	resp = readUntil(t, client, response(5))
	if _, ok := resp["result"].(map[string]interface{}); !ok {
		t.Errorf("expected hover result for +, got %v", resp["result"])
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	send(t, client, 6, "textDocument/definition", map[string]interface{}{})
	resp := readUntil(t, client, response(6))
	if resp["error"] == nil {
		t.Fatalf("expected MethodNotFound error, got %v", resp)
	}
}

func TestServer_UnknownSymbolWarning(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	// printlnooo is not defined anywhere; println is; my-fn is defined
	// later in the file (order must not matter); x is a let binding.
	src := "(printlnooo 3)\n" +
		"(println 4)\n" +
		"(my-fn 5)\n" +
		"(let [x (fn [y] y)] (x 6))\n" +
		"(defn my-fn [a] a)\n"
	diags := didOpen(t, client, "file:///unknown.lisp", src)
	list := diags["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(list), list)
	}
	d := list[0].(map[string]interface{})
	if d["severity"].(float64) != 2 {
		t.Errorf("expected warning severity 2, got %v", d["severity"])
	}
	if !strings.Contains(d["message"].(string), "printlnooo") {
		t.Errorf("expected message about printlnooo, got %v", d["message"])
	}
	start := d["range"].(map[string]interface{})["start"].(map[string]interface{})
	if start["line"].(float64) != 0 {
		t.Errorf("expected warning at line 0, got %v", start["line"])
	}
	if start["character"].(float64) != 1 {
		t.Errorf("expected warning at character 1, got %v", start["character"])
	}
}

// TestServer_RequireImportsSymbols verifies the LSP follows
// `(require "module")` forms: definitions from the resolved module are
// known (no false unknown-symbol warning), completable and hoverable;
// an unresolvable require yields a warning on the require form itself.
func TestServer_RequireImportsSymbols(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "geometry.lisp"),
		[]byte("(require \"nested\")\n(defn area [r] (* r r 3))\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested.lisp"),
		[]byte("(defn perimeter [r] (* 2 r 3))\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client, server, closer := pair()
	ns := testEnv(t)
	// Configure the require search path (also installs `require` in ns).
	if err := require.LoadWithConfig(require.Config{IncludeDirs: []string{dir}})(ns); err != nil {
		t.Fatalf("require.LoadWithConfig: %v", err)
	}
	srv := NewServer(server, ns)
	stop := runServer(t, srv, closer)
	defer stop()

	send(t, client, 1, "initialize", map[string]interface{}{})
	readUntil(t, client, response(1))
	send(t, client, 0, "initialized", map[string]interface{}{})

	// geometry/area comes from the require'd module (qualified),
	// nested/perimeter from its nested require, and area unqualified via
	// :refer: none must be flagged as unknown.
	uri := "file:///main.lisp"
	diags := didOpen(t, client, uri,
		"(require \"geometry\" :refer [\"area\"])\n(println (geometry/area 2))\n(println (nested/perimeter 2))\n(println (area 2))\n")
	list := diags["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(list) != 0 {
		t.Fatalf("expected no diagnostics, got %v", list)
	}

	// completion includes the qualified definition with its module name
	send(t, client, 2, "textDocument/completion", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 0},
	})
	resp := readUntil(t, client, response(2))
	labels := map[string]string{}
	for _, it := range resp["result"].([]interface{}) {
		item := it.(map[string]interface{})
		label, _ := item["label"].(string)
		detail, _ := item["detail"].(string)
		labels[label] = detail
	}
	if !strings.Contains(labels["geometry/area"], "geometry.lisp") {
		t.Errorf("expected geometry/area completion naming geometry.lisp, got %q", labels["geometry/area"])
	}
	if _, ok := labels["area"]; !ok {
		t.Error("expected unqualified area (via :refer) in completion")
	}
	if _, ok := labels["nested/perimeter"]; !ok {
		t.Error("expected nested/perimeter (transitive require) in completion")
	}

	// hover on the qualified symbol shows its signature and module
	send(t, client, 3, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 14}, // over `geometry/area`
	})
	resp = readUntil(t, client, response(3))
	hover, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected hover result, got %v", resp["result"])
	}
	value := hover["contents"].(map[string]interface{})["value"].(string)
	if !strings.Contains(value, "area") || !strings.Contains(value, "geometry.lisp") {
		t.Errorf("expected hover with signature and module, got %q", value)
	}

	// :as aliasing is honoured
	diags = didOpen(t, client, "file:///alias.lisp",
		"(require \"geometry\" :as \"g\")\n(println (g/area 2))\n")
	list = diags["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(list) != 0 {
		t.Fatalf("expected no diagnostics with :as alias, got %v", list)
	}

	// an unresolvable require warns on the require form
	diags = didOpen(t, client, "file:///bad.lisp", "(require \"no/such/module\")\n")
	list = diags["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 diagnostic for missing module, got %v", list)
	}
	msg := list[0].(map[string]interface{})["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Errorf("expected not-found message, got %q", msg)
	}
}
