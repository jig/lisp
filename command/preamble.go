package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/integrity"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"
)

// preambleLineRE matches one placeholder assignment, `$NAME <expr>`,
// optionally with the `;; ` comment prefix used inside files.
var preambleLineRE = regexp.MustCompile(`^(?:;; )?(\$[-\w\d]+)\s+(.+)$`)

// parsePreambleAssignments merges --preamble flag values into a
// placeholder map. Each assignment is `$NAME <lisp expression>` (the
// leading `;; ` used inside files is accepted too).
func parsePreambleAssignments(assignments []string, ns types.EnvType, into map[string]types.MalType) error {
	for _, a := range assignments {
		m := preambleLineRE.FindStringSubmatch(strings.TrimSpace(a))
		if m == nil {
			return fmt.Errorf("invalid --preamble %q: expected \"$NAME <expression>\"", a)
		}
		if m[1] == "$MODULE" {
			return fmt.Errorf("invalid --preamble %q: $MODULE is reserved", a)
		}
		value, err := lisp.READ(m[2], types.NewCursorFile("--preamble"), ns)
		if err != nil {
			return fmt.Errorf("invalid --preamble %q: %w", a, err)
		}
		into[m[1]] = value
	}
	return nil
}

// inFilePreamble collects the leading `;; $NAME <expr>` lines of a
// script as placeholder defaults. The lines are left in place (the
// tokenizer treats them as comments), so source rows are unaffected.
func inFilePreamble(content string, ns types.EnvType) map[string]types.MalType {
	out := map[string]types.MalType{}
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, ";; $") {
			break
		}
		m := preambleLineRE.FindStringSubmatch(line)
		if m == nil || m[1] == "$MODULE" {
			continue
		}
		if value, err := lisp.READ(m[2], types.NewCursorFile("preamble"), ns); err == nil {
			out[m[1]] = value
		}
	}
	return out
}

// runScript executes a script file. Without placeholder assignments and
// without in-file preamble lines it takes the classic `(load-file …)`
// path (bootstrapCursor is the cursor of that synthetic call — the DAP
// server passes an anonymous one so the debugger skips it). Otherwise
// the file is read in Go and its `$NAME` placeholders are filled from
// the in-file preamble lines (defaults) overridden by the --preamble
// assignments; the source is evaluated with the same `;; $MODULE`
// wrapper load-file builds, so source rows — and therefore breakpoints
// — keep matching the on-disk file.
func runScript(ctx context.Context, env types.EnvType, fileName string, preamble []string, bootstrapCursor *types.Position) (types.MalType, error) {
	abs, err := filepath.Abs(fileName)
	if err != nil {
		abs = fileName
	}
	registerModule(fileName, abs)

	// *FILE* is the absolute path of the script being executed — the
	// self-referential counterpart of *ARGV* (cf. Clojure's *file*) —
	// so a script can slurp or spit its own source. Defined for script
	// runs only; the REPL and -e leave it unset. load-file does not
	// rebind it: it always names the top-level script.
	env.Set(types.Symbol{Val: "*FILE*"}, abs)

	contentBytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	// Under --integrity, verify the exact bytes about to be evaluated.
	// integrity.Enable verified the script from an earlier, independent
	// read; the preamble branch below evaluates *this* buffer directly
	// (not through load-file/slurp-source), so without this check the
	// two reads could diverge — a worktree file swapped for a FIFO fed
	// concurrently runs unverified code under a "verified" run. No-op
	// when integrity mode is off. The load-file fast path re-verifies
	// through slurp-source, so the check there is merely redundant.
	if err := integrity.VerifyFile(abs, contentBytes); err != nil {
		return nil, err
	}
	content := string(contentBytes)

	values := inFilePreamble(content, env)
	if len(preamble) == 0 && len(values) == 0 {
		return lisp.REPL(ctx, env, `(load-file "`+fileName+`")`, bootstrapCursor)
	}
	if err := parsePreambleAssignments(preamble, env, values); err != nil {
		return nil, err
	}

	ast, err := reader.Read_program(content, types.NewCursorFile(fileName), &types.HashMap{Val: values}, env)
	if err != nil {
		return nil, err
	}
	res, err := lisp.EVAL(ctx, ast, env)
	if err != nil {
		return nil, err
	}
	return lisp.PRINT(res), nil
}
