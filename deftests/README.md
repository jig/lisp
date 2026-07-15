# deftests

The `deftest` replica of the historical `tests/step*.mal` suites, run by
`TestDeftests` (and by hand with `lisp --test deftests/` from the
repository root — `step6_file_test.lisp` and `stepA_mal_test.lisp` load
fixtures through paths relative to it). The originals stay untouched and
keep running under the line-based Go harness; the two suites complement
each other.

## Conventions

- Assertions compare **values**: `(is (= expected actual))`. The legacy
  `;=>` harness compares **printed forms** instead, so printer output is
  asserted here only where printing is the point, via
  `(is (= "…" (pr-str …)))`.
- Error expectations (`;/regex`) become
  `(is (= :threw (try … (catch e :threw))))`, or a `pr-str` comparison
  of the caught value when the message matters.
- Cases that assert **stdout** (`prn`/`println` output) have no deftest
  equivalent (there is no `with-out-str`) and stay in the legacy
  harness only.
- Multi-key hash-map / multi-element set printing has no stable order:
  compare as values, or accept both orders with `or`.
- `«…»` reader literals need an environment-aware reader and cannot be
  read through `load-file`; those cases are asserted through values and
  `pr-str`.
- Test names and top-level helper `def`s are prefixed per file (all
  files share one environment, and deftest re-registration replaces by
  name).

## Not replicated

- `tests/step0_repl.mal` — REPL echo behaviour; also skipped by the Go
  harness.
- `tests/stepI_marshaling copy.mal` — depends on example marshal types
  registered only by the Go test harness, not by the `lisp` binary.
- Documented divergence pinned in `step1_read_print_test.lisp`: commas
  are **not** whitespace in jig/lisp (they read as a `,` symbol), unlike
  kanaka/mal and Clojure.
