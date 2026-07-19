#!/usr/bin/env bash
# Replays every walkthrough of README.md in throwaway repositories,
# failure cases included. A living check that the examples work as
# documented. Usage: ./demo.sh  (or LISP=/path/to/lisp ./demo.sh)
set -euo pipefail

LISP=${LISP:-lisp}
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

step() { printf '\n\033[1m== %s ==\033[0m\n' "$*"; }
ok()   { printf '✓ %s\n' "$*"; }
must_fail() { # must_fail <description> -- <command...>
	local desc=$1; shift 2
	if "$@" >/dev/null 2>&1; then
		printf '✗ expected failure but succeeded: %s\n' "$desc" >&2; exit 1
	fi
	ok "refused as expected: $desc"
}

repo() { # repo <example-dir> → sets up $WORK/<example-dir> as a fresh tagged repo and cds into it
	local dir="$WORK/$1"
	mkdir -p "$dir"
	cp -r "$HERE/$1/." "$dir/"
	cd "$dir"
	git init -q
	git add -A
	git -c user.name=Demo -c user.email=demo@example.com commit -qm "release"
	git tag v1
}

step "01-basic: verify a script against a ref"
repo 01-basic
$LISP --integrity v1 service.lisp
$LISP --integrity "$(git rev-parse v1)" service.lisp >/dev/null
ok "a commit hash works like the tag"
must_fail "running without --integrity (assert-integrity throws)" -- $LISP service.lisp
echo ";; patched" >> service.lisp
must_fail "script modified after the release" -- $LISP --integrity v1 service.lisp

step "02-requires: the check cascades to required modules"
repo 02-requires
$LISP --integrity v1 service.lisp
echo "(def evil 1)" >> .lisp/util.lisp
must_fail "module modified after the release" -- $LISP --integrity v1 service.lisp

step "03-signed: trust a key, not the local repository"
repo 03-signed
git tag -d v1 >/dev/null
ssh-keygen -q -t ed25519 -f release-key -N "" -C "release@example.com"
git -c user.name=Demo -c user.email=demo@example.com \
    -c gpg.format=ssh -c user.signingkey=./release-key tag -s v1 -m "signed release"
$LISP --integrity v1 --integrity-keys release-key.pub service.lisp
ssh-keygen -q -t ed25519 -f other-key -N ""
must_fail "signed by a key not in the keys file" -- \
	$LISP --integrity v1 --integrity-keys other-key.pub service.lisp
# git allowed_signers format (principal-first) is refused, not misparsed.
printf 'release@example.com %s\n' "$(cat release-key.pub)" > allowed_signers
must_fail "allowed_signers format (principal-first) instead of .pub" -- \
	$LISP --integrity v1 --integrity-keys allowed_signers service.lisp
git tag -d v1 >/dev/null && git tag v1
must_fail "unsigned tag with --integrity-keys" -- \
	$LISP --integrity v1 --integrity-keys release-key.pub service.lisp

step "04-state: persistent state inside the integrity envelope"
repo 04-state
$LISP --integrity v1 service.lisp
$LISP --integrity v1 service.lisp
$LISP --integrity v1 service.lisp
[ "$(git log --oneline | grep -c 'state: db')" -eq 3 ] && ok "three state commits, same ref throughout"
echo "{:visits 999}" > .state/db.lisp
must_fail "state file edited out of band" -- $LISP --integrity v1 service.lisp
git checkout -- .state/db.lisp
$LISP --integrity v1 service.lisp >/dev/null
ok "operator restored the state; runs again"

step "all examples behaved as documented"
