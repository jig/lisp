// Package log adds structured JSON logging to jig/lisp, backed by Go's
// log/slog. Four builtins — log-debug, log-info, log-warn, log-error —
// emit one JSON line to stderr with alternating key/value attributes:
//
//	(log-info "user created" :id 42)
//	{"time":"…","level":"INFO","msg":"user created","id":42}
//
// Keys are keywords (or strings); keywords lose their sigil in the
// output. Values pass through as JSON when primitive and render via the
// Lisp printer otherwise. The minimum level is set by the LOG_LEVEL
// environment variable (debug/info/warn/error; anything else, or unset,
// means info), read when the namespace loads.
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/printer"
	. "github.com/jig/lisp/types"
)

var (
	level  = new(slog.LevelVar)
	logger = newLogger(os.Stderr)
)

func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
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

// logAt logs msg at level with alternating key/value attributes.
func logAt(fnName string, level slog.Level, msg string, kv []MalType) (MalType, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("%s: odd number of key/value arguments", fnName)
	}
	attrs := make([]any, 0, len(kv))
	for i := 0; i < len(kv); i += 2 {
		key, err := logKey(fnName, kv[i])
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, key, logValue(kv[i+1]))
	}
	logger.Log(context.Background(), level, msg, attrs...)
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
		"Emits a structured JSON log line to stderr at debug level with alternating keyword/value attributes; suppressed unless LOG_LEVEL=debug.")
	call.Doc(env, "log-info", "[msg & kv]",
		"Emits a structured JSON log line to stderr at info level with alternating keyword/value attributes, e.g. (log-info \"user created\" :id 42).")
	call.Doc(env, "log-warn", "[msg & kv]",
		"Emits a structured JSON log line to stderr at warn level with alternating keyword/value attributes.")
	call.Doc(env, "log-error", "[msg & kv]",
		"Emits a structured JSON log line to stderr at error level with alternating keyword/value attributes.")
}
