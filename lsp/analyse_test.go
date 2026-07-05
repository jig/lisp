package lsp

import (
	"testing"

	"github.com/jig/lisp/docmeta"
)

// TestAnalyseIncompleteForms guards against panics while analysing the
// half-typed forms that occur constantly during editing. The reader can
// return short lists like `(fn)` or `(let)`, and scan must never slice
// past their length.
func TestAnalyseIncompleteForms(t *testing.T) {
	cases := []string{
		"(fn)", "(let)", "(def)", "(defn)", "(defmacro)", "(catch)",
		"(require)", "(do)", "(if)", "(fn [",
		"(defn foo)", "(let [a])", "(let [a 1] (fn))",
		"(defn f [x] (let (catch)))",
		"(", "()", "(())",
		"(map (fn) (let))",
	}
	for _, src := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic analysing %q: %v", src, r)
				}
			}()
			analyseDocument("file:///t.lisp", src+"\n")
		}()
	}
}

// TestDocumentedSpecialFormsAreRecognised asserts every special form in the
// documentation table is also recognised by the analyser, so a documented
// form is never flagged as an unknown symbol. Guards against the two lists
// drifting apart.
func TestDocumentedSpecialFormsAreRecognised(t *testing.T) {
	for name := range docmeta.SpecialForms {
		if !specialForms[name] {
			t.Errorf("%q is documented but not in the analyser's specialForms set", name)
		}
	}
}
