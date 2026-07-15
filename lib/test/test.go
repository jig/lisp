// Package test implements the deftest/is/are unit-testing library.
//
// The lisp-facing surface lives in header-test.lisp (deftest, is, are
// macros); this file provides the Go builtins they expand into and the
// registry the CLI test runner consumes. A Registry is created per
// environment by Load and stored under test/*registry*, so embedders and
// the command-line runner can retrieve it with FromEnv.
package test

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/printer"
	. "github.com/jig/lisp/types"
)

//go:embed header-test.lisp
var headerTest string

// HeaderTest returns the lisp source of the deftest/is/are macros.
func HeaderTest() string { return headerTest }

// registrySymbol is the environment binding under which Load stores the
// per-environment Registry.
const registrySymbol = "test/*registry*"

// Check is the outcome of a single is/are assertion.
type Check struct {
	Form     string // source form, printed
	OK       bool
	Err      string // evaluation error, if any
	Expected string // printed expected value ((is (= expected actual)) only)
	Actual   string // printed actual value
	Message  string // optional user message
	Module   string
	Line     int
}

// Test is one deftest with its accumulated checks.
type Test struct {
	Name   string
	Module string
	Line   int
	Fn     MalType // the zero-argument test function
	Err    string  // error escaping the test body, if any
	Checks []Check
}

// OK reports whether the test ran without an escaped error and every
// check passed.
func (t *Test) OK() bool {
	if t.Err != "" {
		return false
	}
	for _, c := range t.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

// Registry accumulates deftest registrations and, while running, the
// checks they record.
type Registry struct {
	mu      sync.Mutex
	tests   []*Test
	current *Test
}

// FromEnv returns the Registry stored in env by Load, or nil.
func FromEnv(env EnvType) *Registry {
	v, err := env.Get(Symbol{Val: registrySymbol})
	if err != nil {
		return nil
	}
	reg, _ := v.(*Registry)
	return reg
}

// Tests returns the registered tests, in registration order.
func (r *Registry) Tests() []*Test {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Test, len(r.tests))
	copy(out, r.tests)
	return out
}

func (r *Registry) register(name string, fn MalType, module string, line int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := &Test{Name: name, Fn: fn, Module: module, Line: line}
	// Re-registering a name replaces the previous test, so reloading a
	// file in a live session stays idempotent.
	for i, prev := range r.tests {
		if prev.Name == name {
			r.tests[i] = t
			return
		}
	}
	r.tests = append(r.tests, t)
}

func (r *Registry) record(c Check) (inTest bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return false
	}
	r.current.Checks = append(r.current.Checks, c)
	return true
}

// runOne executes a single test, recording its checks. It is the shared
// core of RunAll and RunOne.
func (r *Registry) runOne(ctx context.Context, t *Test) {
	r.mu.Lock()
	t.Err = ""
	t.Checks = nil
	r.current = t
	r.mu.Unlock()
	if _, err := Apply(ctx, t.Fn, nil); err != nil {
		t.Err = err.Error()
	}
	r.mu.Lock()
	r.current = nil
	r.mu.Unlock()
}

// RunAll executes every registered test sequentially and returns them
// with their results.
func (r *Registry) RunAll(ctx context.Context) []*Test {
	for _, t := range r.Tests() {
		r.runOne(ctx, t)
	}
	return r.Tests()
}

// RunOne executes the single test named name and returns it, or nil when
// no such test is registered. Evaluating the test body here (rather than
// in a spawned runner process) is what lets the DAP debugger stop at
// breakpoints inside it.
func (r *Registry) RunOne(ctx context.Context, name string) *Test {
	for _, t := range r.Tests() {
		if t.Name == name {
			r.runOne(ctx, t)
			return t
		}
	}
	return nil
}

// testsAsData converts results to lisp data for test/run-tests!.
func testsAsData(tests []*Test) MalType {
	out := make([]MalType, 0, len(tests))
	for _, t := range tests {
		checks := make([]MalType, 0, len(t.Checks))
		for _, c := range t.Checks {
			m := map[string]MalType{
				NewKeyword("form"): c.Form,
				NewKeyword("ok"):   c.OK,
			}
			if c.Err != "" {
				m[NewKeyword("error")] = c.Err
			}
			if !c.OK && c.Expected != "" {
				m[NewKeyword("expected")] = c.Expected
				m[NewKeyword("actual")] = c.Actual
			}
			if c.Message != "" {
				m[NewKeyword("message")] = c.Message
			}
			checks = append(checks, HashMap{Val: m})
		}
		m := map[string]MalType{
			NewKeyword("name"):   t.Name,
			NewKeyword("ok"):     t.OK(),
			NewKeyword("checks"): Vector{Val: checks},
		}
		if t.Err != "" {
			m[NewKeyword("error")] = t.Err
		}
		out = append(out, HashMap{Val: m})
	}
	return Vector{Val: out}
}

func position(form MalType) (module string, line int) {
	var cur *Position
	switch f := form.(type) {
	case List:
		cur = f.Cursor
	case Vector:
		cur = f.Cursor
	case Symbol:
		cur = f.Cursor
	case MalFunc:
		cur = f.Cursor
	}
	if cur == nil {
		return "", 0
	}
	if cur.Module != nil {
		module = *cur.Module
	}
	return module, cur.BeginRow
}

func truthy(v MalType) bool { return v != nil && v != false }

func firstMessage(msg []MalType) string {
	if len(msg) == 0 {
		return ""
	}
	if s, ok := msg[0].(string); ok {
		return s
	}
	return printer.Pr_str(msg[0], true)
}

// Load registers the test builtins and macros on env.
func Load(env EnvType) error {
	reg := &Registry{}
	env.Set(Symbol{Val: registrySymbol}, reg)

	register := func(name MalType, fn MalType) (MalType, error) {
		var testName string
		switch n := name.(type) {
		case Symbol:
			testName = n.Val
		case string:
			testName = n
		default:
			return nil, fmt.Errorf("deftest: name must be a symbol (was of type %T)", name)
		}
		module, line := position(name)
		if module == "" && line == 0 {
			module, line = position(fn)
		}
		reg.register(testName, fn, module, line)
		return Symbol{Val: testName}, nil
	}
	call.CallOverrideFN(env, "test/register!", register)
	call.Doc(env, "test/register!", "[name fn]", "Registers fn as the test named name; deftest expands to this.")

	check := func(ctx context.Context, form MalType, thunk MalType, msg ...MalType) (MalType, error) {
		module, line := position(form)
		c := Check{Form: printer.Pr_str(form, true), Message: firstMessage(msg), Module: module, Line: line}
		val, err := Apply(ctx, thunk, nil)
		if err != nil {
			c.Err = err.Error()
		}
		c.OK = err == nil && truthy(val)
		if reg.record(c) {
			return c.OK, nil
		}
		// Outside deftest an is failure throws, so plain scripts and the
		// legacy *_test.mal suites fail loudly.
		if !c.OK {
			if err != nil {
				return nil, err
			}
			return nil, lisperror.NewLispError(fmt.Errorf("assertion failed: %s", c.Form), form)
		}
		return val, nil
	}
	call.CallOverrideFN(env, "test/check!", check)
	call.Doc(env, "test/check!", "[form thunk & msg]", "Records form's outcome in the running test; (is …) expands to this.")

	checkEq := func(ctx context.Context, form MalType, expected MalType, actual MalType, msg ...MalType) (MalType, error) {
		module, line := position(form)
		c := Check{Form: printer.Pr_str(form, true), Message: firstMessage(msg), Module: module, Line: line}
		expVal, expErr := Apply(ctx, expected, nil)
		actVal, actErr := Apply(ctx, actual, nil)
		switch {
		case expErr != nil:
			c.Err = expErr.Error()
		case actErr != nil:
			c.Err = actErr.Error()
		default:
			c.OK = Equal_Q(expVal, actVal)
			if !c.OK {
				c.Expected = printer.Pr_str(expVal, true)
				c.Actual = printer.Pr_str(actVal, true)
			}
		}
		if reg.record(c) {
			return c.OK, nil
		}
		if !c.OK {
			if c.Err != "" {
				if expErr != nil {
					return nil, expErr
				}
				return nil, actErr
			}
			return nil, lisperror.NewLispError(fmt.Errorf("assertion failed: %s: expected %s, got %s", c.Form, c.Expected, c.Actual), form)
		}
		return true, nil
	}
	call.CallOverrideFN(env, "test/check-eq!", checkEq)
	call.Doc(env, "test/check-eq!", "[form expected-thunk actual-thunk & msg]", "Records an equality check with expected/actual reporting; (is (= a b)) expands to this.")

	runTests := func(ctx context.Context) (MalType, error) {
		return testsAsData(reg.RunAll(ctx)), nil
	}
	call.CallOverrideFN(env, "test/run-tests!", runTests)
	call.Doc(env, "test/run-tests!", "[]", "Runs every registered test and returns the results as data.")

	runTest := func(ctx context.Context, name string) (MalType, error) {
		t := reg.RunOne(ctx, name)
		if t == nil {
			return nil, lisperror.NewLispError(fmt.Errorf("no test named %q", name), nil)
		}
		return testsAsData([]*Test{t}), nil
	}
	call.CallOverrideFN(env, "test/run-test!", runTest)
	call.Doc(env, "test/run-test!", "[name]", "Runs the single registered test named name (used by the editor's Debug Test); returns its result as data.")

	expandAre := func(argv MalType, expr MalType, rows MalType) (MalType, error) {
		return ExpandAre(argv, expr, rows)
	}
	call.CallOverrideFN(env, "test/expand-are", expandAre)
	call.Doc(env, "test/expand-are", "[argv expr rows]", "Macro helper: expands an (are …) template into a do of is forms.")

	withOut := func(ctx context.Context, thunk MalType) (MalType, error) {
		return withOutStr(ctx, thunk)
	}
	call.CallOverrideFN(env, "test/with-out-str*", withOut)
	call.Doc(env, "test/with-out-str*", "[thunk]", "Runs thunk capturing standard output and returns it as a string; (with-out-str …) expands to this.")

	return nil
}

// withOutMu serialises stdout capture: os.Stdout is process-global, so
// two concurrent with-out-str calls would steal each other's output.
var withOutMu sync.Mutex

// withOutStr evaluates thunk with os.Stdout redirected to a pipe and
// returns everything printed as a string. Output from other goroutines
// (e.g. futures printing concurrently) is captured too — this is a
// testing aid, not an output-redirection facility.
func withOutStr(ctx context.Context, thunk MalType) (result MalType, err error) {
	withOutMu.Lock()
	defer withOutMu.Unlock()

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	orig := os.Stdout
	os.Stdout = w

	captured := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		captured <- buf.String()
	}()

	_, evalErr := Apply(ctx, thunk, nil)

	os.Stdout = orig
	_ = w.Close()
	out := <-captured
	_ = r.Close()

	if evalErr != nil {
		return nil, evalErr
	}
	return out, nil
}

// ExpandAre expands an (are argv expr & rows) template: rows are consumed
// in chunks of (count argv), each chunk substituted for the argv symbols
// in expr, and every instance wrapped in (is …).
func ExpandAre(argv MalType, expr MalType, rows MalType) (MalType, error) {
	symsRaw, err := GetSlice(argv)
	if err != nil {
		return nil, fmt.Errorf("are: first argument must be a binding vector")
	}
	syms := make([]string, len(symsRaw))
	for i, s := range symsRaw {
		sym, ok := s.(Symbol)
		if !ok {
			return nil, fmt.Errorf("are: binding vector must hold symbols (got %T)", s)
		}
		syms[i] = sym.Val
	}
	if len(syms) == 0 {
		return nil, fmt.Errorf("are: binding vector cannot be empty")
	}
	vals, err := GetSlice(rows)
	if err != nil {
		return nil, err
	}
	if len(vals)%len(syms) != 0 {
		return nil, fmt.Errorf("are: %d values do not fill rows of %d", len(vals), len(syms))
	}
	forms := []MalType{Symbol{Val: "do"}}
	for i := 0; i < len(vals); i += len(syms) {
		subst := map[string]MalType{}
		for j, name := range syms {
			subst[name] = vals[i+j]
		}
		forms = append(forms, List{Val: []MalType{Symbol{Val: "is"}, substitute(expr, subst)}})
	}
	return List{Val: forms}, nil
}

// substitute returns expr with every symbol found in subst replaced.
func substitute(expr MalType, subst map[string]MalType) MalType {
	switch e := expr.(type) {
	case Symbol:
		if v, ok := subst[e.Val]; ok {
			return v
		}
		return e
	case List:
		out := make([]MalType, len(e.Val))
		for i, c := range e.Val {
			out[i] = substitute(c, subst)
		}
		return List{Val: out, Cursor: e.Cursor}
	case Vector:
		out := make([]MalType, len(e.Val))
		for i, c := range e.Val {
			out[i] = substitute(c, subst)
		}
		return Vector{Val: out, Cursor: e.Cursor}
	case HashMap:
		out := make(map[string]MalType, len(e.Val))
		for k, v := range e.Val {
			out[k] = substitute(v, subst)
		}
		return HashMap{Val: out, Cursor: e.Cursor}
	default:
		return expr
	}
}
