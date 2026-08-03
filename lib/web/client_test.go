package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/types"
)

// eval evaluates src, failing the test on error.
func eval(t *testing.T, ns types.EnvType, src string) types.MalType {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("READ %s: %v", src, err)
	}
	v, err := lisp.EVAL(context.Background(), ast, ns)
	if err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
	return v
}

// expectTrue evaluates src and requires the result to be true.
func expectTrue(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if v := eval(t, ns, src); v != true {
		t.Fatalf("%s = %v, want true", src, v)
	}
}

// expectThrow evaluates src and requires a catchable lisp error.
func expectThrow(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatalf("READ %s: %v", src, err)
	}
	if _, err := lisp.EVAL(context.Background(), ast, ns); err == nil {
		t.Fatalf("%s did not throw", src)
	}
}

// TestClientRequestGet pins web-get / web-request against a live
// httptest server: status, lowercased keyword headers, body, and the
// Ring-symmetric response shape.
func TestClientRequestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Probe", "yes")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("OK"))
	}))
	defer srv.Close()

	ns := newEnv(t)
	eval(t, ns, fmt.Sprintf(`(def resp (web-get %q))`, srv.URL))
	expectTrue(t, ns, `(= 200 (get resp :status))`)
	expectTrue(t, ns, `(= "OK" (get resp :body))`)
	expectTrue(t, ns, `(= "yes" (get-in resp [:headers :x-probe]))`)
}

// TestClientRequestPost covers method, request headers and body, and
// that a non-2xx status returns (it does not throw).
func TestClientRequestPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-Token") != "s3cret" {
			w.WriteHeader(403)
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		w.WriteHeader(201)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	ns := newEnv(t)
	eval(t, ns, fmt.Sprintf(
		`(def resp (web-request {:method :post :url %q :headers {:x-token "s3cret"} :body "hola"}))`, srv.URL))
	expectTrue(t, ns, `(= 201 (get resp :status))`)
	expectTrue(t, ns, `(= "hola" (get resp :body))`)

	eval(t, ns, fmt.Sprintf(`(def denied (web-request {:method :post :url %q}))`, srv.URL))
	expectTrue(t, ns, `(= 403 (get denied :status))`)
}

// TestClientErrors pins the throwing cases: unreachable server,
// :timeout-ms exceeded, and contract errors.
func TestClientErrors(t *testing.T) {
	ns := newEnv(t)
	// A port from httptest closed immediately: connection refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dead := srv.URL
	srv.Close()
	expectThrow(t, ns, fmt.Sprintf(`(web-get %q)`, dead))

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer slow.Close()
	expectThrow(t, ns, fmt.Sprintf(`(web-request {:url %q :timeout-ms 50})`, slow.URL))

	expectThrow(t, ns, `(web-request {:method :get})`) // no :url
	expectThrow(t, ns, `(web-request {:url "http://x" :timeout-ms -1})`)
}
