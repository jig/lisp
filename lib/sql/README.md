# lib/sql — SQL databases for jig/lisp

A small, **driver-agnostic** SQL layer over Go's `database/sql`. Queries use
named placeholders (`:name`) bound from a map; the library rewrites them to
the driver's dialect (`?` for SQLite/MySQL, `$1..$n` for PostgreSQL), so your
lisp code never hard-codes a placeholder style. Rows come back as a vector of
maps keyed by keyword column names, and transactions are scoped with a macro.

## Loading

The ready-to-use namespace lives in `lib/sql/nssql`, which also bundles two
pure-Go drivers (no cgo, so cross-compiled binaries need no C toolchain):

| Driver name | Package | Database |
|-------------|---------|----------|
| `sqlite`    | `modernc.org/sqlite`        | SQLite     |
| `pgx`       | `github.com/jackc/pgx/v5`   | PostgreSQL |

`cmd/lisp` loads it by default. To embed it yourself:

```go
import "github.com/jig/lisp/lib/sql/nssql"

ns := env.NewEnv()
// ... load core and the other namespaces ...
if err := nssql.Load(ns); err != nil { /* ... */ }
```

Need another database (MySQL, etc.)? Blank-import its driver next to your
program and call `sql-open` with that driver's name — `lib/sql` itself is
driver-neutral; `nssql` only decides which drivers ship in the binary.

## Quick example

```clojure
(with-open [db (sql-open "sqlite" "app.db")]
  (sql-exec db "CREATE TABLE acct (id INTEGER PRIMARY KEY, name TEXT, bal INTEGER)")

  (with-tx [tx db]
    (sql-exec tx "INSERT INTO acct (id, name, bal) VALUES (:id, :name, :bal)"
              {:id 1 :name "ada" :bal 100})
    (sql-exec tx "UPDATE acct SET bal = bal - :d WHERE id = :id" {:d 30 :id 1}))

  (sql-query db "SELECT * FROM acct WHERE bal > :min" {:min 50}))
;; => [{:id 1 :name "ada" :bal 70}]
```

The exact same code runs on PostgreSQL — only the `sql-open` arguments change:

```clojure
(sql-open "pgx" "postgres://user:pass@localhost:5432/app")
```

## API

| Form | Result |
|------|--------|
| `(sql-open driver dsn)` | an opaque **db handle** (remembers the driver's placeholder dialect) |
| `(sql-close db)` | closes the handle; returns `nil` |
| `(sql-exec db-or-tx sql params?)` | runs a statement (INSERT/UPDATE/DDL); returns `{:rows-affected n :last-insert-id m}` |
| `(sql-query db-or-tx sql params?)` | runs a query; returns a **vector of maps**, one per row, keyed by keyword column names |
| `(sql-query-one db-or-tx sql params?)` | like `sql-query` but returns the first row map, or `nil` |
| `(with-tx [tx db] body…)` | runs `body` in a transaction; commits on success, rolls back if it throws |
| `(with-open [name (sql-open …)] body…)` | binds `name`, guarantees `sql-close` on exit or throw |

`sql-exec`, `sql-query` and `sql-query-one` accept **either a db handle or a
tx** as their first argument, so the same helpers work inside and outside a
transaction. The trailing `params` map is optional when the SQL has no
placeholders.

## Named parameters

Write `:name` in the SQL and pass a map of values. Order does not matter, keys
are self-documenting, and the same query is portable across drivers:

```clojure
(sql-query db "SELECT * FROM acct WHERE id = :id AND bal > :min"
           {:id 1 :min 50})
```

The rewriter is dialect-aware: it skips `'string literals'`, `"quoted
identifiers"`, `-- line` and `/* block */` comments, and PostgreSQL `::casts`,
so a colon in any of those is never mistaken for a parameter. A `:name` with
no matching key raises an error.

## Transactions

`with-tx` binds the transaction and controls its lifetime by the body's
outcome:

```clojure
(with-tx [tx db]
  (sql-exec tx "INSERT INTO t (id) VALUES (:id)" {:id 1})
  (when (broke?) (throw "abort"))      ; any throw rolls the whole tx back
  (sql-exec tx "INSERT INTO t (id) VALUES (:id)" {:id 2}))
;; commits only if the body returns normally
```

A thrown error (or a panic) rolls back and re-propagates; a clean return
commits. The transaction is bound to the evaluation context, so it is
cancelled if the context is.

## Type mapping

| SQL | lisp |
|-----|------|
| NULL | `nil` |
| INTEGER | int |
| REAL / FLOAT | float |
| TEXT | string |
| BOOLEAN | bool |
| BLOB | binary (`[]byte`) |
| DATE / TIME / TIMESTAMP | string (RFC 3339) |

Parameter values are converted the other way with the same correspondence.

## Limitations (v1)

- **Binding is abstracted, SQL syntax is not.** Placeholders and value types
  are portable; dialect-specific *syntax* (`RETURNING`, `LIMIT/OFFSET`,
  upserts, autoincrement types, …) is still yours to write. Stick to the
  common subset for portable code.
- `:last-insert-id` is `nil` on PostgreSQL (it has no `LastInsertId`); use
  `RETURNING` with `sql-query` instead.
- No connection-pool tuning is exposed yet (`SetMaxOpenConns`, etc.).
- A data-DSL (queries as lisp maps compiled per dialect) could later sit on
  top of this layer; it is not part of v1.
