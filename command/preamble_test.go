package command

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
)

func preambleTestEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	concurrent.Load(ns)
	ns.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return lisp.EVAL(ctx, a[0], ns)
	}})
	ctx := context.Background()
	for _, header := range []string{core.HeaderBasic(), core.HeaderLoadFile(), concurrent.HeaderConcurrent()} {
		if _, err := lisp.REPL(ctx, ns, header, types.NewCursorFile("preamble")); err != nil {
			t.Fatalf("header: %v", err)
		}
	}
	return ns
}

func writeScript(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunScript_PreambleFlags(t *testing.T) {
	script := writeScript(t, "flags.lisp", "(str \"v=\" $A \"/\" $B)\n")
	ns := preambleTestEnv(t)
	out, err := runScript(context.Background(), ns, script,
		[]string{"$A 42", `$B "hola"`}, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if out != `"v=42/hola"` {
		t.Errorf("expected \"v=42/hola\", got %v", out)
	}
}

func TestRunScript_InFilePreambleDefaultsAndOverride(t *testing.T) {
	script := writeScript(t, "defaults.lisp",
		";; $A 1\n"+
			";; $B 2\n"+
			"(+ $A $B)\n")
	ns := preambleTestEnv(t)

	// In-file defaults alone.
	out, err := runScript(context.Background(), ns, script, nil, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript defaults: %v", err)
	}
	if out != "3" {
		t.Errorf("expected 3 from in-file defaults, got %v", out)
	}

	// --preamble overrides the in-file value.
	out, err = runScript(context.Background(), ns, script,
		[]string{"$A 10"}, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript override: %v", err)
	}
	if out != "12" {
		t.Errorf("expected 12 with override, got %v", out)
	}
}

func TestRunScript_ClassicPathUnchanged(t *testing.T) {
	// No placeholders anywhere → the classic load-file path runs.
	script := writeScript(t, "plain.lisp", "(str \"plain\")\n")
	ns := preambleTestEnv(t)
	out, err := runScript(context.Background(), ns, script, nil, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if out != `"plain"` {
		t.Errorf("expected \"plain\", got %v", out)
	}
}

func TestRunScript_MissingPlaceholderStillDefined(t *testing.T) {
	// A placeholder with no value provided anywhere reads as nil (the
	// historical READWithPreamble behaviour, kept for compatibility).
	script := writeScript(t, "missing.lisp", ";; $A 1\n(if $UNSET \"set\" \"unset\")\n")
	ns := preambleTestEnv(t)
	out, err := runScript(context.Background(), ns, script, nil, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if out != `"unset"` {
		t.Errorf("expected \"unset\", got %v", out)
	}
}

func TestParsePreambleAssignments_Errors(t *testing.T) {
	ns := preambleTestEnv(t)
	for _, bad := range []string{"", "NAME 1", "$MODULE x", "$ONLYNAME"} {
		if err := parsePreambleAssignments([]string{bad}, ns, map[string]types.MalType{}); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	// the `;; ` file prefix is accepted
	into := map[string]types.MalType{}
	if err := parsePreambleAssignments([]string{";; $X 7"}, ns, into); err != nil {
		t.Fatalf("prefixed assignment: %v", err)
	}
	if v, ok := into["$X"].(int); !ok || v != 7 {
		t.Errorf("expected $X=7, got %#v", into["$X"])
	}
}
