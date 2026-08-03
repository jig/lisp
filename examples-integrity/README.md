# Integrity mode examples

Mini-examples for the concepts in [INTEGRITY.md](../INTEGRITY.md), one
directory per concept. Each is a complete program plus the shell
commands that take it from "files on disk" to "verified run".

**The examples must run in their own repository.** `lisp-integrity`
verifies against the Git repository *enclosing the script* — run in
place, that would be jig/lisp itself. Copy an example into a fresh
directory and follow its steps, or run the whole tour non-interactively:

```bash
./demo.sh                 # uses `lisp` and `lisp-integrity` from PATH
LISP=/path/to/lisp LISP_INTEGRITY=/path/to/lisp-integrity ./demo.sh
```

`demo.sh` replays every walkthrough below in throwaway repositories,
including the failure cases, and is kept runnable as a living check of
this documentation. Exception: 03-signed is manual — its trust anchor
is `/etc/lisp/allowed_signers`, and a demo must never touch a real
host's trust anchor.

The interactive runs below ask `proceed? [y/N]` after the green block;
`demo.sh` passes `-y`. On Linux, `lisp-integrity` requires the
systemd journal (every run is attested there); read a run back with
`journalctl -t service -o json`.

## 01-basic — verify a script against HEAD

`assert-integrity` makes the program refuse to run unverified.

```bash
cp 01-basic/service.lisp /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release" && git tag v1

lisp-integrity service.lisp          # ✓ runs, prints the commit hash
lisp service.lisp                    # ✗ assert-integrity throws
echo ";; patched" >> service.lisp
lisp-integrity service.lisp          # ✗ differs from its committed version
```

Pin a release by checking it out: `git checkout --detach v1` — HEAD
stays there across restarts, and `git pull` cannot move it.

## 02-requires — the check cascades to required modules

`require` resolves `util` to `.lisp/util.lisp` in the same repository;
the module must match its committed blob too, and modules resolving
*outside* the repository are refused.

```bash
cp -r 02-requires/. /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release"

lisp-integrity service.lisp          # ✓ script and module verified
echo "(def evil 1)" >> .lisp/util.lisp
lisp-integrity service.lisp          # ✗ util.lisp differs
```

The same cascade covers `load-file`/`load-file-once` targets.

## 03-signed — trust a key, not the local repository (manual)

When `/etc/lisp/allowed_signers` exists, every `lisp-integrity` run on
the host requires the HEAD chain to be SSH-signed by a listed key —
the guarantee upgrades from "matches this repository" to "matches what
a trusted key released", and survives cloning the repository
elsewhere. **Do this on a disposable host**: installing the file
affects every `lisp-integrity` run on it.

```bash
cp 03-signed/service.lisp /tmp/demo && cd /tmp/demo
git init && git add -A && git commit -m "release"

ssh-keygen -t ed25519 -f release-key -N "" -C "release@example.com"
git -c gpg.format=ssh -c user.signingkey=./release-key tag -s v1 -m "signed release"

# The trust anchor is authorized_keys / .pub format — the .pub file as
# is, NOT git's allowed_signers (a principal-first line is rejected).
# Root-owned, outside any repository:
sudo mkdir -p /etc/lisp
sudo cp release-key.pub /etc/lisp/allowed_signers

lisp-integrity service.lisp          # ✓ signer reported in the green block

git tag -d v1 && git tag v1          # re-tag, unsigned
lisp-integrity service.lisp          # ✗ commit is not signed
sudo rm /etc/lisp/allowed_signers    # clean up the host!
```

`(assert-integrity :with-signature)` in the script makes it refuse to
run on hosts *without* the file, closing the "quietly unsigned" gap.
