package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	liblog "github.com/jig/lisp/lib/log"
	"github.com/jig/lisp/lib/log/nslog"
	"github.com/jig/lisp/types"
)

// newEnv builds an environment with the log namespace loaded (reading
// LOG_LEVEL). Destination is left to each test via the export hooks.
func newEnv(t *testing.T, logLevel string) types.EnvType {
	t.Helper()
	t.Setenv("LOG_LEVEL", logLevel)
	t.Cleanup(liblog.Reset)
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nslog.Load(ns); err != nil {
		t.Fatal(err)
	}
	return ns
}

func eval(t *testing.T, ns types.EnvType, src string) {
	t.Helper()
	if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name())); err != nil {
		t.Fatalf("EVAL %s: %v", src, err)
	}
}

// jsonLines decodes each JSON log line in data.
func jsonLines(t *testing.T, data string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(data), "\n") {
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
	ns := newEnv(t, "")
	var buf bytes.Buffer
	liblog.SetOutput(&buf)
	eval(t, ns, `(log-info "user created" :id 42 "name" "ada" :ok true :ratio 1.5 :tags [1 2])`)
	got := jsonLines(t, buf.String())
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
}

func TestLogLevels(t *testing.T) {
	ns := newEnv(t, "")
	var buf bytes.Buffer
	liblog.SetOutput(&buf)
	eval(t, ns, `(log-debug "d")`)
	eval(t, ns, `(log-info "i")`)
	eval(t, ns, `(log-warn "w")`)
	eval(t, ns, `(log-error "e")`)
	got := jsonLines(t, buf.String())
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
	ns := newEnv(t, "debug")
	var buf bytes.Buffer
	liblog.SetOutput(&buf)
	eval(t, ns, `(log-debug "d" :k :v)`)
	got := jsonLines(t, buf.String())
	if len(got) != 1 || got[0]["level"] != "DEBUG" || got[0]["k"] != "v" {
		t.Fatalf("LOG_LEVEL=debug must emit debug lines: %v", got)
	}

	ns = newEnv(t, "error")
	buf.Reset()
	liblog.SetOutput(&buf)
	eval(t, ns, `(log-warn "w")`)
	eval(t, ns, `(log-error "e")`)
	got = jsonLines(t, buf.String())
	if len(got) != 1 || got[0]["level"] != "ERROR" {
		t.Fatalf("LOG_LEVEL=error must drop warn: %v", got)
	}
}

func TestLogBadArguments(t *testing.T) {
	ns := newEnv(t, "")
	var buf bytes.Buffer
	liblog.SetOutput(&buf)
	for _, src := range []string{
		`(log-info "msg" :dangling)`,
		`(log-info "msg" 42 "value")`,
	} {
		if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile(t.Name())); err == nil {
			t.Fatalf("%s did not error", src)
		}
	}
}

func TestJournalFields(t *testing.T) {
	ns := newEnv(t, "")
	var entries []liblog.JournalEntry
	liblog.ForceJournal(&entries)
	liblog.SetRunFields(map[string]string{"commit": "abc123", "trace-id": "deadbeef"})
	eval(t, ns, `(log-warn "disk low" :free-mb 128 :message "sneaky")`)

	if len(entries) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Message != "disk low" || e.Priority != journal.PriWarning {
		t.Fatalf("unexpected message/priority: %+v", e)
	}
	want := map[string]string{
		"FREE_MB":    "128",
		"COMMIT":     "abc123",
		"TRACE_ID":   "deadbeef",
		"KV_MESSAGE": "sneaky", // must not clobber the journal MESSAGE field
	}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Fatalf("field %s: want %q, got %q (all: %v)", k, v, e.Fields[k], e.Fields)
		}
	}
	if e.Fields["SYSLOG_IDENTIFIER"] == "" {
		t.Fatalf("missing SYSLOG_IDENTIFIER: %v", e.Fields)
	}
}

func TestJournalPriorities(t *testing.T) {
	ns := newEnv(t, "debug")
	var entries []liblog.JournalEntry
	liblog.ForceJournal(&entries)
	eval(t, ns, `(log-debug "d")`)
	eval(t, ns, `(log-info "i")`)
	eval(t, ns, `(log-warn "w")`)
	eval(t, ns, `(log-error "e")`)
	want := []journal.Priority{journal.PriDebug, journal.PriInfo, journal.PriWarning, journal.PriErr}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(entries))
	}
	for i, p := range want {
		if entries[i].Priority != p {
			t.Fatalf("entry %d: want priority %d, got %d", i, p, entries[i].Priority)
		}
	}
}

func TestStateFileFallback(t *testing.T) {
	ns := newEnv(t, "")
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	liblog.ForceStateFile()
	liblog.SetIdentifier("myscript")

	eval(t, ns, `(log-info "hello" :id 7)`)

	data, err := os.ReadFile(filepath.Join(stateDir, "lisp", "myscript.log"))
	if err != nil {
		t.Fatalf("state log file not written: %v", err)
	}
	got := jsonLines(t, string(data))
	if len(got) != 1 || got[0]["msg"] != "hello" || got[0]["id"] != float64(7) {
		t.Fatalf("unexpected state log content: %v", got)
	}
}

func TestStateFileXDGDefault(t *testing.T) {
	ns := newEnv(t, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	liblog.ForceStateFile()

	eval(t, ns, `(log-info "hi")`)

	if _, err := os.Stat(filepath.Join(home, ".local", "state", "lisp", "lisp.log")); err != nil {
		t.Fatalf("expected default XDG path under ~/.local/state: %v", err)
	}
}
