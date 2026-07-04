// Package docmeta is the curated documentation table for things the
// interpreter cannot describe by itself: pure Go builtins (reflection
// exposes arity and types but not parameter names) and special forms
// (handled by EVAL's switch, never present in the environment).
//
// Lisp-defined library functions and macros are NOT listed here: their
// MalFunc value already carries real parameter names (and, when written
// with a docstring, {:doc "…"} metadata), so tooling reads them from a
// live environment instead.
//
// It is a leaf data package (no imports) so any consumer — the LSP
// server, a documentation generator, a REPL `(doc …)` — can share one
// source of truth. A consistency test asserts every Function entry
// resolves as a Go builtin in a loaded environment and every
// SpecialForm entry is absent from it.
package docmeta

// Kind classifies a documented name.
type Kind int

const (
	Function Kind = iota
	SpecialForm
)

func (k Kind) String() string {
	switch k {
	case SpecialForm:
		return "special form"
	default:
		return "function"
	}
}

// Entry describes one builtin or special form.
type Entry struct {
	Params string // parameter vector, e.g. "[map key val & kvs]"
	Kind   Kind
	Group  string // category for the reference documentation
	Doc    string // one-line description
}

// Builtins is the curated table, keyed by symbol name. Extend
// incrementally; the consistency test keeps names honest.
var Builtins = map[string]Entry{
	// --- Special forms (only documentable here) ---
	"def":         {"[symbol value]", SpecialForm, "special-forms", "Binds symbol to the evaluated value in the current environment."},
	"let":         {"[bindings & body]", SpecialForm, "special-forms", "Evaluates body in a new scope with the vector's symbol/value bindings; returns the last body form."},
	"fn":          {"[params & body]", SpecialForm, "special-forms", "Creates an anonymous function with the given parameter vector and body."},
	"do":          {"[& body]", SpecialForm, "special-forms", "Evaluates each form in order and returns the value of the last."},
	"if":          {"[test then else]", SpecialForm, "special-forms", "Evaluates test; returns then when it is truthy, else otherwise (else is optional)."},
	"quote":       {"[form]", SpecialForm, "special-forms", "Returns form unevaluated."},
	"quasiquote":  {"[form]", SpecialForm, "special-forms", "Like quote, but ~ (unquote) and ~@ (splice-unquote) inside form are evaluated."},
	"defmacro":    {"[name fn]", SpecialForm, "special-forms", "Binds name to a macro (a function expanded at evaluation time)."},
	"macroexpand": {"[form]", SpecialForm, "special-forms", "Fully expands the macro call form without evaluating the result."},
	"try":         {"[expr & clauses]", SpecialForm, "special-forms", "Evaluates expr, dispatching to a (catch …) and/or (finally …) clause on error."},
	"catch":       {"[binding & body]", SpecialForm, "special-forms", "Inside try: binds the caught error and evaluates body."},
	"finally":     {"[& body]", SpecialForm, "special-forms", "Inside try: body is always evaluated for side effects, error or not."},
	"context":     {"[& body]", SpecialForm, "special-forms", "Provides a Go context to the enclosed forms."},

	// --- Arithmetic ---
	"+": {"[& numbers]", Function, "arithmetic", "Sum of its arguments (0 with none)."},
	"-": {"[x & more]", Function, "arithmetic", "Subtracts the remaining arguments from x; negates x when alone."},
	"*": {"[& numbers]", Function, "arithmetic", "Product of its arguments (1 with none)."},
	"/": {"[x & more]", Function, "arithmetic", "Divides x by the remaining arguments."},

	// --- Comparison ---
	"=":    {"[a b]", Function, "comparison", "Value equality."},
	"not=": {"[a b]", Function, "comparison", "Logical negation of =."},
	"<":    {"[a b]", Function, "comparison", "Less-than."},
	"<=":   {"[a b]", Function, "comparison", "Less-than-or-equal."},
	">":    {"[a b]", Function, "comparison", "Greater-than."},
	">=":   {"[a b]", Function, "comparison", "Greater-than-or-equal."},

	// --- Collections ---
	"list":      {"[& items]", Function, "collections", "Creates a list of the given items."},
	"vector":    {"[& items]", Function, "collections", "Creates a vector of the given items."},
	"hash-map":  {"[& kvs]", Function, "collections", "Creates a hash-map from alternating key/value arguments."},
	"hash-set":  {"[& items]", Function, "collections", "Creates a set of the given items."},
	"cons":      {"[x seq]", Function, "collections", "Prepends x to seq."},
	"concat":    {"[& seqs]", Function, "collections", "Concatenates the sequences into one list."},
	"conj":      {"[coll & items]", Function, "collections", "Adds items to a collection (position depends on the collection type)."},
	"first":     {"[coll]", Function, "collections", "First element of coll, or nil."},
	"rest":      {"[coll]", Function, "collections", "All but the first element of coll, as a list."},
	"nth":       {"[coll n]", Function, "collections", "The element of coll at zero-based index n."},
	"count":     {"[coll]", Function, "collections", "Number of elements in coll."},
	"get":       {"[coll key]", Function, "collections", "Value at key in a map/vector, or nil."},
	"get-in":    {"[coll keys]", Function, "collections", "Nested value reached by following the vector of keys."},
	"assoc":     {"[map key val & kvs]", Function, "collections", "Copy of map with the given key/value pairs added or replaced."},
	"assoc-in":  {"[coll keys val]", Function, "collections", "Copy of coll with val set at the nested path keys."},
	"dissoc":    {"[map & keys]", Function, "collections", "Copy of map without the given keys."},
	"update":    {"[map key f & args]", Function, "collections", "Copy of map with key updated to (f old & args)."},
	"keys":      {"[map]", Function, "collections", "Vector of the map's keys."},
	"vals":      {"[map]", Function, "collections", "Vector of the map's values."},
	"contains?": {"[coll key]", Function, "collections", "Whether coll has the given key/index."},
	"merge":     {"[& maps]", Function, "collections", "Merges maps left to right; later keys win."},
	"map":       {"[f coll]", Function, "collections", "Applies f to each element of coll, returning a list."},
	"range":     {"[start end]", Function, "collections", "Vector of integers from start to end-1."},
	"take":      {"[n coll]", Function, "collections", "First n elements of coll."},
	"drop":      {"[n coll]", Function, "collections", "coll without its first n elements."},
	"empty?":    {"[coll]", Function, "collections", "Whether coll has no elements."},
	"seq":       {"[coll]", Function, "collections", "coll as a sequence, or nil when empty."},
	"vec":       {"[coll]", Function, "collections", "coll as a vector."},

	// --- Strings & symbols ---
	"str":         {"[& args]", Function, "strings", "Concatenates the printed representations of its arguments."},
	"pr-str":      {"[& args]", Function, "strings", "Like str but with readable (quoted) representations."},
	"split":       {"[string cutset]", Function, "strings", "Splits string on any character of cutset, returning a vector."},
	"read-string": {"[string]", Function, "strings", "Reads the first lisp form from string."},
	"keyword":     {"[name]", Function, "strings", "Creates a keyword from a string."},
	"symbol":      {"[name]", Function, "strings", "Creates a symbol from a string."},

	// --- I/O ---
	"println":  {"[& args]", Function, "io", "Prints its arguments (unquoted) separated by spaces, then a newline."},
	"prn":      {"[& args]", Function, "io", "Prints its arguments (readable) separated by spaces, then a newline."},
	"slurp":    {"[filename]", Function, "io", "Reads a file and returns its contents as a string."},
	"readline": {"[prompt]", Function, "io", "Prints prompt and reads a line from input."},
	"sleep":    {"[ms]", Function, "io", "Sleeps for ms milliseconds."},

	// --- Atoms ---
	"atom":   {"[value]", Function, "atoms", "Creates a mutable, thread-safe atom holding value."},
	"deref":  {"[atom]", Function, "atoms", "Current value of the atom (also written @atom)."},
	"reset!": {"[atom value]", Function, "atoms", "Sets the atom to value and returns it."},
	"swap!":  {"[atom f & args]", Function, "atoms", "Atomically sets the atom to (f current & args)."},

	// --- Predicates ---
	"nil?":        {"[x]", Function, "predicates", "Whether x is nil."},
	"true?":       {"[x]", Function, "predicates", "Whether x is boolean true."},
	"false?":      {"[x]", Function, "predicates", "Whether x is boolean false."},
	"number?":     {"[x]", Function, "predicates", "Whether x is a number."},
	"string?":     {"[x]", Function, "predicates", "Whether x is a string."},
	"symbol?":     {"[x]", Function, "predicates", "Whether x is a symbol."},
	"keyword?":    {"[x]", Function, "predicates", "Whether x is a keyword."},
	"list?":       {"[x]", Function, "predicates", "Whether x is a list."},
	"vector?":     {"[x]", Function, "predicates", "Whether x is a vector."},
	"map?":        {"[x]", Function, "predicates", "Whether x is a hash-map."},
	"set?":        {"[x]", Function, "predicates", "Whether x is a set."},
	"fn?":         {"[x]", Function, "predicates", "Whether x is callable."},
	"atom?":       {"[x]", Function, "predicates", "Whether x is an atom."},
	"sequential?": {"[x]", Function, "predicates", "Whether x is a list or vector."},

	// --- Evaluation & meta ---
	"eval":      {"[form]", Function, "evaluation", "Evaluates form in the global environment."},
	"apply":     {"[f & args]", Function, "evaluation", "Calls f with args, the last of which is a sequence spread as arguments."},
	"throw":     {"[value]", Function, "evaluation", "Raises value as an error."},
	"meta":      {"[obj]", Function, "meta", "Metadata attached to obj, or nil."},
	"with-meta": {"[obj m]", Function, "meta", "Copy of obj with metadata m."},
	"type?":     {"[x]", Function, "meta", "Type name of x as a string."},
	"uuid":      {"[]", Function, "meta", "A random RFC-4122 UUID string."},
}
