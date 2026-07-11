package integrity_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatalf("load core: %v", err)
	}
	integrity.Load(ns)
	return ns
}

func run(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	res, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile("test"))
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	s, ok := res.(string)
	if !ok {
		t.Fatalf("REPL returned %T for %q, want printed string", res, src)
	}
	return s
}

// Boolean-valued expressions keep the expected outputs independent of
// how the printer renders strings.
func assertTrue(t *testing.T, ns types.EnvType, name, src string) {
	t.Helper()
	if got := run(t, ns, src); got != "true" {
		t.Errorf("%s: %s = %s, want true", name, src, got)
	}
}

func TestSha2256(t *testing.T) {
	ns := newEnv(t)
	// FIPS 180-2 test vector for "abc".
	assertTrue(t, ns, "known-vector",
		`(= (sha2-256 "abc") "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")`)
	assertTrue(t, ns, "empty-string",
		`(= (sha2-256 "") "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")`)
}

func TestFmt(t *testing.T) {
	ns := newEnv(t)
	assertTrue(t, ns, "canonical-input-fixpoint",
		`(= (fmt "(+ 1 2)\n") "(+ 1 2)\n")`)
	assertTrue(t, ns, "normalises-spacing",
		`(= (fmt "(+  1   2)") "(+ 1 2)\n")`)
	assertTrue(t, ns, "collapses-blank-lines",
		`(= (fmt "(+ 1 2)\n\n\n\n(+ 3 4)\n") "(+ 1 2)\n\n(+ 3 4)\n")`)
	assertTrue(t, ns, "unbalanced-source-errors",
		`(try (do (fmt "(((") false) (catch e true))`)
}

func TestEd25519(t *testing.T) {
	ns := newEnv(t)
	run(t, ns, `(def k (ed25519-generate))`)
	run(t, ns, `(def sig (ed25519-sign (get k :private) "hello"))`)

	assertTrue(t, ns, "verify-roundtrip",
		`(ed25519-verify (get k :public) "hello" sig)`)
	assertTrue(t, ns, "tampered-message-fails",
		`(= false (ed25519-verify (get k :public) "hellO" sig))`)
	assertTrue(t, ns, "deterministic-signature",
		`(= sig (ed25519-sign (get k :private) "hello"))`)
	assertTrue(t, ns, "other-key-fails",
		`(= false (ed25519-verify (get (ed25519-generate) :public) "hello" sig))`)

	assertTrue(t, ns, "bad-base64-private-errors",
		`(try (do (ed25519-sign "not base64!" "hello") false) (catch e true))`)
	assertTrue(t, ns, "short-public-key-errors",
		`(try (do (ed25519-verify "c2hvcnQ=" "hello" sig) false) (catch e true))`)
	assertTrue(t, ns, "bad-base64-signature-errors",
		`(try (do (ed25519-verify (get k :public) "hello" "not base64!") false) (catch e true))`)
}

// TestIntegrityDocs asserts every integrity builtin carries doc
// metadata with nil meta, as the other libraries' doc tests do.
func TestIntegrityDocs(t *testing.T) {
	ns := env.NewEnv()
	integrity.Load(ns)
	for _, name := range []string{"fmt", "sha2-256", "ed25519-generate", "ed25519-sign", "ed25519-verify"} {
		v, err := ns.Get(types.Symbol{Val: name})
		if err != nil {
			t.Errorf("%q not registered", name)
			continue
		}
		fn, ok := v.(types.Func)
		if !ok {
			t.Errorf("%q is %T, expected Func", name, v)
			continue
		}
		if fn.Doc == "" || fn.Arglist == "" {
			t.Errorf("%q missing doc fields", name)
		}
		if fn.Meta != nil {
			t.Errorf("%q meta should stay nil, got %v", name, fn.Meta)
		}
	}
}
