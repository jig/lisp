package types

import "testing"

func TestIncludesSimple(t *testing.T) {
	pos := Position{
		BeginRow: 1,
		BeginCol: 1,
		EndRow:   1,
		EndCol:   2,
	}
	if !pos.Includes(*NewAnonymousCursorHere(1, 1)) {
		t.Fatal()
	}
	if !pos.Includes(*NewAnonymousCursorHere(1, 2)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(1, 0)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(1, 3)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(0, 1)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(2, 1)) {
		t.Fatal()
	}
}

func TestIncludesLong(t *testing.T) {
	pos := Position{
		BeginRow: 2,
		BeginCol: 2,
		EndRow:   10,
		EndCol:   2,
	}
	// before
	if pos.Includes(*NewAnonymousCursorHere(0, 1)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(1, 0)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(1, 3)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(1, 20)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(2, 1)) {
		t.Fatal()
	}
	// in
	if !pos.Includes(*NewAnonymousCursorHere(2, 2)) {
		t.Fatal()
	}
	if !pos.Includes(*NewAnonymousCursorHere(2, 3)) {
		t.Fatal()
	}
	if !pos.Includes(*NewAnonymousCursorHere(2, 20)) {
		t.Fatal()
	}
	if !pos.Includes(*NewAnonymousCursorHere(2, 20)) {
		t.Fatal()
	}
	if !pos.Includes(*NewAnonymousCursorHere(10, 1)) {
		t.Fatal()
	}
	if !pos.Includes(*NewAnonymousCursorHere(10, 2)) {
		t.Fatal()
	}
	// after
	if pos.Includes(*NewAnonymousCursorHere(10, 3)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(11, 0)) {
		t.Fatal()
	}
	if pos.Includes(*NewAnonymousCursorHere(10, 3)) {
		t.Fatal()
	}
}
