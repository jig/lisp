package core

import (
	"github.com/jig/lisp/lib/call"

	. "github.com/jig/lisp/types"
)

// loadDocs attaches Clojure-style {:arglists … :doc …} metadata to the
// core Go builtins registered by Load, so the LSP, (doc …) and
// documentation generators describe them from the live environment. Go
// reflection cannot recover parameter names, so the arglists are
// written here next to the builtins. TestCoreDocsAreHonest asserts
// every entry names a real builtin.
func loadDocs(env EnvType) {
	for _, d := range coreDocs {
		call.Doc(env, d.name, d.arglist, d.doc)
	}
}

// loadInputDocs documents the builtins registered by LoadInput (they
// touch stdin/files, so they live in a separate namespace load).
func loadInputDocs(env EnvType) {
	for _, d := range inputDocs {
		call.Doc(env, d.name, d.arglist, d.doc)
	}
}

var inputDocs = []struct{ name, arglist, doc string }{
	{"slurp", "[filename]", "Reads a file and returns its contents as a string."},
	{"spit", "[filename s & opts]", "Writes string s to a file, creating or truncating it; with :append true, appends instead."},
	{"readline", "[prompt]", "Prints prompt and reads a line from input."},
	{"read-password", "[prompt]", "Prints prompt (to stderr) and reads a line with terminal echo disabled; falls back to a plain read when input is not a terminal. Returns nil on end of input."},
	{"exit", "([] [status])", "Terminates the process with status (an integer, 0 when omitted). Does not return."},
}

var coreDocs = []struct{ name, arglist, doc string }{
	// Arithmetic
	{"+", "[& numbers]", "Sum of its arguments (0 with none)."},
	{"-", "[x & more]", "Subtracts the remaining arguments from x; negates x when alone."},
	{"*", "[& numbers]", "Product of its arguments (1 with none)."},
	{"/", "[x & more]", "Divides x by the remaining arguments; (/ x) is the inverse 1/x."},

	// Comparison
	{"=", "[a b]", "Value equality."},
	{"not=", "[a b]", "Logical negation of =."},
	{"<", "[x & more]", "True when the arguments are monotonically increasing."},
	{"<=", "[x & more]", "True when the arguments are monotonically non-decreasing."},
	{">", "[x & more]", "True when the arguments are monotonically decreasing."},
	{">=", "[x & more]", "True when the arguments are monotonically non-increasing."},

	// Collections
	{"list", "[& items]", "Creates a list of the given items."},
	{"vector", "[& items]", "Creates a vector of the given items."},
	{"hash-map", "[& kvs]", "Creates a hash-map from alternating key/value arguments."},
	{"hash-set", "[& items]", "Creates a set of the given items."},
	{"cons", "[x seq]", "Prepends x to seq."},
	{"concat", "[& seqs]", "Concatenates the sequences into one list."},
	{"conj", "[coll & items]", "Adds items to a collection (position depends on the collection type)."},
	{"first", "[coll]", "First element of coll, or nil."},
	{"rest", "[coll]", "All but the first element of coll, as a list."},
	{"nth", "[coll n]", "The element of coll at zero-based index n."},
	{"count", "[coll]", "Number of elements in coll."},
	{"get", "[coll key]", "Value at key in a map/vector, or nil."},
	{"get-in", "[coll keys]", "Nested value reached by following the vector of keys."},
	{"assoc", "[map key val & kvs]", "Copy of map with the given key/value pairs added or replaced."},
	{"assoc-in", "[coll keys val]", "Copy of coll with val set at the nested path keys."},
	{"dissoc", "[map & keys]", "Copy of map without the given keys."},
	{"update", "[map key f & args]", "Copy of map with key updated to (f old & args)."},
	{"keys", "[map]", "Vector of the map's keys."},
	{"vals", "[map]", "Vector of the map's values."},
	{"contains?", "[coll key]", "Whether coll has the given key/index."},
	{"merge", "[& maps]", "Merges maps left to right; later keys win."},
	{"map", "[f coll]", "Applies f to each element of coll, returning a list."},
	{"range", "[start end]", "Vector of integers from start to end-1."},
	{"take", "[n coll]", "First n elements of coll."},
	{"drop", "[n coll]", "coll without its first n elements."},
	{"empty?", "[coll]", "Whether coll has no elements."},
	{"seq", "[coll]", "coll as a sequence, or nil when empty."},
	{"vec", "[coll]", "coll as a vector."},

	// Strings & symbols
	{"str", "[& args]", "Concatenates the printed representations of its arguments."},
	{"pr-str", "[& args]", "Like str but with readable (quoted) representations."},
	{"split", "[string cutset]", "Splits string on any character of cutset, returning a vector."},
	{"subs", "[s start end]", "Substring of s from start to end (end optional), counted in Unicode code points."},
	{"starts-with?", "[s prefix]", "Whether string s starts with prefix."},
	{"ends-with?", "[s suffix]", "Whether string s ends with suffix."},
	{"read-string", "[string]", "Reads the first lisp form from string."},
	{"keyword", "[name]", "Creates a keyword from a string."},
	{"symbol", "[name]", "Creates a symbol from a string."},

	// I/O
	{"print", "[& args]", "Prints its arguments (unquoted) separated by spaces, without a trailing newline."},
	{"println", "[& args]", "Prints its arguments (unquoted) separated by spaces, then a newline."},
	{"prn", "[& args]", "Prints its arguments (readable) separated by spaces, then a newline."},
	{"sleep", "[ms]", "Sleeps for ms milliseconds."},
	{"gensym", "[]", "Returns a fresh, hopefully-unique symbol like G__N (for writing hygienic macros)."},

	// Predicates
	{"nil?", "[x]", "Whether x is nil."},
	{"true?", "[x]", "Whether x is boolean true."},
	{"false?", "[x]", "Whether x is boolean false."},
	{"number?", "[x]", "Whether x is a number."},
	{"string?", "[x]", "Whether x is a string."},
	{"symbol?", "[x]", "Whether x is a symbol."},
	{"keyword?", "[x]", "Whether x is a keyword."},
	{"list?", "[x]", "Whether x is a list."},
	{"vector?", "[x]", "Whether x is a vector."},
	{"map?", "[x]", "Whether x is a hash-map."},
	{"set?", "[x]", "Whether x is a set."},
	{"fn?", "[x]", "Whether x is callable."},
	{"sequential?", "[x]", "Whether x is a list or vector."},

	// Evaluation & meta
	{"apply", "[f & args]", "Calls f with args, the last of which is a sequence spread as arguments."},
	{"throw", "[value]", "Raises value as an error."},
	{"meta", "[obj]", "Metadata attached to obj, or nil."},
	{"with-meta", "[obj m]", "Copy of obj with metadata m."},
	{"type?", "[x]", "Type name of x as a string."},
	{"uuid", "[]", "A random RFC-4122 UUID string."},
	{"deref", "[ref]", "Current value of an atom or other dereferenceable (also @ref)."},
	{"doc", "[f]", "Documentation string of a function/macro, or nil."},
	{"macro?", "[x]", "Whether x is a macro."},
	{"assert", "[expr & error]", "Returns nil when expr is truthy, otherwise raises error (or a default)."},

	// More collections
	{"take-last", "[n coll]", "Last n elements of coll."},
	{"drop-last", "[n coll]", "coll without its last n elements."},
	{"subvec", "[vec start end]", "Sub-vector of vec from start to end (end optional)."},
	{"update-in", "[coll keys f]", "Copy of coll with the nested value at keys replaced by (f old)."},
	{"rename-keys", "[map keymap]", "Copy of map with keys renamed according to keymap."},
	{"set", "[coll]", "Creates a set from the elements of coll."},

	// Time & version
	{"read-program", "[src module]", "Reads every form in src as one (do …) AST with positions attributed to module; load-file builds on it."},
	{"time-ms", "[]", "Current time in milliseconds since the epoch."},
	{"time-ns", "[]", "Current time in nanoseconds since the epoch."},
	{"time-format", "[ms]", "Formats epoch milliseconds (as of time-ms) as an RFC 3339 UTC timestamp with millisecond precision."},
	{"time-parse", "[string]", "Parses an RFC 3339 timestamp and returns epoch milliseconds (as of time-ms)."},
	{"version", "[]", "Interpreter build information as a hash-map."},

	// Bytes, base64 & JSON
	{"base64", "[bytes]", "Encodes a byte string to a base64 string."},
	{"unbase64", "[string]", "Decodes a base64 string to a byte string."},
	{"str2binary", "[string]", "Converts a string to a byte string."},
	{"binary2str", "[bytes]", "Converts a byte string to a string."},
	{"json-encode", "[obj]", "Encodes a lisp value (or Go object) to a JSON string."},
	{"json-decode", "[factory json]", "Decodes a JSON string into a lisp value."},
	{"hash-map-decode", "[factory json]", "Decodes JSON into a Go-backed hash-map."},

	// Errors & debugging
	{"error-string", "[err]", "The message of an error as a string."},
	{"new-error", "[value & [cursor]]", "Creates a lisp error wrapping value, optionally at a source position."},
	{"go-error", "[format & args]", "Creates a Go error from a format string and arguments."},
	{"new-go-error", "[message]", "Creates a Go error with the given message."},
	{"unwrap-error", "[err]", "The error wrapped inside err (Go's errors.Unwrap)."},
	{"panic", "[value]", "Raises value as a Go panic."},
	{"spew", "[x]", "Dumps x to stderr in Go syntax for debugging; returns nil."},
}
