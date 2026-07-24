// Package sql exposes database/sql to jig/lisp as a small, driver-agnostic
// set of builtins. Queries use named placeholders (:name) bound from a
// parameter map; the library rewrites them to the driver's dialect (? for
// SQLite/MySQL, $1..$n for PostgreSQL) so the lisp code never hard-codes a
// placeholder style. Rows come back as a vector of maps keyed by keyword
// column names, and transactions are scoped with the with-tx macro.
package sql

import (
	"context"
	stdsql "database/sql"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

//go:embed header-sql.lisp
var headerSQL string

// HeaderSQL returns the lisp-defined part of the namespace (the with-tx and
// with-open macros), loaded by nssql.
func HeaderSQL() string { return headerSQL }

// bindKind is a driver's placeholder dialect.
type bindKind int

const (
	bindQuestion bindKind = iota // ? — SQLite, MySQL
	bindDollar                   // $1..$n — PostgreSQL
)

// bindFor picks a placeholder dialect from the driver name.
func bindFor(driver string) bindKind {
	d := strings.ToLower(driver)
	if strings.Contains(d, "postgres") || strings.Contains(d, "pgx") || d == "pq" {
		return bindDollar
	}
	return bindQuestion
}

// DB is an open database handle. It remembers its driver's placeholder
// dialect so queries can be written driver-agnostically.
type DB struct {
	db     *stdsql.DB
	bind   bindKind
	driver string
}

func (d *DB) LispPrint(_ func(MalType, bool) string) string { return "«sql-db " + d.driver + "»" }

// Tx is an in-progress transaction, yielded to a with-tx body.
type Tx struct {
	tx   *stdsql.Tx
	bind bindKind
}

func (t *Tx) LispPrint(_ func(MalType, bool) string) string { return "«sql-tx»" }

// querier is the shared subset of *sql.DB and *sql.Tx used here.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (stdsql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error)
}

// asQuerier unwraps a DB or Tx handle to its querier and bind dialect.
func asQuerier(v MalType) (querier, bindKind, error) {
	switch t := v.(type) {
	case *DB:
		return t.db, t.bind, nil
	case *Tx:
		return t.tx, t.bind, nil
	default:
		return nil, 0, fmt.Errorf("expected a SQL db or tx, got %T", v)
	}
}

// Load registers the SQL builtins in env.
func Load(env EnvType) {
	call.CallOverrideFN(env, "sql-open", sqlOpen)
	call.CallOverrideFN(env, "sql-close", sqlClose)
	call.CallOverrideFN(env, "sql-exec", sqlExec, 3, 4)
	call.CallOverrideFN(env, "sql-query", sqlQuery, 3, 4)
	call.CallOverrideFN(env, "sql-query-one", sqlQueryOne, 3, 4)
	call.CallOverrideFN(env, "sql-transact", sqlTransact)

	call.Doc(env, "sql-open", "[driver dsn]", "Opens a database and returns a handle; driver is e.g. \"sqlite\" or \"pgx\".")
	call.Doc(env, "sql-close", "[db]", "Closes a database handle.")
	call.Doc(env, "sql-exec", "[db-or-tx sql & params]", "Runs a statement (INSERT/UPDATE/DDL); returns {:rows-affected n :last-insert-id m}.")
	call.Doc(env, "sql-query", "[db-or-tx sql & params]", "Runs a query; returns a vector of maps, one per row, keyed by keyword column names.")
	call.Doc(env, "sql-query-one", "[db-or-tx sql & params]", "Like sql-query but returns the first row map, or nil.")
	call.Doc(env, "sql-transact", "[db fn]", "Runs (fn tx) inside a transaction: commits on success, rolls back if it throws.")
}

func sqlOpen(ctx context.Context, driver, dsn string) (MalType, error) {
	db, err := stdsql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db, bind: bindFor(driver), driver: driver}, nil
}

func sqlClose(_ context.Context, dbv MalType) (MalType, error) {
	db, ok := dbv.(*DB)
	if !ok {
		return nil, fmt.Errorf("sql-close: expected a db handle, got %T", dbv)
	}
	return nil, db.db.Close()
}

func sqlExec(ctx context.Context, target MalType, query string, params ...MalType) (MalType, error) {
	q, bind, err := asQuerier(target)
	if err != nil {
		return nil, err
	}
	stmt, args, err := rewrite(query, bind, params)
	if err != nil {
		return nil, err
	}
	res, err := q.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	out := HashMap{Items: map[MalType]MalType{}}
	if ra, err := res.RowsAffected(); err == nil {
		out.Items[NewKeyword("rows-affected")] = int(ra)
	} else {
		out.Items[NewKeyword("rows-affected")] = nil
	}
	if li, err := res.LastInsertId(); err == nil {
		out.Items[NewKeyword("last-insert-id")] = int(li)
	} else {
		// Not all drivers (e.g. PostgreSQL) support LastInsertId; use RETURNING.
		out.Items[NewKeyword("last-insert-id")] = nil
	}
	return out, nil
}

func sqlQuery(ctx context.Context, target MalType, query string, params ...MalType) (MalType, error) {
	q, bind, err := asQuerier(target)
	if err != nil {
		return nil, err
	}
	stmt, args, err := rewrite(query, bind, params)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return rowsToVector(rows)
}

func sqlQueryOne(ctx context.Context, target MalType, query string, params ...MalType) (MalType, error) {
	v, err := sqlQuery(ctx, target, query, params...)
	if err != nil {
		return nil, err
	}
	rows := v.(Vector).Val
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// sqlTransact begins a transaction, applies body to the Tx, and commits on
// success or rolls back if body returns an error or panics.
func sqlTransact(ctx context.Context, dbv MalType, body MalType) (result MalType, err error) {
	db, ok := dbv.(*DB)
	if !ok {
		return nil, fmt.Errorf("sql-transact: first argument must be a db handle, got %T", dbv)
	}
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback() // no-op after a successful commit
		}
	}()
	res, err := Apply(ctx, body, []MalType{&Tx{tx: tx, bind: db.bind}})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return res, nil
}

// rowsToVector materialises a result set as a vector of keyword-keyed maps.
func rowsToVector(rows *stdsql.Rows) (MalType, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []MalType{}
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := HashMap{Items: make(map[MalType]MalType, len(cols))}
		for i, col := range cols {
			row.Items[NewKeyword(col)] = fromDriver(cells[i])
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return Vector{Val: out}, nil
}

// fromDriver converts a database/sql scanned value to a lisp value.
func fromDriver(v any) MalType {
	switch t := v.(type) {
	case nil:
		return nil
	case int64:
		return int(t)
	case int32:
		return int(t)
	case int:
		return t
	case float64:
		return t
	case float32:
		return float64(t)
	case bool:
		return t
	case string:
		return t
	case []byte:
		return t // binary column; text arrives as string
	case time.Time:
		return t.Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// toDriver converts a lisp parameter value to a database/sql argument.
func toDriver(v MalType) any {
	switch t := v.(type) {
	case nil:
		return nil
	case Keyword:
		return string(t)
	case string:
		return t
	default:
		return t // int, float64, bool, []byte pass through
	}
}

// lookup finds a named parameter, accepting either a keyword key (:name,
// the idiomatic form) or a plain string key ("name").
func lookup(args map[MalType]MalType, name string) (MalType, bool) {
	if v, ok := args[NewKeyword(name)]; ok {
		return v, true
	}
	v, ok := args[name]
	return v, ok
}

// namedArgs extracts the optional trailing parameter map.
func namedArgs(params []MalType) (map[MalType]MalType, error) {
	switch len(params) {
	case 0:
		return nil, nil
	case 1:
		hm, ok := params[0].(HashMap)
		if !ok {
			return nil, fmt.Errorf("SQL parameters must be a map, got %T", params[0])
		}
		return hm.Items, nil
	default:
		return nil, fmt.Errorf("expected a single parameter map, got %d arguments", len(params))
	}
}

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool  { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// rewrite converts :name placeholders to the driver's dialect and returns
// the rewritten statement plus the ordered argument list. It skips string
// literals, quoted identifiers, -- and /* */ comments, and PostgreSQL ::
// casts so a colon there is never mistaken for a parameter.
func rewrite(query string, bind bindKind, params []MalType) (string, []any, error) {
	args, err := namedArgs(params)
	if err != nil {
		return "", nil, err
	}
	rs := []rune(query)
	var b strings.Builder
	var out []any
	n := 0
	i := 0
	for i < len(rs) {
		c := rs[i]
		switch {
		case c == '\'': // string literal ('' escapes a quote)
			b.WriteRune(c)
			i++
			for i < len(rs) {
				b.WriteRune(rs[i])
				if rs[i] == '\'' {
					if i+1 < len(rs) && rs[i+1] == '\'' {
						b.WriteRune(rs[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '"': // quoted identifier
			b.WriteRune(c)
			i++
			for i < len(rs) {
				b.WriteRune(rs[i])
				if rs[i] == '"' {
					i++
					break
				}
				i++
			}
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-': // line comment
			for i < len(rs) && rs[i] != '\n' {
				b.WriteRune(rs[i])
				i++
			}
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*': // block comment
			b.WriteString("/*")
			i += 2
			for i < len(rs) {
				if rs[i] == '*' && i+1 < len(rs) && rs[i+1] == '/' {
					b.WriteString("*/")
					i += 2
					break
				}
				b.WriteRune(rs[i])
				i++
			}
		case c == ':' && i+1 < len(rs) && rs[i+1] == ':': // PostgreSQL cast
			b.WriteString("::")
			i += 2
		case c == ':' && i+1 < len(rs) && isIdentStart(rs[i+1]):
			i++ // consume ':'
			start := i
			for i < len(rs) && isIdentPart(rs[i]) {
				i++
			}
			name := string(rs[start:i])
			val, ok := lookup(args, name)
			if !ok {
				return "", nil, fmt.Errorf("missing SQL parameter :%s", name)
			}
			out = append(out, toDriver(val))
			n++
			if bind == bindDollar {
				b.WriteByte('$')
				b.WriteString(strconv.Itoa(n))
			} else {
				b.WriteByte('?')
			}
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String(), out, nil
}
