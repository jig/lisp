#!/usr/bin/env bash
# Replays every walkthrough of README.md in throwaway repositories,
# failure cases included. A living check that the examples work as
# documented. Usage: ./demo.sh
#   (or LISP=/path/to/lisp LISP_INTEGRITY=/path/to/lisp-integrity ./demo.sh)
#
# 03-signed is NOT replayed: the signature rule activates through
# /etc/lisp/allowed_signers, and a demo must never touch a real host's
# trust anchor. Follow 03-signed/README.md manually on a disposable
# host; the rule itself is covered by the Go tests.
set -euo pipefail

LISP=${LISP:-lisp}
LISP_INTEGRITY=${LISP_INTEGRITY:-lisp-integrity}
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
	# Hermetic: ignore the runner's global commit.gpgsign / tag.gpgsign so
	# the plain commit and lightweight tag below never demand a signature.
	git config commit.gpgsign false
	git config tag.gpgsign false
	git add -A
	git -c user.name=Demo -c user.email=demo@example.com commit -qm "release"
	git tag v1
}

step "01-basic: verify a script against HEAD"
repo 01-basic
$LISP_INTEGRITY -y service.lisp
git checkout -q --detach v1
$LISP_INTEGRITY -y service.lisp >/dev/null
ok "a detached checkout pins the release across restarts"
must_fail "running under plain lisp (assert-integrity throws)" -- $LISP service.lisp
echo ";; patched" >> service.lisp
must_fail "script modified after the release" -- $LISP_INTEGRITY -y service.lisp

step "02-requires: the check cascades to required modules"
repo 02-requires
$LISP_INTEGRITY -y service.lisp
echo "(def evil 1)" >> .lisp/util.lisp
must_fail "module modified after the release" -- $LISP_INTEGRITY -y service.lisp

step "all examples behaved as documented (03-signed is manual, see its README)"
