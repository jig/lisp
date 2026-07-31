package docgen_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jig/lisp/docgen"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/types"
)

const langDoc = "../LANGUAGE.md"

// TestLanguageDocUpToDate fails when the generated builtin reference in
// LANGUAGE.md has drifted from the interpreter — i.e. a builtin was
// added, removed, or its doc/arglist changed without regenerating the
// document. Regenerate with `go generate ./...`.
func TestLanguageDocUpToDate(t *testing.T) {
	current, err := os.ReadFile(langDoc)
	if err != nil {
		t.Fatalf("read %s: %v", langDoc, err)
	}
	want, err := docgen.Splice(string(current))
	if err != nil {
		t.Fatalf("splice: %v", err)
	}
	if want != string(current) {
		t.Errorf("%s is out of date; run `go generate ./...` to regenerate the builtin reference", langDoc)
	}
}

// TestMarkdownRenders is a smoke test: the reference must generate
// without error and be non-trivial.
func TestMarkdownRenders(t *testing.T) {
	md, err := docgen.Markdown()
	if err != nil {
		t.Fatal(err)
	}
	if len(md) < 1000 {
		t.Fatalf("generated reference suspiciously short (%d bytes)", len(md))
	}
}

// TestMarkdownForEmbedder checks the embedder path: an extra library is
// rendered as its own section, with the title and description given, and
// its symbols are attributed to it rather than to a standard library.
func TestMarkdownForEmbedder(t *testing.T) {
	load := func(ns types.EnvType) error {
		call.CallOverrideFN(ns, "embedder-probe", embedderProbe)
		call.Doc(ns, "embedder-probe", "[]", "Probe builtin used by the docgen test.")
		return nil
	}
	md, err := docgen.MarkdownFor([]docgen.Library{{
		Name:  "probe",
		Load:  load,
		Title: "probe",
		Desc:  "Namespace added by an embedder.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"### probe",
		"Namespace added by an embedder.",
		"embedder-probe",
		"Probe builtin used by the docgen test.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("generated reference is missing %q", want)
		}
	}

	// Without the extra library none of it appears.
	std, err := docgen.Markdown()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(std, "embedder-probe") {
		t.Error("the standard reference must not contain the embedder's symbols")
	}
}

func embedderProbe() (types.MalType, error) { return nil, nil }

// TestMarkdownForRejectsBadLibraries pins the fail-closed guards: a
// name colliding with a standard namespace (which would silently
// hijack its section) and a nil loader both error instead of rendering.
func TestMarkdownForRejectsBadLibraries(t *testing.T) {
	noop := func(ns types.EnvType) error { return nil }
	for _, tc := range []struct {
		name string
		lib  docgen.Library
		want string
	}{
		{"standard-name collision", docgen.Library{Name: "log", Load: noop}, "collides"},
		{"nil Load", docgen.Library{Name: "probe"}, "nil Load"},
	} {
		if _, err := docgen.MarkdownFor([]docgen.Library{tc.lib}); err == nil ||
			!strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: MarkdownFor = %v, want error containing %q", tc.name, err, tc.want)
		}
	}
	// Duplicates within the extra list collide too.
	dup := []docgen.Library{
		{Name: "probe", Load: noop},
		{Name: "probe", Load: noop},
	}
	if _, err := docgen.SymbolsFor(dup); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Errorf("duplicate extra names: SymbolsFor = %v, want a collision error", err)
	}
}

// TestSymbolsForIncludesEmbedder checks that SymbolsFor reports the
// embedder's symbols on top of the standard ones.
func TestSymbolsForIncludesEmbedder(t *testing.T) {
	std, err := docgen.StandardSymbols()
	if err != nil {
		t.Fatal(err)
	}
	all, err := docgen.SymbolsFor([]docgen.Library{{
		Name: "probe",
		Load: func(ns types.EnvType) error {
			call.CallOverrideFN(ns, "embedder-probe", embedderProbe)
			return nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(std)+1 {
		t.Fatalf("SymbolsFor returned %d symbols, want %d", len(all), len(std)+1)
	}
}
