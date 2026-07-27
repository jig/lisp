# lib/log — structured logging

Structured logging for jig/lisp scripts. Each call emits one record:

```clojure
(log-info "user created" :id 42 :name "ada")
```

## Destination

Resolved once, at the first record — never to the screen, and not
configurable by environment (by design):

- **systemd-journald** when its socket is available (Linux). Records
  are native journal fields: `MESSAGE`, `PRIORITY` (mapped from the
  level), `SYSLOG_IDENTIFIER` (the script basename) and each attribute
  uppercased (`:trace-id` → `TRACE_ID`). The journal is append-only
  storage the writing process cannot alter or delete. Read back with:

  ```sh
  journalctl -t myscript -o json | lm
  ```

- **XDG state file** otherwise (macOS, Linux without systemd):
  JSON lines appended to `$XDG_STATE_HOME/lisp/<script>.log` —
  `~/.local/state/lisp/<script>.log` by default:

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
themselves; anything else (vectors, maps, errors…) is rendered with
the Lisp printer. An odd number of `kv` arguments is an error.

## Level filtering

The minimum level comes from the `LOG_LEVEL` environment variable —
`debug`, `info`, `warn` or `error` — read when the namespace loads.
Unset (or unrecognised) means `info`, so `log-debug` records are
suppressed by default:

```sh
LOG_LEVEL=debug lisp service.lisp
```
