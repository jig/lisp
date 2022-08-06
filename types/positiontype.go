package types

import "fmt"

type Position struct {
	Module   *string
	BeginRow int
	BeginCol int
	Row      int
	Col      int
}

func NewCursorFile(module string) *Position {
	return &Position{
		Module:   &module,
		BeginRow: 1,
		BeginCol: 1,
	}
}

func NewAnonymousCursorHere(row, col int) *Position {
	return &Position{
		BeginRow: row,
		BeginCol: col,
		Row:      row,
		Col:      col,
	}
}

func NewCursorHere(moduleName string, row, col int) *Position {
	pos := NewAnonymousCursorHere(row, col)
	pos.Module = &moduleName
	return pos
}

func NewCursor() *Position {
	return &Position{
		BeginRow: 1,
		BeginCol: 1,
		Row:      1,
		Col:      1,
	}
}

func (p *Position) SetPos(row int) *Position {
	return &Position{
		BeginRow: row,
		BeginCol: 1,
		Row:      row,
		Col:      1,
	}
}

func (p *Position) Here(here *Position) *Position {
	if here.Module == nil {
		return &Position{
			Module:   p.Module,
			BeginRow: here.BeginRow,
			BeginCol: here.BeginCol,
			Row:      here.Row,
			Col:      here.Col,
		}
	}
	return &Position{
		Module:   here.Module,
		BeginRow: here.BeginRow,
		BeginCol: here.BeginCol,
		Row:      here.Row,
		Col:      here.Col,
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
			Row:      p.Row,
			Col:      p.Col,
			BeginRow: p.BeginRow,
			BeginCol: p.BeginCol,
		}
	}
	v := *p.Module
	return &Position{
		Module:   &v,
		Row:      p.Row,
		Col:      p.Col,
		BeginRow: p.BeginRow,
		BeginCol: p.BeginCol,
	}
}

func (c *Position) Close(here *Position) *Position {
	return &Position{
		Module:   c.Module,
		BeginRow: c.BeginRow,
		BeginCol: c.BeginCol,
		Row:      here.Row,
		Col:      here.Col,
	}
}

func (cursor *Position) String() string {
	if cursor == nil {
		return ""
	}
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
	if cursor.Row < 0 {
		return ""
	}
	return fmt.Sprintf("%d…%d,%d…%d", cursor.BeginRow, cursor.Row, cursor.BeginCol, cursor.Col)
}

func (cursor *Position) StringPositionRow() string {
	if cursor == nil {
		return ""
	}
	if cursor.Row < 0 {
		return ""
	}
	if cursor.BeginRow != cursor.Row {
		return fmt.Sprintf("%d…%d", cursor.BeginRow, cursor.Row)
	}
	return fmt.Sprintf("%d", cursor.Row)
}

func (cursor *Position) Includes(inside Position) bool {
	if cursor == nil {
		return false
	}
	return (cursor.BeginRow < inside.BeginRow ||
		(cursor.BeginRow == inside.BeginRow &&
			cursor.BeginCol <= inside.BeginCol)) &&
		(cursor.Row > inside.Row ||
			(cursor.Row == inside.Row &&
				cursor.Col >= inside.Col))
}
