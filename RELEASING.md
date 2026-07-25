# Releasing jig/lisp

How versions are published. Rewritten 2026-07-25 after the 0.3.x
incident; the previous `main`-based flow is retired.

## Ground rules

- **Tags are the release mechanism.** They are placed directly on the
  commit to release (normally on `develop`, or on a `release/*` branch
  when cherry-picking). `main` is not part of the flow and stays
  untouched; GitHub Releases are not used for now.
- **A public semver tag is immediately live**: the Go module proxy
  archives it — immutably — the first time anyone resolves it, and a
  plain release tag becomes the `@latest` answer at once. There is no
  such thing as a quiet release tag. Do not create `vX.Y.Z` until that
  version should be what `go install …@latest` delivers.
- **Pre-release tags are the validation tool.** `vX.Y.Z-rc.N` (or
  `-beta.N`) can be tagged and shared freely: the go command never
  selects pre-releases for `@latest` while any release version exists,
  so users can `go install …@v0.6.0-rc.1` explicitly without affecting
  everyone else.
- **`@latest` currently must resolve to `v0.2.24`** until embedders
  have been validated against the new features. Nothing above v0.2.24
  gets a release (non-pre-release) tag until then.

## Version numbering

| Range | Status |
| ----- | ------ |
| ≤ v0.2.24 | Historical releases; v0.2.24 is the current `@latest`. |
| v0.3.0, v0.3.1 | **Retracted** (published prematurely; v0.3.1 is retraction-only). Never reuse. |
| v0.4.x | **Skipped forever** — pre-release documents used "0.4" for the keyword work; a `retract [v0.4.0, v0.4.99]` guard hides any accidental tag. |
| v0.5.0 | Will ship the pre-keyword line: everything since v0.2.24 up to (excluding) the keyword-type migration — the tree at the merge of #133 (`30349af`). **`v0.5.0-rc.1` is published at that commit (2026-07-25)** for embedder validation; when validated, tag `v0.5.0` on the same commit to flip `@latest`. |
| v0.6.0 | Will ship the keyword-type migration (the Go-embedder breaking change) and everything after. |

The `retract` block in `go.mod` **must be carried unchanged into every
future release**: the go command reads retractions from the highest
published version, so dropping the block would resurrect the retracted
versions in listings.

## Cutting a release

1. `develop` green in CI (test, race, fuzz, quality, golangci).
2. Finalise `CHANGELOG.md`: rename the target `## X.Y.Z (unreleased)`
   heading to `## vX.Y.Z — YYYY-MM-DD` (keep ⚠️ items at the top) and
   merge that via PR.
3. Check `go.mod` still contains the `retract` block.
4. Tag the release commit with an **annotated tag** and push it:

   ```bash
   git tag -a vX.Y.Z -m "jig/lisp vX.Y.Z"
   git push origin vX.Y.Z
   ```

   For a validation cycle, tag `vX.Y.Z-rc.N` instead — same commands.
5. Verify resolution and installation:

   ```bash
   go list -m github.com/jig/lisp@vX.Y.Z     # forces the proxy to index it
   go install -tags debugger github.com/jig/lisp/cmd/lisp@vX.Y.Z
   go list -m github.com/jig/lisp@latest     # release tags only: confirm the flip
   ```

   The sum DB may answer 500 for a brand-new tag; retry after a few
   seconds. `@latest` may lag a few minutes behind (proxy cache).

## Undoing a mistake

A published version cannot be unpublished (proxy and sum DB are
immutable). The correct tool is `retract`: add the bad version to the
`retract` block and publish a new highest version carrying it (a
retraction-only patch release based on the last good tree, as v0.3.1
does). `@latest` then falls back to the highest non-retracted version.
Keep the git tag of the bad version pointing at the tree the proxy
recorded, so `GOPROXY=direct` users do not get checksum mismatches.
