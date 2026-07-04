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
	{"readline", "[prompt]", "Prints prompt and reads a line from input."},
}

var coreDocs = []struct{ name, arglist, doc string }{
	// Arithmetic
	{"+", "[& numbers]", "Sum of its arguments (0 with none)."},
	{"-", "[x & more]", "Subtracts the remaining arguments from x; negates x when alone."},
	{"*", "[& numbers]", "Product of its arguments (1 with none)."},
	{"/", "[x & more]", "Divides x by the remaining arguments."},

	// Comparison
	{"=", "[a b]", "Value equality."},
	{"not=", "[a b]", "Logical negation of =."},
	{"<", "[a b]", "Less-than."},
	{"<=", "[a b]", "Less-than-or-equal."},
	{">", "[a b]", "Greater-than."},
	{">=", "[a b]", "Greater-than-or-equal."},

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
	{"read-string", "[string]", "Reads the first lisp form from string."},
	{"keyword", "[name]", "Creates a keyword from a string."},
	{"symbol", "[name]", "Creates a symbol from a string."},

	// I/O
	{"println", "[& args]", "Prints its arguments (unquoted) separated by spaces, then a newline."},
	{"prn", "[& args]", "Prints its arguments (readable) separated by spaces, then a newline."},
	{"sleep", "[ms]", "Sleeps for ms milliseconds."},

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
}
