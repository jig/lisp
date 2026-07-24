package printer

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jig/lisp/marshaler"
	"github.com/jig/lisp/types"
)

// Pr_data prints readable Lisp data deterministically, wrapping collections
// at maxWidth rune columns. Scalar values are never split.
func Pr_data(value types.MalType, maxWidth int) string {
	if maxWidth < 1 {
		maxWidth = 1
	}
	return dataPrinter{maxWidth: maxWidth}.render(value, 0)
}

type dataPrinter struct {
	maxWidth int
}

func (p dataPrinter) render(value types.MalType, column int) string {
	flat := p.flat(value)
	if column+runeWidth(flat) <= p.maxWidth || isEmptyCollection(value) {
		return flat
	}

	switch value := value.(type) {
	case types.LispPrintable:
		return flat
	case types.List:
		if prefix, form, ok := simpleReaderMacro(value.Val); ok {
			return prefix + p.render(form, column+runeWidth(prefix))
		}
		if form, meta, ok := withMetaForm(value.Val); ok {
			head := "^" + p.flat(meta) + " "
			return head + p.render(form, column+runeWidth(head))
		}
		return p.renderList(value.Val, column)
	case types.Vector:
		return p.renderSequence(value.Val, column, "[", "]")
	case marshaler.HashMap:
		mapped, err := value.MarshalHashMap()
		if err != nil {
			return "{}"
		}
		if hashMap, ok := mapped.(types.HashMap); ok {
			return p.renderMap(hashMap, column)
		}
		return flat
	case types.HashMap:
		return p.renderMap(value, column)
	case types.Set:
		return p.renderSet(value, column)
	default:
		if indirect, ok := indirectData(value); ok {
			return p.render(indirect, column)
		}
		return flat
	}
}

func (p dataPrinter) flat(value types.MalType) string {
	switch value := value.(type) {
	case types.LispPrintable:
		return value.LispPrint(func(value types.MalType, _ bool) string {
			return p.flat(value)
		})
	case types.List:
		if prefix, form, ok := simpleReaderMacro(value.Val); ok {
			return prefix + p.flat(form)
		}
		if form, meta, ok := withMetaForm(value.Val); ok {
			return "^" + p.flat(meta) + " " + p.flat(form)
		}
		return p.flatSequence(value.Val, "(", ")")
	case types.Vector:
		return p.flatSequence(value.Val, "[", "]")
	case marshaler.HashMap:
		mapped, err := value.MarshalHashMap()
		if err != nil {
			return "{}"
		}
		if hashMap, ok := mapped.(types.HashMap); ok {
			return p.flatMap(hashMap)
		}
		return Pr_str(value, true)
	case types.HashMap:
		return p.flatMap(value)
	case types.Set:
		entries := p.sortedSetEntries(value)
		return "#{" + strings.Join(entries, " ") + "}"
	default:
		if indirect, ok := indirectData(value); ok {
			return p.flat(indirect)
		}
		return Pr_str(value, true)
	}
}

func (p dataPrinter) flatSequence(values []types.MalType, open, close string) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = p.flat(value)
	}
	return open + strings.Join(parts, " ") + close
}

func (p dataPrinter) flatMap(value types.HashMap) string {
	entries := p.sortedMapEntries(value)
	parts := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		parts = append(parts, entry.keyFlat, p.flat(entry.value))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func (p dataPrinter) renderSequence(values []types.MalType, column int, open, close string) string {
	if len(values) == 0 {
		return open + close
	}

	childColumn := column + runeWidth(open)
	var b strings.Builder
	b.WriteString(open)
	for i, value := range values {
		if i > 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", childColumn))
		}
		b.WriteString(p.render(value, childColumn))
	}
	b.WriteString(close)
	return b.String()
}

func (p dataPrinter) renderSet(value types.Set, column int) string {
	entries := p.sortedSetEntries(value)
	if len(entries) == 0 {
		return "#{}"
	}

	childColumn := column + 2
	var b strings.Builder
	b.WriteString("#{")
	for i, entry := range entries {
		if i > 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", childColumn))
		}
		b.WriteString(entry)
	}
	b.WriteByte('}')
	return b.String()
}

func (p dataPrinter) renderMap(value types.HashMap, column int) string {
	entries := p.sortedMapEntries(value)
	if len(entries) == 0 {
		return "{}"
	}

	childColumn := column + 1
	var b strings.Builder
	b.WriteByte('{')
	for i, entry := range entries {
		if i > 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", childColumn))
		}
		b.WriteString(entry.keyFlat)
		b.WriteByte(' ')
		valueColumn := childColumn + runeWidth(entry.keyFlat) + 1
		b.WriteString(p.render(entry.value, valueColumn))
	}
	b.WriteByte('}')
	return b.String()
}

func (p dataPrinter) renderList(values []types.MalType, column int) string {
	if len(values) == 0 {
		return "()"
	}

	childColumn := column + 2
	var b strings.Builder
	b.WriteByte('(')

	lineColumn := column + 1
	rendered := p.render(values[0], lineColumn)
	b.WriteString(rendered)
	lineColumn = columnAfter(lineColumn, rendered)

	for i, value := range values[1:] {
		flat := p.flat(value)
		if lineColumn+1+runeWidth(flat) <= p.maxWidth {
			b.WriteByte(' ')
			b.WriteString(flat)
			lineColumn += 1 + runeWidth(flat)
			continue
		}

		if i == 0 {
			rendered = p.render(value, lineColumn+1)
			b.WriteByte(' ')
			b.WriteString(rendered)
			lineColumn = columnAfter(lineColumn+1, rendered)
			continue
		}

		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", childColumn))
		rendered = p.render(value, childColumn)
		b.WriteString(rendered)
		lineColumn = columnAfter(childColumn, rendered)
	}
	b.WriteByte(')')
	return b.String()
}

type mapEntry struct {
	keyFlat string
	value   types.MalType
}

func (p dataPrinter) sortedMapEntries(value types.HashMap) []mapEntry {
	entries := make([]mapEntry, 0, len(value.Items))
	for key, item := range value.Items {
		entries = append(entries, mapEntry{keyFlat: p.flat(key), value: item})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].keyFlat < entries[j].keyFlat
	})
	return entries
}

func (p dataPrinter) sortedSetEntries(value types.Set) []string {
	entries := make([]string, 0, len(value.Items))
	for entry := range value.Items {
		entries = append(entries, p.flat(entry))
	}
	sort.Strings(entries)
	return entries
}

func indirectData(value types.MalType) (types.MalType, bool) {
	if _, ok := value.(fmt.Stringer); ok {
		return nil, false
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Kind() != reflect.Pointer || reflected.IsNil() {
		return nil, false
	}
	return reflect.Indirect(reflected).Interface(), true
}

func isEmptyCollection(value types.MalType) bool {
	switch value := value.(type) {
	case types.List:
		return len(value.Val) == 0
	case types.Vector:
		return len(value.Val) == 0
	case types.HashMap:
		return len(value.Items) == 0
	case types.Set:
		return len(value.Items) == 0
	default:
		return false
	}
}

func columnAfter(start int, value string) int {
	if newline := strings.LastIndexByte(value, '\n'); newline >= 0 {
		return runeWidth(value[newline+1:])
	}
	return start + runeWidth(value)
}

func runeWidth(value string) int {
	return utf8.RuneCountInString(value)
}

// Reader-macro sugar: the reader expands 'x → (quote x), `x →
// (quasiquote x), ~x → (unquote x), ~@x → (splice-unquote x), @x →
// (deref x), and ^meta form → (with-meta form meta). Pr_data prints
// these back sugared so state files read naturally; the forms still
// round-trip (the reader re-expands them to the same value).
var readerMacroPrefix = map[string]string{
	"quote":          "'",
	"quasiquote":     "`",
	"unquote":        "~",
	"splice-unquote": "~@",
	"deref":          "@",
}

// simpleReaderMacro returns the sugar prefix and wrapped form when list
// is a two-element reader-macro expansion like (quote x).
func simpleReaderMacro(list []types.MalType) (string, types.MalType, bool) {
	if len(list) != 2 {
		return "", nil, false
	}
	sym, ok := list[0].(types.Symbol)
	if !ok {
		return "", nil, false
	}
	prefix, ok := readerMacroPrefix[sym.Val]
	if !ok {
		return "", nil, false
	}
	return prefix, list[1], true
}

// withMetaForm returns the form and its metadata when list is a
// (with-meta form meta) expansion, printed as "^meta form".
func withMetaForm(list []types.MalType) (form, meta types.MalType, ok bool) {
	if len(list) != 3 {
		return nil, nil, false
	}
	sym, isSym := list[0].(types.Symbol)
	if !isSym || sym.Val != "with-meta" {
		return nil, nil, false
	}
	return list[1], list[2], true
}
