# lib/log — structured JSON logging

Structured logging for jig/lisp scripts, backed by Go's `log/slog`.
Each call emits one JSON line to stderr:

```clojure
(log-info "user created" :id 42 :name "ada")
```

```json
{"time":"2026-07-25T15:04:05Z","level":"INFO","msg":"user created","id":42,"name":"ada"}
```

## Builtins

| Function | Signature | Level |
|---|---|---|
| `log-debug` | `[msg & kv]` | DEBUG |
| `log-info` | `[msg & kv]` | INFO |
| `log-warn` | `[msg & kv]` | WARN |
| `log-error` | `[msg & kv]` | ERROR |

`kv` are alternating key/value pairs. Keys are keywords (or strings);
keywords lose their sigil in the output (`:id` → `"id"`). Primitive
values (strings, numbers, booleans, nil, keywords) pass through as
JSON; anything else (vectors, maps, errors…) is rendered with the Lisp
printer. An odd number of `kv` arguments is an error.

## Level filtering

The minimum level comes from the `LOG_LEVEL` environment variable —
`debug`, `info`, `warn` or `error` — read when the namespace loads.
Unset (or unrecognised) means `info`, so `log-debug` lines are
suppressed by default:

```sh
LOG_LEVEL=debug lisp run service.lisp
```
