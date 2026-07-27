// Package log adds structured logging to jig/lisp. Four builtins —
// log-debug, log-info, log-warn, log-error — emit one record per call
// with alternating key/value attributes:
//
//	(log-info "user created" :id 42)
//
// The destination is resolved once, at the first record:
//
//   - systemd-journald when its socket is available (Linux): records
//     are sent as native journal fields — MESSAGE, PRIORITY,
//     SYSLOG_IDENTIFIER (the script name) and each attribute as an
//     uppercased field (:trace-id → TRACE_ID). Read them back with
//     `journalctl -t <script> -o json`.
//   - otherwise (macOS, no systemd) a JSON-lines file under the XDG
//     state directory: $XDG_STATE_HOME/lisp/<script>.log, defaulting
//     to ~/.local/state/lisp/<script>.log.
//
// There is no environment variable to choose the destination, by
// design; LOG_LEVEL (debug/info/warn/error; anything else, or unset,
// means info) sets the minimum level and is read when the namespace
// loads.
//
// Keys are keywords (or strings). Values pass through as themselves
// when primitive and render via the Lisp printer otherwise.
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coreos/go-systemd/v22/journal"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/printer"
	. "github.com/jig/lisp/types"
)

var (
	level = new(slog.LevelVar)

	mu         sync.Mutex
	identifier = "lisp"
	runFields  map[string]string
	current    *sink // nil until the first record resolves it

	// journalAvailable and journalSend are swapped by tests.
	journalAvailable = journal.Enabled
	journalSend      = journal.Send
)

// sink is a resolved destination: the journal, or a slog JSON logger
// (state file; tests inject arbitrary writers).
type sink struct {
	journald bool
	logger   *slog.Logger
}

// SetIdentifier names the running script (journal SYSLOG_IDENTIFIER
// and state-file basename). The command layer calls it before the
// script starts; it resets any previously resolved destination.
func SetIdentifier(name string) {
	mu.Lock()
	defer mu.Unlock()
	if name != "" {
		identifier = name
	}
	current = nil
}

// SetRunFields attaches constant fields to every record of this run
// (e.g. commit, repo, trace_id under lisp-integrity).
func SetRunFields(fields map[string]string) {
	mu.Lock()
	defer mu.Unlock()
	runFields = fields
}

func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// stateLogPath returns $XDG_STATE_HOME/lisp/<identifier>.log with the
// spec fallback ~/.local/state.
func stateLogPath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("log: cannot resolve state directory: %w", err)
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "lisp", identifier+".log"), nil
}

// resolve picks the destination. Callers hold mu.
func resolve() (*sink, error) {
	if current != nil {
		return current, nil
	}
	if journalAvailable() {
		current = &sink{journald: true}
		return current, nil
	}
	path, err := stateLogPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	current = &sink{logger: newLogger(f)}
	return current, nil
}

// parseLevel maps a LOG_LEVEL value to a slog level; unknown or empty
// values mean Info.
func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// priority maps a slog level to a syslog/journal priority.
func priority(l slog.Level) journal.Priority {
	switch {
	case l <= slog.LevelDebug:
		return journal.PriDebug
	case l <= slog.LevelInfo:
		return journal.PriInfo
	case l <= slog.LevelWarn:
		return journal.PriWarning
	default:
		return journal.PriErr
	}
}

// logKey renders a log attribute key: keywords lose their sigil,
// strings pass verbatim.
func logKey(fnName string, v MalType) (string, error) {
	switch v := v.(type) {
	case Keyword:
		return string(v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("%s: expected a keyword or string key, got %T", fnName, v)
	}
}

// logValue renders a Lisp value for a log attribute: keywords lose
// their sigil, primitives pass through as themselves, everything else
// is printed.
func logValue(v MalType) any {
	switch v := v.(type) {
	case Keyword:
		return string(v)
	case string:
		return v
	case int, float32, float64, bool, nil:
		return v
	default:
		return printer.Pr_str(v, false)
	}
}

// fieldName converts an attribute key to a journal field name:
// uppercased, non-alphanumerics become underscores, and names that
// would collide with the fields the runtime itself sets are prefixed.
func fieldName(key string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(key) {
		switch {
		case r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" || name[0] == '_' || name[0] >= '0' && name[0] <= '9' {
		name = "K" + name
	}
	switch name {
	case "MESSAGE", "PRIORITY", "SYSLOG_IDENTIFIER":
		name = "KV_" + name
	}
	return name
}

// logAt logs msg at l with alternating key/value attributes.
func logAt(fnName string, l slog.Level, msg string, kv []MalType) (MalType, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("%s: odd number of key/value arguments", fnName)
	}
	if l < level.Level() {
		return nil, nil
	}
	keys := make([]string, 0, len(kv)/2)
	vals := make([]any, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key, err := logKey(fnName, kv[i])
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		vals = append(vals, logValue(kv[i+1]))
	}

	mu.Lock()
	defer mu.Unlock()
	s, err := resolve()
	if err != nil {
		return nil, err
	}
	if s.journald {
		fields := make(map[string]string, len(keys)+len(runFields)+1)
		fields["SYSLOG_IDENTIFIER"] = identifier
		for k, v := range runFields {
			fields[fieldName(k)] = v
		}
		for i, k := range keys {
			fields[fieldName(k)] = fmt.Sprint(vals[i])
		}
		if err := journalSend(msg, priority(l), fields); err != nil {
			return nil, fmt.Errorf("%s: %w", fnName, err)
		}
		return nil, nil
	}
	attrs := make([]any, 0, 2*(len(keys)+len(runFields)))
	for k, v := range runFields {
		attrs = append(attrs, k, v)
	}
	for i, k := range keys {
		attrs = append(attrs, k, vals[i])
	}
	s.logger.Log(context.Background(), l, msg, attrs...)
	return nil, nil
}

func logDebug(msg string, kv ...MalType) (MalType, error) {
	return logAt("log-debug", slog.LevelDebug, msg, kv)
}

func logInfo(msg string, kv ...MalType) (MalType, error) {
	return logAt("log-info", slog.LevelInfo, msg, kv)
}

func logWarn(msg string, kv ...MalType) (MalType, error) {
	return logAt("log-warn", slog.LevelWarn, msg, kv)
}

func logError(msg string, kv ...MalType) (MalType, error) {
	return logAt("log-error", slog.LevelError, msg, kv)
}

// Load registers the log builtins and reads LOG_LEVEL.
func Load(env EnvType) {
	level.Set(parseLevel(os.Getenv("LOG_LEVEL")))

	call.CallOverrideFN(env, "log-debug", logDebug)
	call.CallOverrideFN(env, "log-info", logInfo)
	call.CallOverrideFN(env, "log-warn", logWarn)
	call.CallOverrideFN(env, "log-error", logError)

	call.Doc(env, "log-debug", "[msg & kv]",
		"Emits a structured log record at debug level with alternating keyword/value attributes; suppressed unless LOG_LEVEL=debug. Records go to systemd-journald, or to $XDG_STATE_HOME/lisp/<script>.log without it.")
	call.Doc(env, "log-info", "[msg & kv]",
		"Emits a structured log record at info level with alternating keyword/value attributes, e.g. (log-info \"user created\" :id 42). Records go to systemd-journald, or to $XDG_STATE_HOME/lisp/<script>.log without it.")
	call.Doc(env, "log-warn", "[msg & kv]",
		"Emits a structured log record at warn level with alternating keyword/value attributes. Records go to systemd-journald, or to $XDG_STATE_HOME/lisp/<script>.log without it.")
	call.Doc(env, "log-error", "[msg & kv]",
		"Emits a structured log record at error level with alternating keyword/value attributes. Records go to systemd-journald, or to $XDG_STATE_HOME/lisp/<script>.log without it.")
}
