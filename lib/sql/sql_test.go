package sql

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"

	_ "modernc.org/sqlite"
)

func kw(name string) types.Keyword { return types.KW(name) }

func paramMap(kv map[string]types.MalType) []types.MalType {
	hm := types.HashMap{Items: map[types.MalType]types.MalType{}}
	for k, v := range kv {
		hm.Items[kw(k)] = v
	}
	return []types.MalType{hm}
}

func TestRewrite(t *testing.T) {
	p := paramMap(map[string]types.MalType{"id": 1, "nm": "x"})

	cases := []struct {
		name     string
		query    string
		bind     bindKind
		params   []types.MalType
		wantStmt string
		wantArgs []any
	}{
		{"question", "a = :id AND b = :nm", bindQuestion, p, "a = ? AND b = ?", []any{1, "x"}},
		{"dollar", "a = :id AND b = :nm", bindDollar, p, "a = $1 AND b = $2", []any{1, "x"}},
		{"repeated question", ":id = :id", bindQuestion, p, "? = ?", []any{1, 1}},
		{"repeated dollar", ":id = :id", bindDollar, p, "$1 = $2", []any{1, 1}},
		{"pg cast skipped", "x::text = :id", bindQuestion, p, "x::text = ?", []any{1}},
		{"string literal untouched", "name = ':id' OR id = :id", bindQuestion, p, "name = ':id' OR id = ?", []any{1}},
		{"line comment untouched", "-- :id here\nid = :id", bindQuestion, p, "-- :id here\nid = ?", []any{1}},
		{"block comment untouched", "/* :id */ id = :id", bindQuestion, p, "/* :id */ id = ?", []any{1}},
		{"no params", "SELECT 1", bindQuestion, nil, "SELECT 1", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, args, err := rewrite(tc.query, tc.bind, tc.params)
			if err != nil {
				t.Fatalf("rewrite error: %v", err)
			}
			if stmt != tc.wantStmt {
				t.Errorf("stmt = %q, want %q", stmt, tc.wantStmt)
			}
			if len(args) != len(tc.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tc.wantArgs)
			}
			for i := range args {
				if args[i] != tc.wantArgs[i] {
					t.Errorf("arg[%d] = %v, want %v", i, args[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestRewriteMissingParam(t *testing.T) {
	_, _, err := rewrite("id = :missing", bindQuestion, paramMap(map[string]types.MalType{"id": 1}))
	if err == nil {
		t.Fatal("expected an error for a missing parameter")
	}
}

// testEnv builds an interpreter with core, the basic header (defn/defmacro
// helpers) and the SQL namespace loaded.
func testEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	if _, err := lisp.REPL(context.Background(), ns, core.HeaderBasic(), types.NewCursorFile("preamble")); err != nil {
		t.Fatalf("HeaderBasic: %v", err)
	}
	Load(ns)
	if _, err := lisp.REPL(context.Background(), ns, HeaderSQL(), types.NewCursorFile("header-sql")); err != nil {
		t.Fatalf("HeaderSQL: %v", err)
	}
	return ns
}

func eval(t *testing.T, ns types.EnvType, src string) types.MalType {
	t.Helper()
	res, err := evalErr(ns, src)
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	return res
}

// evalErr reads and evaluates src, returning the raw value (REPL would
// return its printed form instead).
func evalErr(ns types.EnvType, src string) (types.MalType, error) {
	ast, err := lisp.READ(src, types.NewCursorFile("test"), ns)
	if err != nil {
		return nil, err
	}
	return lisp.EVAL(context.Background(), ast, ns)
}

func TestEndToEnd(t *testing.T) {
	ns := testEnv(t)
	dbPath := filepath.Join(t.TempDir(), "t.db")

	eval(t, ns, `(def db (sql-open "sqlite" "`+dbPath+`"))`)
	eval(t, ns, `(sql-exec db "CREATE TABLE acct (id INTEGER PRIMARY KEY, name TEXT, bal INTEGER)")`)

	// insert with named params (driver-agnostic placeholders)
	ins := eval(t, ns, `(sql-exec db "INSERT INTO acct (id, name, bal) VALUES (:id, :name, :bal)" {:id 1 :name "ada" :bal 100})`)
	hm, ok := ins.(types.HashMap)
	if !ok {
		t.Fatalf("sql-exec returned %T, want HashMap", ins)
	}
	if ra := hm.Items[kw("rows-affected")]; ra != 1 {
		t.Errorf("rows-affected = %v, want 1", ra)
	}

	eval(t, ns, `(sql-exec db "INSERT INTO acct (id, name, bal) VALUES (:id, :name, :bal)" {:id 2 :name "bob" :bal 50})`)

	// query returns a vector of keyword-keyed maps
	rows := eval(t, ns, `(sql-query db "SELECT id, name FROM acct ORDER BY id")`)
	vec, ok := rows.(types.Vector)
	if !ok {
		t.Fatalf("sql-query returned %T, want Vector", rows)
	}
	if len(vec.Val) != 2 {
		t.Fatalf("got %d rows, want 2", len(vec.Val))
	}
	first := vec.Val[0].(types.HashMap)
	if first.Items[kw("id")] != 1 || first.Items[kw("name")] != "ada" {
		t.Errorf("row 0 = %v, want {:id 1 :name ada}", first.Items)
	}

	// query-one with a named param
	one := eval(t, ns, `(sql-query-one db "SELECT name FROM acct WHERE id = :id" {:id 2})`)
	if got := one.(types.HashMap).Items[kw("name")]; got != "bob" {
		t.Errorf("query-one name = %v, want bob", got)
	}

	// query-one with no match returns nil
	if none := eval(t, ns, `(sql-query-one db "SELECT * FROM acct WHERE id = :id" {:id 99})`); none != nil {
		t.Errorf("expected nil for no match, got %v", none)
	}
}

func TestTransactionCommit(t *testing.T) {
	ns := testEnv(t)
	dbPath := filepath.Join(t.TempDir(), "t.db")
	eval(t, ns, `(def db (sql-open "sqlite" "`+dbPath+`"))`)
	eval(t, ns, `(sql-exec db "CREATE TABLE t (id INTEGER)")`)

	eval(t, ns, `(with-tx [tx db]
	                (sql-exec tx "INSERT INTO t (id) VALUES (:id)" {:id 1})
	                (sql-exec tx "INSERT INTO t (id) VALUES (:id)" {:id 2}))`)

	n := eval(t, ns, `(get (sql-query-one db "SELECT count(*) AS n FROM t") :n)`)
	if n != 2 {
		t.Errorf("after commit count = %v, want 2", n)
	}
}

func TestTransactionRollback(t *testing.T) {
	ns := testEnv(t)
	dbPath := filepath.Join(t.TempDir(), "t.db")
	eval(t, ns, `(def db (sql-open "sqlite" "`+dbPath+`"))`)
	eval(t, ns, `(sql-exec db "CREATE TABLE t (id INTEGER)")`)

	// the body throws after one insert; the whole transaction must roll back
	_, err := lisp.REPL(context.Background(), ns, `(with-tx [tx db]
	                (sql-exec tx "INSERT INTO t (id) VALUES (:id)" {:id 1})
	                (throw "boom"))`, types.NewCursorFile("test"))
	if err == nil {
		t.Fatal("expected the thrown error to propagate")
	}

	n := eval(t, ns, `(get (sql-query-one db "SELECT count(*) AS n FROM t") :n)`)
	if n != 0 {
		t.Errorf("after rollback count = %v, want 0", n)
	}
}

func TestWithOpenCloses(t *testing.T) {
	ns := testEnv(t)
	dbPath := filepath.Join(t.TempDir(), "t.db")
	got := eval(t, ns, `(with-open [db (sql-open "sqlite" "`+dbPath+`")]
	                      (sql-exec db "CREATE TABLE t (id INTEGER)")
	                      (sql-exec db "INSERT INTO t (id) VALUES (:id)" {:id 7})
	                      (get (sql-query-one db "SELECT id FROM t") :id))`)
	if got != 7 {
		t.Errorf("with-open result = %v, want 7", got)
	}
}
