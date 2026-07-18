# Releasing jig/lisp

How a tagged release is cut. The goal of this document (roadmap item
5.4) is that every future tag ships with notes — the historical tags up
to `v0.2.24` have none.

## Branch model

- **`develop`** — integration branch; every PR merges here.
- **`main`** — release branch; only fast-forwarded/merged from `develop`
  at release time. `origin/HEAD` points here.
- **Tags** `vMAJOR.MINOR.PATCH` live on `main`.

Pre-1.0, so a **minor** bump may carry breaking changes (e.g. the
`coreextented` → `coreextended` rename and the error-format change land
in 0.3). Anything users must act on goes in `CHANGELOG.md` under
**Changed** with a migration note.

## Cutting a release

1. **Make sure `develop` is green** — the `test`, `race`, `fuzz` and
   `quality` CI jobs all pass.

2. **Finalise the changelog.** In `CHANGELOG.md`, rename the
   `## Unreleased (since vX)` heading to `## vNEW — YYYY-MM-DD` and open
   a fresh empty `## Unreleased` above it. This section becomes the
   release notes, so keep the **Changed / breaking** items at the top.

3. **Merge `develop` into `main`.**

   ```bash
   git checkout main && git merge --ff-only develop   # or a merge commit
   git push origin main
   ```

4. **Tag `main` with an annotated tag** (historical tags are lightweight;
   prefer annotated from now on so `git describe` and the release carry a
   message):

   ```bash
   git tag -a vNEW -m "jig/lisp vNEW"
   git push origin vNEW
   ```

5. **Publish the GitHub release** with the changelog section as the body:

   ```bash
   # notes taken from the just-finalised CHANGELOG section
   gh release create vNEW --title "vNEW" --notes-file <(sed -n '/## vNEW/,/## v/p' CHANGELOG.md)
   # or let GitHub auto-generate from merged PRs and edit afterwards:
   # gh release create vNEW --generate-notes
   ```

6. **Verify** the module is installable at the new tag:

   ```bash
   go install -tags debugger github.com/jig/lisp/cmd/lisp@vNEW
   ```

## Backfilling older tags (optional)

The tags up to `v0.2.24` predate the changelog and have no GitHub
release. Backfilling is optional; if wanted, `gh release create <tag>
--generate-notes` produces reasonable notes from the merged PRs between
tags. Start with the next release rather than spending effort on old
ones.
