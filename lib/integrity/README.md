# integrity

Builtins to attest and verify lisp source text: canonical formatting,
hashing and digital signatures. Typical use: compute a stable digest of
a `.lisp` file (formatting first, so cosmetic layout differences do not
change the digest) and sign or verify it.

## Functions

| Function | Returns |
|---|---|
| `(fmt s)` | source `s` in canonical form (as `lisp --fmt`); errors if `s` does not parse |
| `(sha2-256 s)` | SHA2-256 digest of `s`, lowercase hex |
| `(ed25519-generate)` | key pair `{:public "…" :private "…"}`, base64 |
| `(ed25519-sign private s)` | base64 signature of `s` |
| `(ed25519-verify public s signature)` | `true` or `false` |

Ed25519 signing is deterministic: the same key and message always yield
the same signature bytes, so signatures are reproducible and
diff-friendly.

## Example

```clojure
(def keys (ed25519-generate))

(def source (fmt (slurp "program.lisp")))
(def digest (sha2-256 source))

(def signature (ed25519-sign (get keys :private) digest))
(ed25519-verify (get keys :public) digest signature) ;; => true
```

## Loading

Go embedders load the namespace with:

```go
import "github.com/jig/lisp/lib/integrity/nsintegrity"

nsintegrity.Load(env)
```

The library has no dependencies beyond `core` (the example above uses
`slurp` and `get` from core). The `lisp` binary loads it by default.
