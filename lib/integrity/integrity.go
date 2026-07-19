// Package integrity provides builtins to attest and verify lisp source:
// canonical formatting (fmt), hashing (sha2-256) and Ed25519 signatures
// (ed25519-generate, ed25519-sign, ed25519-verify).
//
// Together they let a lisp program compute a stable digest of source
// text — hashing the canonical form so cosmetic layout differences do
// not change the digest — and sign or verify it. Ed25519 is used
// because signing is deterministic: the same key and message always
// produce the same signature bytes, so signatures are reproducible and
// diff-friendly. Keys and signatures are exchanged as standard base64
// strings; digests as lowercase hex.
//
// The package also implements the interpreter's integrity mode
// (`lisp --integrity <ref>`, see mode.go), which runs a script if and
// only if it — and, in cascade, its repo-local requires — match what is
// committed in its Git repository at <ref>; the (assert-integrity)
// builtin lets a script demand that mode.
package integrity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/jig/lisp/format"
	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

func Load(env EnvType) {
	// fmt clashes with the Go stdlib package name, so it cannot take
	// its lisp name from the Go function name as the others do.
	call.CallOverrideFN(env, "fmt", fmtSource)
	call.Call(env, sha2_256)
	call.Call(env, ed25519_generate)
	call.Call(env, ed25519_sign)
	call.Call(env, ed25519_verify)
	call.Call(env, assert_integrity)
	call.Call(env, state_save)
	call.Call(env, state_load, 1, 2)

	call.Doc(env, "fmt", "[s]",
		"Formats lisp source s into its canonical form (as lisp --fmt does); errors if s does not parse.")
	call.Doc(env, "sha2-256", "[s]",
		"SHA2-256 digest of string s, as lowercase hex.")
	call.Doc(env, "ed25519-generate", "[]",
		"Generates an Ed25519 key pair, as a map {:public :private} of base64 strings.")
	call.Doc(env, "ed25519-sign", "[private s]",
		"Signs string s with a base64 Ed25519 private key; returns the base64 signature (deterministic).")
	call.Doc(env, "ed25519-verify", "[public s signature]",
		"Reports whether the base64 signature of string s verifies against the base64 Ed25519 public key.")
	call.Doc(env, "assert-integrity", "[]",
		"Throws unless the interpreter runs under --integrity; returns the verified commit hash.")
	call.Doc(env, "state-save", "[name value]",
		"Writes value as canonical lisp data to .state/name.lisp at the repository root and commits it; returns the commit hash. Under --integrity the commit keeps the verified ref valid.")
	call.Doc(env, "state-load", "[name & [default]]",
		"Reads .state/name.lisp back as data (READ, never EVAL); returns default (or throws) when absent. Under --integrity the file must match its committed version at HEAD.")
}

func assert_integrity() (string, error) {
	if !Active() {
		return "", fmt.Errorf("assert-integrity: source integrity is not verified (run with --integrity <ref>)")
	}
	return CommitHash(), nil
}

func fmtSource(s string) (string, error) {
	out, err := format.Source([]byte(s))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sha2_256(s string) (string, error) {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:]), nil
}

func ed25519_generate() (MalType, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return HashMap{Val: map[string]MalType{
		NewKeyword("public"):  base64.StdEncoding.EncodeToString(pub),
		NewKeyword("private"): base64.StdEncoding.EncodeToString(priv),
	}}, nil
}

func ed25519_sign(private, s string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(private)
	if err != nil {
		return "", fmt.Errorf("ed25519-sign: private key is not valid base64: %w", err)
	}
	if len(key) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("ed25519-sign: private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(key))
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), []byte(s))), nil
}

func ed25519_verify(public, s, signature string) (bool, error) {
	key, err := base64.StdEncoding.DecodeString(public)
	if err != nil {
		return false, fmt.Errorf("ed25519-verify: public key is not valid base64: %w", err)
	}
	if len(key) != ed25519.PublicKeySize {
		return false, fmt.Errorf("ed25519-verify: public key must be %d bytes, got %d", ed25519.PublicKeySize, len(key))
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("ed25519-verify: signature is not valid base64: %w", err)
	}
	return ed25519.Verify(ed25519.PublicKey(key), []byte(s), sig), nil
}
