// Package regexp adds regular expressions to jig/lisp, backed by Go's
// RE2 engine (linear-time matching; no backreferences or lookaround, so
// it differs from Java/Clojure's PCRE-style engine). Patterns are best
// written as raw ¬…¬ strings to avoid escaping — ¬\d+¬ rather than
// "\\d+" (a normal "…" string rejects \d) — the jig/lisp equivalent of
// Clojure's #"…" literal.
//
// re-pattern compiles a first-class, reusable regex; the other functions
// accept either a compiled regex or a raw pattern string (compiled on
// use). Following Clojure, re-matches / re-matches? are anchored (the
// whole string must match) while re-find / re-find? are unanchored
// (match anywhere).
package regexp

import (
	"fmt"
	"regexp"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

// Regexp is a compiled pattern — a first-class lisp value.
type Regexp struct {
	src      string
	re       *regexp.Regexp // as written (unanchored), for find
	anchored *regexp.Regexp // \A(?:src)\z, for matches
}

// LispPrint renders the regex as «regex src».
func (r *Regexp) LispPrint(_ func(MalType, bool) string) string {
	return "«regex " + r.src + "»"
}

func compile(fnName, src string) (*Regexp, error) {
	re, err := regexp.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fnName, err)
	}
	// \A…\z anchor the whole text regardless of any (?m) flag in src, so
	// re-matches means "the entire string", as in Clojure/Java.
	anchored, err := regexp.Compile(`\A(?:` + src + `)\z`)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fnName, err)
	}
	return &Regexp{src: src, re: re, anchored: anchored}, nil
}

// asRegexp accepts a compiled Regexp or a raw pattern string.
func asRegexp(fnName string, v MalType) (*Regexp, error) {
	switch t := v.(type) {
	case *Regexp:
		return t, nil
	case string:
		return compile(fnName, t)
	default:
		return nil, fmt.Errorf("%s: expected a regex or a pattern string, got %T", fnName, v)
	}
}

func Load(env EnvType) {
	call.CallOverrideFN(env, "re-pattern", rePattern)
	call.CallOverrideFN(env, "re-matches?", reMatchesQ)
	call.CallOverrideFN(env, "re-find?", reFindQ)
	call.CallOverrideFN(env, "re-matches", reMatches)
	call.CallOverrideFN(env, "re-find", reFind)

	call.Doc(env, "re-pattern", "[pattern]",
		"Compiles a raw pattern string (Go RE2 syntax; write it as ¬…¬) into a reusable regex value.")
	call.Doc(env, "re-matches?", "[re-or-pattern s]",
		"Reports whether the whole string s matches (anchored). Accepts a compiled regex or a raw pattern string.")
	call.Doc(env, "re-find?", "[re-or-pattern s]",
		"Reports whether the pattern matches anywhere in s (substring). Accepts a compiled regex or a raw pattern string.")
	call.Doc(env, "re-matches", "[re-or-pattern s]",
		"Anchored match of the whole string: nil, the match string when there are no groups, or a vector [whole g1 g2 …] (nil for an unmatched group).")
	call.Doc(env, "re-find", "[re-or-pattern s]",
		"First match anywhere in s: nil, the match string when there are no groups, or a vector [whole g1 g2 …] (nil for an unmatched group).")
}

func rePattern(pattern MalType) (MalType, error) {
	return asRegexp("re-pattern", pattern)
}

func reMatchesQ(reOrPattern MalType, s string) (MalType, error) {
	rx, err := asRegexp("re-matches?", reOrPattern)
	if err != nil {
		return nil, err
	}
	return rx.anchored.MatchString(s), nil
}

func reFindQ(reOrPattern MalType, s string) (MalType, error) {
	rx, err := asRegexp("re-find?", reOrPattern)
	if err != nil {
		return nil, err
	}
	return rx.re.MatchString(s), nil
}

func reMatches(reOrPattern MalType, s string) (MalType, error) {
	rx, err := asRegexp("re-matches", reOrPattern)
	if err != nil {
		return nil, err
	}
	return submatch(rx.anchored, s), nil
}

func reFind(reOrPattern MalType, s string) (MalType, error) {
	rx, err := asRegexp("re-find", reOrPattern)
	if err != nil {
		return nil, err
	}
	return submatch(rx.re, s), nil
}

// submatch mirrors Clojure's re-matches/re-find return: nil when there is
// no match; the whole match string when the pattern has no capture
// groups; else a vector [whole g1 g2 …] with an unmatched optional group
// rendered as nil (Go leaves it "", so index -1 is used to tell apart).
func submatch(re *regexp.Regexp, s string) MalType {
	idx := re.FindStringSubmatchIndex(s)
	if idx == nil {
		return nil
	}
	n := len(idx) / 2
	if n == 1 {
		return s[idx[0]:idx[1]]
	}
	out := make([]MalType, n)
	for i := range n {
		start, end := idx[2*i], idx[2*i+1]
		if start < 0 {
			out[i] = nil
		} else {
			out[i] = s[start:end]
		}
	}
	return Vector{Val: out}
}
