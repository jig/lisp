# Integrity mode examples

Mini-examples for the concepts in [INTEGRITY.md](../INTEGRITY.md), one
directory per concept. Each is a complete program plus the shell
commands that take it from "files on disk" to "verified run".

**The examples must run in their own repository.** `--integrity`
verifies against the Git repository *enclosing the script* — run in
place, that would be jig/lisp itself. Copy an example into a fresh
directory and follow its steps, or run the whole tour non-interactively:

```bash
./demo.sh                 # uses `lisp` from PATH
LISP=/path/to/lisp ./demo.sh
```

`demo.sh` replays every walkthrough below in throwaway repositories,
including the failure cases, and is kept runnable as a living check of
this documentation.

## 01-basic — verify a script against a ref

`assert-integrity` makes the program refuse to run unverified.

```bash
cp 01-basic/service.lisp /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release" && git tag v1

lisp --integrity v1 service.lisp     # ✓ runs, prints the commit hash
lisp service.lisp                    # ✗ assert-integrity throws
echo ";; patched" >> service.lisp
lisp --integrity v1 service.lisp     # ✗ differs from its committed version
```

A commit hash works exactly like the tag: `lisp --integrity $(git
rev-parse v1) service.lisp`.

## 02-requires — the check cascades to required modules

`require` resolves `util` to `.lisp/util.lisp` in the same repository;
under `--integrity` the module must match its committed blob too, and
modules resolving *outside* the repository are refused.

```bash
cp -r 02-requires/. /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release" && git tag v1

lisp --integrity v1 service.lisp     # ✓ script and module verified
echo "(def evil 1)" >> .lisp/util.lisp
lisp --integrity v1 service.lisp     # ✗ util.lisp differs
```

The same cascade covers `load-file`/`load-file-once` targets.

## 03-signed — trust a key, not the local repository

An SSH-signed tag plus `--integrity-signers` upgrades the guarantee
from "matches this repository" to "matches what a trusted key
released" — it survives cloning the repository elsewhere.

```bash
cp 03-signed/service.lisp /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release"

ssh-keygen -t ed25519 -f release-key -N "" -C "release@example.com"
git -c gpg.format=ssh -c user.signingkey=./release-key tag -s v1 -m "signed release"

# The signers file is authorized_keys format — the .pub file as is.
# In production it lives OUTSIDE the repository (e.g. /etc/lisp/).
lisp --integrity v1 --integrity-signers release-key.pub service.lisp   # ✓

ssh-keygen -t ed25519 -f other-key -N ""
lisp --integrity v1 --integrity-signers other-key.pub service.lisp     # ✗ no allowed key matches
git tag -d v1 && git tag v1                                            # re-tag, unsigned
lisp --integrity v1 --integrity-signers release-key.pub service.lisp   # ✗ tag is not signed
```

## 04-state — persistent state inside the integrity envelope

`state-save` writes `.state/db.lisp` as canonical lisp data and
commits it in the same operation; `state-load` reads it back as pure
data and requires it to match `HEAD`. The state commits are accepted
by the startup check, so **every restart uses the same ref**:

```bash
cp 04-state/service.lisp /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release" && git tag v1

lisp --integrity v1 service.lisp     # visit number 1
lisp --integrity v1 service.lisp     # visit number 2  (same ref!)
lisp --integrity v1 service.lisp     # visit number 3
git log --oneline                    # release + three "state: db" commits

echo "{:visits 999}" > .state/db.lisp
lisp --integrity v1 service.lisp     # ✗ differs from its committed version at HEAD
git checkout .state/db.lisp          # operator resolves; runs again
```

A commit touching anything outside `.state/` after the ref invalidates
the run — code changes always require a new release ref.
