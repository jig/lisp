package types

import "fmt"

type Position struct {
	Module   *string
	BeginRow int
	BeginCol int
	EndRow   int
	EndCol   int
}

func NewCursor() *Position {
	return &Position{
		BeginRow: 1,
		BeginCol: 1,
		EndRow:   1,
		EndCol:   1,
	}
}

func NewCursorFile(module string) *Position {
	return &Position{
		Module:   &module,
		BeginRow: 1,
		BeginCol: 1,
		EndRow:   1,
		EndCol:   1,
	}
}

func NewAnonymousCursorHere(row, col int) *Position {
	return &Position{
		BeginRow: row,
		BeginCol: col,
		EndRow:   row,
		EndCol:   col,
	}
}

func NewCursorHere(moduleName string, row, col int) *Position {
	pos := NewAnonymousCursorHere(row, col)
	pos.Module = &moduleName
	return pos
}

func (p *Position) SetPos(row int) *Position {
	return &Position{
		BeginRow: row,
		BeginCol: 1,
		EndRow:   row,
		EndCol:   1,
	}
}

func (p *Position) Here(here *Position) *Position {
	if here.Module == nil {
		return &Position{
			Module:   p.Module,
			BeginRow: here.BeginRow,
			BeginCol: here.BeginCol,
			EndRow:   here.EndRow,
			EndCol:   here.EndCol,
		}
	}
	return &Position{
		Module:   here.Module,
		BeginRow: here.BeginRow,
		BeginCol: here.BeginCol,
		EndRow:   here.EndRow,
		EndCol:   here.EndCol,
	}
}

// func (p *Position) Row(row int) *Position {
// 	p := &Position{}
// }

func (p *Position) Copy() *Position {
	if p == nil {
		return nil
	}
	if p.Module == nil {
		return &Position{
			EndRow:   p.EndRow,
			EndCol:   p.EndCol,
			BeginRow: p.BeginRow,
			BeginCol: p.BeginCol,
		}
	}
	v := *p.Module
	return &Position{
		Module:   &v,
		EndRow:   p.EndRow,
		EndCol:   p.EndCol,
		BeginRow: p.BeginRow,
		BeginCol: p.BeginCol,
	}
}

func (c *Position) Close(here *Position) *Position {
	return &Position{
		Module:   c.Module,
		BeginRow: c.BeginRow,
		BeginCol: c.BeginCol,
		EndRow:   here.EndRow,
		EndCol:   here.EndCol,
	}
}

func (cursor Position) String() string {
	return cursor.StringModule() + "§" + cursor.StringPosition()
}

func (cursor *Position) StringModule() string {
	if cursor == nil {
		return ""
	}
	moduleName := ""
	if cursor.Module != nil {
		moduleName = *cursor.Module
	}
	return moduleName
}

func (cursor *Position) StringPosition() string {
	if cursor == nil {
		return ""
	}
	if cursor.EndRow < 0 {
		return ""
	}
	if cursor.BeginRow == cursor.EndRow {
		if cursor.BeginCol == cursor.EndCol {
			return fmt.Sprintf("L%d,C%d", cursor.EndRow, cursor.EndCol)
		}
		return fmt.Sprintf("L%d,C%d…%d", cursor.EndRow, cursor.BeginCol, cursor.EndCol)
	}
	return fmt.Sprintf("L%d…L%d", cursor.BeginRow, cursor.EndRow)
}

func (cursor *Position) StringPositionRow() string {
	if cursor == nil {
		return ""
	}
	if cursor.EndRow < 0 {
		return ""
	}
	if cursor.BeginRow != cursor.EndRow {
		return fmt.Sprintf("%d…%d", cursor.BeginRow, cursor.EndRow)
	}
	return fmt.Sprintf("%d", cursor.EndRow)
}

func (cursor *Position) Includes(inside Position) bool {
	if cursor == nil {
		return false
	}
	if inside.BeginRow < cursor.BeginRow {
		return false
	}
	if inside.EndRow > cursor.EndRow {
		return false
	}
	if inside.BeginRow > cursor.BeginRow && inside.EndRow < cursor.EndRow {
		return true
	}
	// starts on same row
	if inside.BeginRow == cursor.BeginRow && inside.BeginCol >= cursor.BeginCol {
		if inside.EndRow < cursor.EndRow {
			return true
		}
		if inside.EndRow == cursor.EndRow && inside.EndCol <= cursor.EndCol {
			return true
		}
	}
	// ends on same row
	if inside.EndRow == cursor.EndRow && inside.EndCol <= cursor.EndCol {
		if inside.BeginRow > cursor.BeginRow {
			return true
		}
		if inside.BeginRow == cursor.BeginRow && inside.BeginCol >= cursor.BeginCol {
			return true
		}
	}
	return false
	// return (cursor.BeginRow < inside.BeginRow || (cursor.BeginRow == inside.BeginRow && cursor.BeginCol <= inside.BeginCol)) &&
	// 	(cursor.EndRow > inside.EndRow || (cursor.EndRow == inside.EndRow && cursor.EndCol >= inside.EndCol))
}
