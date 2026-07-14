// Package docmeta is the curated documentation table for special forms
// — the constructs handled directly by EVAL's switch (do, if, let, fn,
// quote, try/catch/finally, def, defmacro, …). They are intrinsic to
// the language, never bound in an environment, so they cannot carry
// documentation as metadata the way builtins and lisp functions do;
// a small static table is the only place to describe them.
//
// Everything else is documented from the live environment instead:
// Go builtins carry {:arglists :doc} metadata attached at registration
// (see call.Doc), and lisp-defined functions/macros carry their
// parameter vector and optional docstring on their MalFunc value. That
// keeps tooling in sync with whatever an interpreter actually loads,
// including an embedder's own builtins.
//
// It is a leaf data package (no imports). A consistency test asserts
// every entry is absent from a fully loaded environment (i.e. it really
// is a special form, not a relocated builtin).
package docmeta

// Entry describes one special form.
type Entry struct {
	Params string // parameter vector, e.g. "[test then else]"
	Group  string // category for the reference documentation
	Doc    string // one-line description
}

// Kind returns the constant description shown by tooling.
func (Entry) Kind() string { return "special form" }

// SpecialForms is the curated table, keyed by name. It mirrors the
// cases of EVAL's dispatch switch; the consistency test keeps it from
// documenting anything that resolves as a value instead.
var SpecialForms = map[string]Entry{
	"def":         {"[symbol value]", "special-forms", "Binds symbol to the evaluated value in the current environment."},
	"let":         {"[bindings & body]", "special-forms", "Evaluates body in a new scope with the vector's symbol/value bindings; returns the last body form."},
	"fn":          {"[params & body]", "special-forms", "Creates an anonymous function with the given parameter vector and body."},
	"do":          {"[& body]", "special-forms", "Evaluates each form in order and returns the value of the last."},
	"if":          {"[test then else]", "special-forms", "Evaluates test; returns then when it is truthy, else otherwise (else is optional)."},
	"quote":       {"[form]", "special-forms", "Returns form unevaluated."},
	"quasiquote":  {"[form]", "special-forms", "Like quote, but ~ (unquote) and ~@ (splice-unquote) inside form are evaluated."},
	"defmacro":    {"[name fn]", "special-forms", "Binds name to a macro (a function expanded at evaluation time)."},
	"macroexpand": {"[form]", "special-forms", "Fully expands the macro call form without evaluating the result."},
	"try":         {"[expr & clauses]", "special-forms", "Evaluates expr, dispatching to a (catch …) and/or (finally …) clause on error."},
	"catch":       {"[binding & body]", "special-forms", "Inside try: binds the caught error and evaluates body."},
	"finally":     {"[& body]", "special-forms", "Inside try: body is always evaluated for side effects, error or not."},
	"context":     {"[& body]", "special-forms", "Provides a Go context to the enclosed forms."},
	"loop":        {"[bindings & body]", "special-forms", "Like let, but a recursion point: recur in tail position rebinds the bindings and jumps back, in constant stack."},
	"recur":       {"[& args]", "special-forms", "In tail position, rebinds the nearest recursion point — the enclosing loop's bindings, or the enclosing function's parameters — to args and iterates."},
}
