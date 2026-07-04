package lsp

import "testing"

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
