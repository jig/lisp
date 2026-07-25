package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	liblog "github.com/jig/lisp/lib/log"
	"github.com/jig/lisp/lib/log/nslog"
	"github.com/jig/lisp/types"
)

// newEnv builds an environment with the log namespace loaded (reading
// LOG_LEVEL) and the package logger redirected to the returned buffer.
func newEnv(t *testing.T, logLevel string) (types.EnvType, *bytes.Buffer) {
	t.Helper()
	t.Setenv("LOG_LEVEL", logLevel)
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nslog.Load(ns); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	liblog.SetOutput(&buf)
	return ns, &buf
}

func eval(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name())); err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
}

// lines decodes each JSON log line in buf.
func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestLogAttributes(t *testing.T) {
	ns, buf := newEnv(t, "")
	eval(t, ns, `(log-info "user created" :id 42 "name" "ada" :ok true :ratio 1.5 :tags [1 2])`)
	got := lines(t, buf)
	if len(got) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(got))
	}
	m := got[0]
	if m["level"] != "INFO" || m["msg"] != "user created" {
		t.Fatalf("unexpected level/msg: %v", m)
	}
	if m["id"] != float64(42) || m["name"] != "ada" || m["ok"] != true || m["ratio"] != 1.5 {
		t.Fatalf("unexpected attributes: %v", m)
	}
	if m["tags"] != "[1 2]" {
		t.Fatalf("composite value should render via the printer, got %v", m["tags"])
	}
	if _, ok := m["time"]; !ok {
		t.Fatalf("missing time: %v", m)
	}
}

func TestLogLevels(t *testing.T) {
	ns, buf := newEnv(t, "")
	eval(t, ns, `(log-debug "d")`)
	eval(t, ns, `(log-info "i")`)
	eval(t, ns, `(log-warn "w")`)
	eval(t, ns, `(log-error "e")`)
	got := lines(t, buf)
	if len(got) != 3 {
		t.Fatalf("default level must drop debug; got %d lines: %v", len(got), got)
	}
	for i, want := range []string{"INFO", "WARN", "ERROR"} {
		if got[i]["level"] != want {
			t.Fatalf("line %d: expected level %s, got %v", i, want, got[i]["level"])
		}
	}
}

func TestLogLevelEnv(t *testing.T) {
	ns, buf := newEnv(t, "debug")
	eval(t, ns, `(log-debug "d" :k :v)`)
	got := lines(t, buf)
	if len(got) != 1 || got[0]["level"] != "DEBUG" || got[0]["k"] != "v" {
		t.Fatalf("LOG_LEVEL=debug must emit debug lines: %v", got)
	}

	ns, buf = newEnv(t, "error")
	eval(t, ns, `(log-warn "w")`)
	eval(t, ns, `(log-error "e")`)
	got = lines(t, buf)
	if len(got) != 1 || got[0]["level"] != "ERROR" {
		t.Fatalf("LOG_LEVEL=error must drop warn: %v", got)
	}
}

func TestLogBadArguments(t *testing.T) {
	ns, _ := newEnv(t, "")
	for _, src := range []string{
		`(log-info "msg" :dangling)`,
		`(log-info "msg" 42 "value")`,
	} {
		if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name())); err == nil {
			t.Fatalf("%s did not error", src)
		}
	}
}
