// Package docgen renders the builtin-reference section of LANGUAGE.md
// from the live interpreter environment, so the documentation describes
// exactly the builtins the binary loads (their names, argument lists and
// doc strings) and cannot drift from the code.
//
// The reference is produced by loading every namespace from
// github.com/jig/lisp/libraries in order and attributing each newly
// defined symbol to the library that introduced it. Go builtins carry
// their arglist/doc in the types.Func value (attached with call.Doc);
// lisp-defined functions and macros carry a docstring in their :doc
// metadata and their parameter vector in the value itself.
package docgen

//go:generate go run ./gen.go -o ../LANGUAGE.md

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jig/lisp/docmeta"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

// specialFormOrder is the reading order for the special-forms table;
// any name in docmeta.SpecialForms not listed here is appended, sorted.
var specialFormOrder = []string{
	"def", "fn", "defmacro", "let", "do", "if",
	"quote", "quasiquote", "macroexpand",
	"try", "catch", "finally",
	"loop", "recur", "context",
}

// Markers delimit the generated block inside LANGUAGE.md. Everything
// between them is owned by the generator; everything outside is written
// by hand.
const (
	BeginMarker = "<!-- BEGIN GENERATED BUILTINS -->"
	EndMarker   = "<!-- END GENERATED BUILTINS -->"
)

// sectionBlurb maps a library name (from standardLibraries) to the heading
// and one-line description used in the reference. A library with no
// entry here falls back to its raw name and no description.
var sectionBlurb = map[string]struct{ title, desc string }{
	"core mal":            {"core", "Arithmetic, collections, predicates, strings, JSON, errors — always loaded."},
	"core mal with input": {"core — input/output", "Reading and writing files and stdin."},
	"command line args":   {"core — runtime variables", "Values the runtime binds for a running script."},
	"concurrent":          {"concurrent", "Atoms and futures for shared, thread-safe state."},
	"core mal extended":   {"coreextended", "Higher-order helpers written in lisp (the prelude): reduce, map, partial, protocols…"},
	"assert":              {"assert", "Minimal test library (run with `lisp --test DIR`)."},
	"system":              {"system", "Access to the host environment (env vars, …)."},
	"lazy":                {"lazy", "Lazy sequences."},
	"require":             {"require", "Module loading by name through a search path."},
	"sql":                 {"sql", "SQL database access."},
	"cli":                 {"cli", "Command-line option parsing (clojure/tools.cli style)."},
	"integrity":           {"integrity", "Attest and verify lisp source (formatting, hashing, Ed25519 signatures, --integrity mode)."},
}

// entry is one documented symbol.
type entry struct {
	name    string
	arglist string
	doc     string
	kind    string // "fn", "macro", "builtin" or "var"
}

// section groups the entries a single library introduces.
type section struct {
	name    string
	title   string
	desc    string
	entries []entry
}

// collect loads every library in order and returns one section per
// library, each holding the symbols that library added to the
// environment.
func collect() ([]section, error) {
	ns := env.NewEnv()
	seen := map[string]bool{}
	var sections []section

	for _, lib := range standardLibraries() {
		if err := lib.load(ns); err != nil {
			return nil, fmt.Errorf("load %q: %w", lib.name, err)
		}

		var added []entry
		for _, name := range ns.(*env.Env).LocalSymbols() {
			if seen[name] || isPrivate(name) {
				continue
			}
			seen[name] = true
			v, err := ns.Get(types.Symbol{Val: name})
			if err != nil {
				continue
			}
			if e, ok := describe(name, v); ok {
				added = append(added, e)
			}
		}
		if len(added) == 0 {
			continue
		}
		sort.Slice(added, func(i, j int) bool { return added[i].name < added[j].name })

		blurb := sectionBlurb[lib.name]
		title := blurb.title
		if title == "" {
			title = lib.name
		}
		sections = append(sections, section{
			name:    lib.name,
			title:   title,
			desc:    blurb.desc,
			entries: added,
		})
	}
	return sections, nil
}

// isPrivate reports whether a symbol is an internal helper that should
// not appear in the reference: a leading underscore, or a namespaced
// name whose local part starts with "-" (Clojure's private convention,
// e.g. cli/-coerce). Top-level names starting with "-" such as -> are
// kept.
func isPrivate(name string) bool {
	if strings.HasPrefix(name, "_") {
		return true
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return strings.HasPrefix(name[i+1:], "-")
	}
	return false
}

// describe turns an environment value into a documentation entry. It
// reports ok=false for values that are not worth documenting (plain data
// left in the environment).
func describe(name string, v types.MalType) (entry, bool) {
	switch f := v.(type) {
	case types.Func:
		return entry{name: name, arglist: f.Arglist, doc: f.Doc, kind: "builtin"}, true
	case types.MalFunc:
		kind := "fn"
		if f.IsMacro {
			kind = "macro"
		}
		return entry{
			name:    name,
			arglist: printer.Pr_str(f.Params, true),
			doc:     malFuncDoc(f),
			kind:    kind,
		}, true
	case *types.MalFunc:
		return describe(name, *f)
	default:
		// Runtime variables such as *ARGV*, *host-language*, *FILE*.
		if strings.HasPrefix(name, "*") && strings.HasSuffix(name, "*") {
			return entry{name: name, kind: "var"}, true
		}
		return entry{}, false
	}
}

// malFuncDoc extracts the :doc metadata string attached by defn's
// docstring support, or "" when absent.
func malFuncDoc(f types.MalFunc) string {
	hm, ok := f.Meta.(types.HashMap)
	if !ok {
		return ""
	}
	if d, ok := hm.Val[types.NewKeyword("doc")].(string); ok {
		return d
	}
	return ""
}

// Markdown renders the full generated reference block (without the
// surrounding markers).
func Markdown() (string, error) {
	sections, err := collect()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("_This section is generated from the interpreter's own documentation")
	b.WriteString(" metadata; do not edit it by hand — run `go generate ./...`._\n")

	writeSpecialForms(&b)

	for _, s := range sections {
		fmt.Fprintf(&b, "\n### %s\n\n", s.title)
		if s.desc != "" {
			fmt.Fprintf(&b, "%s\n\n", s.desc)
		}

		vars := filterKind(s.entries, "var")
		callables := excludeKind(s.entries, "var")

		if len(callables) > 0 {
			b.WriteString("| Name | Arguments | Description |\n")
			b.WriteString("| ---- | --------- | ----------- |\n")
			for _, e := range callables {
				name := e.name
				if e.kind == "macro" {
					name += " ⁽ᵐ⁾"
				}
				fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n",
					name, mdCell(e.arglist), mdCell(defaultDoc(e.doc)))
			}
		}
		if len(vars) > 0 {
			names := make([]string, len(vars))
			for i, e := range vars {
				names[i] = "`" + e.name + "`"
			}
			fmt.Fprintf(&b, "\nRuntime variables: %s\n", strings.Join(names, ", "))
		}
	}
	b.WriteString("\n⁽ᵐ⁾ = macro (arguments are not evaluated before the call).\n")
	return b.String(), nil
}

// writeSpecialForms renders the curated special-forms table from
// docmeta. Special forms are handled directly by EVAL and never bound in
// an environment, so they cannot be discovered like builtins.
func writeSpecialForms(b *strings.Builder) {
	order := append([]string(nil), specialFormOrder...)
	listed := map[string]bool{}
	for _, n := range order {
		listed[n] = true
	}
	var extra []string
	for name := range docmeta.SpecialForms {
		if !listed[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	order = append(order, extra...)

	b.WriteString("\n### special forms\n\n")
	b.WriteString("Handled directly by the evaluator (they control when their arguments are evaluated), so they are not ordinary functions.\n\n")
	b.WriteString("| Name | Arguments | Description |\n")
	b.WriteString("| ---- | --------- | ----------- |\n")
	for _, name := range order {
		e, ok := docmeta.SpecialForms[name]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", name, mdCell(e.Params), mdCell(e.Doc))
	}
}

func defaultDoc(doc string) string {
	if strings.TrimSpace(doc) == "" {
		return "—"
	}
	return doc
}

// Splice replaces the text between BeginMarker and EndMarker in doc with
// a freshly generated reference block, leaving the hand-written parts
// untouched. It errors if the markers are missing or out of order.
func Splice(doc string) (string, error) {
	begin := strings.Index(doc, BeginMarker)
	end := strings.Index(doc, EndMarker)
	if begin == -1 || end == -1 {
		return "", fmt.Errorf("docgen: markers %q / %q not found in document", BeginMarker, EndMarker)
	}
	if end < begin {
		return "", fmt.Errorf("docgen: %q appears before %q", EndMarker, BeginMarker)
	}
	block, err := Markdown()
	if err != nil {
		return "", err
	}
	return doc[:begin+len(BeginMarker)] + "\n" + block + "\n" + doc[end:], nil
}

// UpdateFile rewrites path in place, regenerating the block between the
// markers. It is the entry point of the go:generate runner and reports
// whether the file actually changed.
func UpdateFile(path string) (changed bool, err error) {
	current, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated, err := Splice(string(current))
	if err != nil {
		return false, err
	}
	if updated == string(current) {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

// mdCell makes a string safe inside a Markdown table cell.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

func filterKind(es []entry, kind string) []entry {
	var out []entry
	for _, e := range es {
		if e.kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func excludeKind(es []entry, kind string) []entry {
	var out []entry
	for _, e := range es {
		if e.kind != kind {
			out = append(out, e)
		}
	}
	return out
}
