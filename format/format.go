// Package format implements a gofmt-style pretty-printer for jig/lisp
// source code.
//
// It preserves the author's line breaks and comments and only normalises
// layout: two-space indentation for the body of ( ) lists, alignment to
// the first element for [ ] { } #{ } collections, a single space between
// tokens on the same line, runs of blank lines collapsed to one, trailing
// close delimiters pulled up against the last element, and exactly one
// trailing newline.
//
// Unlike a formatter built on the evaluator's reader, this one works on a
// concrete token tree so comments survive and the source is never
// re-parsed as an AST (which would drop them).
package format

import "unicode/utf8"

// Source formats jig/lisp source code. It returns an error, and no output,
// when the input does not parse (unbalanced delimiters or an unterminated
// string); callers should leave such input untouched.
func Source(src []byte) ([]byte, error) {
	toks, err := lex(string(src))
	if err != nil {
		return nil, err
	}
	root, err := parse(toks)
	if err != nil {
		return nil, err
	}
	var p printer
	p.printChildList(root, 0, 0)
	return []byte(p.result()), nil
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }
