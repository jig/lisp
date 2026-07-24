package lisperror_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/types"
)

func pos(module string, row int) *types.Position {
	var m *string
	if module != "" {
		m = &module
	}
	return &types.Position{Module: m, Row: row, Col: 1, BeginRow: row, BeginCol: 1}
}

// TestErrorFormat pins the rendered form of an error: "module:row: msg"
// with a cursor, bare message without. Embedders match these with
// strings.Contains, so the shape must not drift silently.
func TestErrorFormat(t *testing.T) {
	if got := lisperror.NewLispError(errors.New("boom"), nil).Error(); got != "boom" {
		t.Fatalf("no-cursor Error = %q, want %q", got, "boom")
	}
	got := lisperror.NewLispError(errors.New("boom"), pos("mod", 3)).Error()
	if got != "mod:3: boom" {
		t.Fatalf("cursor Error = %q, want %q", got, "mod:3: boom")
	}
}

// TestErrorNonErrorPayload covers the branch that renders a thrown
// non-error value (e.g. (throw {:a 1})) with the Lisp printer.
func TestErrorNonErrorPayload(t *testing.T) {
	e := lisperror.NewLispError(types.HashMap{Items: map[types.MalType]types.MalType{types.KW("a"): 1}}, nil)
	if got := e.Error(); got != "{:a 1}" {
		t.Fatalf("payload Error = %q, want %q", got, "{:a 1}")
	}
	if _, ok := e.ErrorValue().(types.HashMap); !ok {
		t.Fatalf("ErrorValue should be the thrown HashMap, got %T", e.ErrorValue())
	}
}

// TestUnwrapAndIs covers Unwrap and Is for errors.Is/As interop.
func TestUnwrapAndIs(t *testing.T) {
	sentinel := errors.New("sentinel")
	le := lisperror.NewLispError(sentinel, nil)

	if !errors.Is(le, sentinel) {
		t.Fatal("errors.Is(lispErr, sentinel) should hold via Unwrap")
	}
	// Non-error payload: Unwrap yields nil, so errors.Is on an unrelated
	// target must not match.
	nonErr := lisperror.NewLispError("just a string", nil)
	if errors.Is(nonErr, sentinel) {
		t.Fatal("non-error payload should not unwrap to sentinel")
	}
}

// TestNewGoError covers the Go-error wrapper used for builtin failures.
func TestNewGoError(t *testing.T) {
	err := lisperror.NewGoError("myfn", errors.New("boom"))
	if got := err.Error(); !strings.Contains(got, "myfn") || !strings.Contains(got, "boom") {
		t.Fatalf("NewGoError = %q, want it to mention myfn and boom", got)
	}
	// Non-error second argument is still wrapped.
	err2 := lisperror.NewGoError("myfn", "raw value")
	if !strings.Contains(err2.Error(), "raw value") {
		t.Fatalf("NewGoError(non-error) = %q", err2.Error())
	}
}

// TestMarshalHashMap covers the structured form used when an error is
// serialised as Lisp data.
func TestMarshalHashMap(t *testing.T) {
	e := lisperror.NewLispError(errors.New("boom"), pos("mod", 3))
	m, err := e.MarshalHashMap()
	if err != nil {
		t.Fatal(err)
	}
	hm, ok := m.(types.HashMap)
	if !ok {
		t.Fatalf("MarshalHashMap returned %T, want HashMap", m)
	}
	if !strings.Contains(hm.Items[types.KW("err")].(string), "boom") {
		t.Fatalf(":err = %v, want it to contain boom", hm.Items[types.KW("err")])
	}
	if _, ok := hm.Items[types.KW("pos")]; !ok {
		t.Fatal(":pos missing for an error with a cursor")
	}
}

// TestLispPrint covers the «error …» rendering.
func TestLispPrint(t *testing.T) {
	e := lisperror.NewLispError(errors.New("boom"), nil)
	got := e.LispPrint(func(v types.MalType, _ bool) string {
		if err, ok := v.(error); ok {
			return err.Error()
		}
		return "?"
	})
	if !strings.HasPrefix(got, "«error") || !strings.Contains(got, "boom") {
		t.Fatalf("LispPrint = %q", got)
	}
}

// TestAddStackFrame covers frame appending and the same-position dedup.
func TestAddStackFrame(t *testing.T) {
	base := lisperror.NewLispError(errors.New("boom"), pos("mod", 3))

	withFrame := base.AddStackFrame(pos("mod", 5), "myfn")
	got := withFrame.Error()
	if !strings.Contains(got, "at myfn") {
		t.Fatalf("Error after AddStackFrame = %q, want an 'at myfn' frame", got)
	}

	// A frame at the cursor position with no function name is dropped.
	deduped := base.AddStackFrame(pos("mod", 3), "")
	if strings.Contains(deduped.Error(), "\n  at") {
		t.Fatalf("duplicate cursor frame should be skipped, got %q", deduped.Error())
	}
}

// TestPosition covers the accessor.
func TestPosition(t *testing.T) {
	p := pos("mod", 7)
	e := lisperror.NewLispError(errors.New("x"), p)
	if e.Position() == nil || e.Position().Row != 7 {
		t.Fatalf("Position = %v, want row 7", e.Position())
	}
}
