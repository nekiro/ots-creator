// Package itemsxml reads the items.xml of a server and writes changes to it
// without touching the rest of the file: every item that is not changed
// keeps its bytes, so comments, formatting and unknown data survive.
package itemsxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ErrNoItems = errors.New("items.xml: no <items> element")

// Field is an attribute of an <item> element other than its ids, name,
// article and plural (editorsuffix, for example).
type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Attribute is an <attribute key value> element; fields like "field" or
// "abilities" nest further attributes.
type Attribute struct {
	Key      string      `json:"key"`
	Value    string      `json:"value"`
	Children []Attribute `json:"children"`
}

// Item is the data of one item id.
type Item struct {
	Name       string      `json:"name"`
	Article    string      `json:"article"`
	Plural     string      `json:"plural"`
	Fields     []Field     `json:"fields"`
	Attributes []Attribute `json:"attributes"`
}

func (item *Item) Clone() *Item {
	if item == nil {
		return nil
	}
	clone := *item
	clone.Fields = slices.Clone(item.Fields)
	clone.Attributes = cloneAttributes(item.Attributes)
	return &clone
}

func cloneAttributes(attributes []Attribute) []Attribute {
	clone := slices.Clone(attributes)
	for i := range clone {
		clone[i].Children = cloneAttributes(clone[i].Children)
	}
	return clone
}

func (item *Item) Equal(other *Item) bool {
	return item.Name == other.Name && item.Article == other.Article && item.Plural == other.Plural &&
		slices.Equal(item.Fields, other.Fields) && slices.EqualFunc(item.Attributes, other.Attributes, Attribute.equal)
}

func (attribute Attribute) equal(other Attribute) bool {
	return attribute.Key == other.Key && attribute.Value == other.Value &&
		slices.EqualFunc(attribute.Children, other.Children, Attribute.equal)
}

// Attribute returns the value of the first attribute with a key (keys
// compare without case, as servers read them).
func (item *Item) Attribute(key string) (string, bool) {
	for _, attribute := range item.Attributes {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Value, true
		}
	}
	return "", false
}

// SetAttribute sets the value of the attribute with a key, adding it at the
// end when the item has none.
func (item *Item) SetAttribute(key, value string) {
	for i := range item.Attributes {
		if strings.EqualFold(item.Attributes[i].Key, key) {
			item.Attributes[i].Value = value
			return
		}
	}
	item.Attributes = append(item.Attributes, Attribute{Key: key, Value: value})
}

func (item *Item) RemoveAttribute(key string) {
	item.Attributes = slices.DeleteFunc(item.Attributes, func(attribute Attribute) bool {
		return strings.EqualFold(attribute.Key, key)
	})
}

// entry is one <item> element: a single id or a range sharing its data.
type entry struct {
	from, to   uint16
	start, end int // the element in File.data
	item       *Item
}

// File is a parsed items.xml. It is immutable: Encode returns the changed
// file.
type File struct {
	data    []byte
	entries []*entry
	byID    map[uint16]*entry
	// latin1 is set for a file that is not UTF-8: its bytes are read and
	// written as ISO-8859-1.
	latin1 bool
	// closing is where </items> starts (len(data) when it is missing).
	closing  int
	indent   string
	newline  string
	selfEnd  string
	maxID    uint16
	numItems int
}

func Parse(data []byte) (*File, error) {
	file := &File{data: data, byID: map[uint16]*entry{}, closing: len(data), indent: "\t", newline: "\n", selfEnd: " />"}
	// Text written to a file that declares another charset stays in it.
	prolog := bytes.ToLower(data[:min(len(data), 200)])
	declared := bytes.Contains(prolog, []byte("encoding=")) && !bytes.Contains(prolog, []byte("utf-8"))
	file.latin1 = !utf8.Valid(data) || (declared && !bytes.ContainsFunc(data, func(char rune) bool { return char >= 0x80 }))
	if bytes.Contains(data, []byte("\r\n")) {
		file.newline = "\r\n"
	}
	if !bytes.Contains(data, []byte(" />")) && bytes.Contains(data, []byte("/>")) {
		file.selfEnd = "/>"
	}
	// The decoder only reads UTF-8: a file in another charset is read from
	// a UTF-8 copy, and offset maps the offsets in the copy back.
	source, offset := data, func(at int64) int { return int(at) }
	if file.latin1 {
		var wide []int // offsets in the copy after each two byte character
		source = make([]byte, 0, len(data)+len(data)/8)
		for _, char := range data {
			source = utf8.AppendRune(source, rune(char))
			if char >= 0x80 {
				wide = append(wide, len(source))
			}
		}
		offset = func(at int64) int {
			before, _ := slices.BinarySearch(wide, int(at)+1)
			return int(at) - before
		}
	}
	decoder := xml.NewDecoder(bytes.NewReader(source))
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	depth, found := 0, false
	// open holds the attribute lists being filled: the one of the item and
	// of every nested attribute.
	var current *entry
	var open []*[]Attribute
	for {
		start := offset(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("items.xml: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			depth++
			name := strings.ToLower(element.Name.Local)
			switch {
			case depth == 1:
				if name != "items" {
					return nil, ErrNoItems
				}
				found = true
			case depth == 2 && name == "item":
				current = file.newEntry(element, start)
				open = []*[]Attribute{&current.item.Attributes}
			case current != nil && name == "attribute":
				list := open[len(open)-1]
				*list = append(*list, file.attribute(element))
				open = append(open, &(*list)[len(*list)-1].Children)
			case current != nil:
				// Anything else inside an item is not kept when the item
				// is rewritten.
				open = append(open, &[]Attribute{})
			}
		case xml.EndElement:
			depth--
			switch {
			case depth == 0:
				file.closing = start
			case depth == 1 && current != nil:
				current.end = offset(decoder.InputOffset())
				file.add(current)
				current, open = nil, nil
			case current != nil && len(open) > 1:
				open = open[:len(open)-1]
			}
		}
		if depth == 0 && found {
			break
		}
	}
	if !found {
		return nil, ErrNoItems
	}
	if len(file.entries) > 0 {
		file.indent = file.indentBefore(file.entries[0].start)
	}
	return file, nil
}

func (f *File) newEntry(element xml.StartElement, start int) *entry {
	created := &entry{start: start, item: &Item{}}
	var id, from, to string
	for _, attr := range element.Attr {
		value := attr.Value
		switch key := strings.ToLower(attr.Name.Local); key {
		case "id":
			id = value
		case "fromid":
			from = value
		case "toid":
			to = value
		case "name":
			created.item.Name = value
		case "article":
			created.item.Article = value
		case "plural":
			created.item.Plural = value
		default:
			created.item.Fields = append(created.item.Fields, Field{Key: attr.Name.Local, Value: value})
		}
	}
	created.from, created.to = parseRange(id, from, to)
	return created
}

func (f *File) attribute(element xml.StartElement) Attribute {
	var attribute Attribute
	for _, attr := range element.Attr {
		switch strings.ToLower(attr.Name.Local) {
		case "key":
			attribute.Key = attr.Value
		case "value":
			attribute.Value = attr.Value
		}
	}
	return attribute
}

func (f *File) add(added *entry) {
	if added.from == 0 {
		return
	}
	f.entries = append(f.entries, added)
	f.numItems += int(added.to-added.from) + 1
	f.maxID = max(f.maxID, added.to)
	for id := uint32(added.from); id <= uint32(added.to); id++ {
		if _, ok := f.byID[uint16(id)]; !ok {
			f.byID[uint16(id)] = added
		}
	}
}

func parseRange(id, from, to string) (uint16, uint16) {
	parse := func(text string) uint16 {
		number, _ := strconv.ParseUint(strings.TrimSpace(text), 10, 16)
		return uint16(number)
	}
	if id != "" {
		single := parse(id)
		return single, single
	}
	first, last := parse(from), parse(to)
	if last < first {
		last = first
	}
	return first, last
}

// indentBefore returns the white space between the start of the line and
// offset.
func (f *File) indentBefore(offset int) string {
	lineStart := bytes.LastIndexByte(f.data[:offset], '\n') + 1
	indent := f.data[lineStart:offset]
	if len(bytes.TrimLeft(indent, " \t")) != 0 {
		return "\t"
	}
	return string(indent)
}

// Data is the file as it was parsed.
func (f *File) Data() []byte { return f.data }

// Items is the number of item ids the file describes, MaxID the highest.
func (f *File) Items() int    { return f.numItems }
func (f *File) MaxID() uint16 { return f.maxID }

// Get returns a copy of the data of an item id, false when the file has
// none.
func (f *File) Get(id uint16) (*Item, bool) {
	found, ok := f.byID[id]
	if !ok {
		return nil, false
	}
	return found.item.Clone(), true
}

func (f *File) Has(id uint16) bool {
	_, ok := f.byID[id]
	return ok
}

// Changes is what Encode changes. Ids are the ones of the file.
type Changes struct {
	// Moves gives items new ids (old to new). An item at an id that
	// another one moves to, and that does not move itself, is dropped.
	Moves map[uint16]uint16
	// Items sets the data of the items at these ids (the ids after Moves),
	// adding the ones the file does not have.
	Items map[uint16]*Item
}

// Encode returns the file with the changes applied. Elements of items that
// do not change keep their bytes.
func (f *File) Encode(changes Changes) []byte {
	dropped := map[uint16]bool{}
	for from, to := range changes.Moves {
		if _, moves := changes.Moves[to]; !moves && from != to {
			dropped[to] = true
		}
	}
	pending := map[uint16]*Item{}
	for id, item := range changes.Items {
		pending[id] = item
	}
	var pieces []piece
	// anchors are the ends of the elements new items can follow, by the
	// highest id of the element.
	type anchor struct {
		id uint16
		at int
	}
	var anchors []anchor
	for _, current := range f.entries {
		runs, changed := f.rewrite(current, changes.Moves, dropped, pending)
		highest := current.to
		if changed && len(runs) == 0 {
			pieces = append(pieces, f.removal(current.start, current.end))
			continue
		}
		if changed {
			highest = 0
			texts := make([]string, len(runs))
			for i, written := range runs {
				texts[i] = f.element(written.from, written.to, written.item)
				highest = max(highest, written.to)
			}
			pieces = append(pieces, piece{at: current.start, end: current.end, text: strings.Join(texts, f.newline+f.indent)})
		}
		anchors = append(anchors, anchor{id: highest, at: current.end})
	}
	added := make([]uint16, 0, len(pending))
	for id := range pending {
		added = append(added, id)
	}
	slices.Sort(added)
	for _, id := range added {
		// A new item follows the element with the highest id below it.
		at, best := -1, uint16(0)
		for _, candidate := range anchors {
			if candidate.id < id && candidate.id >= best {
				at, best = candidate.at, candidate.id
			}
		}
		text := f.element(id, id, pending[id])
		if at < 0 {
			at = f.firstSlot()
			pieces = append(pieces, piece{at: at, end: at, text: f.lineBefore(at, text), maxID: id})
			continue
		}
		pieces = append(pieces, piece{at: at, end: at, text: f.newline + f.indent + text, maxID: id})
	}
	slices.SortStableFunc(pieces, func(a, b piece) int {
		if a.at != b.at {
			return a.at - b.at
		}
		// What is inserted at the start of a replaced element is before it.
		if (a.end > a.at) != (b.end > b.at) {
			if a.end > a.at {
				return 1
			}
			return -1
		}
		return int(a.maxID) - int(b.maxID)
	})
	var out bytes.Buffer
	out.Grow(len(f.data) + 64*len(pieces))
	offset := 0
	for _, part := range pieces {
		out.Write(f.data[offset:part.at])
		out.WriteString(part.text)
		offset = part.end
	}
	out.Write(f.data[offset:])
	return out.Bytes()
}

// piece is a part of the file Encode replaces (at == end: inserts).
type piece struct {
	at, end int
	text    string
	maxID   uint16
}

type run struct {
	from, to uint16
	item     *Item
}

// rewrite returns the elements an entry becomes: runs of consecutive ids
// sharing their data. changed is false when the entry stays as it is. It
// takes the items it writes out of pending.
func (f *File) rewrite(current *entry, moves map[uint16]uint16, dropped map[uint16]bool, pending map[uint16]*Item) (runs []run, changed bool) {
	for id := uint32(current.from); id <= uint32(current.to); id++ {
		target, moved := moves[uint16(id)]
		if !moved {
			target = uint16(id)
		}
		_, edited := pending[target]
		if (moved && target != uint16(id)) || edited || dropped[uint16(id)] {
			changed = true
			break
		}
	}
	if !changed {
		return nil, false
	}
	for id := uint32(current.from); id <= uint32(current.to); id++ {
		target, moved := moves[uint16(id)]
		if !moved {
			if dropped[uint16(id)] {
				continue
			}
			target = uint16(id)
		}
		item := current.item
		if edited, ok := pending[target]; ok {
			item = edited
			delete(pending, target)
		}
		if last := len(runs) - 1; last >= 0 && runs[last].item == item && runs[last].to+1 == target {
			runs[last].to = target
			continue
		}
		runs = append(runs, run{from: target, to: target, item: item})
	}
	return runs, true
}

// removal replaces an element and the line it stands alone on with nothing.
func (f *File) removal(start, end int) piece {
	lineStart := bytes.LastIndexByte(f.data[:start], '\n') + 1
	lineEnd := end
	for lineEnd < len(f.data) && (f.data[lineEnd] == ' ' || f.data[lineEnd] == '\t' || f.data[lineEnd] == '\r') {
		lineEnd++
	}
	if len(bytes.TrimLeft(f.data[lineStart:start], " \t")) == 0 && lineEnd < len(f.data) && f.data[lineEnd] == '\n' {
		return piece{at: lineStart, end: lineEnd + 1}
	}
	return piece{at: start, end: end}
}

// firstSlot is where an item is inserted that follows no other: before the
// first item, or before </items> in a file without items, at the start of
// the line.
func (f *File) firstSlot() int {
	at := f.closing
	if len(f.entries) > 0 {
		at = f.entries[0].start
	}
	lineStart := bytes.LastIndexByte(f.data[:at], '\n') + 1
	if len(bytes.TrimLeft(f.data[lineStart:at], " \t")) == 0 {
		return lineStart
	}
	return at
}

// lineBefore puts an element on a line of its own before offset.
func (f *File) lineBefore(offset int, text string) string {
	if offset > 0 && f.data[offset-1] != '\n' {
		return f.newline + f.indent + text + f.newline
	}
	return f.indent + text + f.newline
}

func (f *File) element(from, to uint16, item *Item) string {
	var out strings.Builder
	out.WriteString("<item")
	if from == to {
		f.attr(&out, "id", strconv.Itoa(int(from)))
	} else {
		f.attr(&out, "fromid", strconv.Itoa(int(from)))
		f.attr(&out, "toid", strconv.Itoa(int(to)))
	}
	if item.Article != "" {
		f.attr(&out, "article", item.Article)
	}
	f.attr(&out, "name", item.Name)
	if item.Plural != "" {
		f.attr(&out, "plural", item.Plural)
	}
	for _, field := range item.Fields {
		if field.Key != "" {
			f.attr(&out, field.Key, field.Value)
		}
	}
	if len(item.Attributes) == 0 {
		out.WriteString(f.selfEnd)
		return out.String()
	}
	out.WriteString(">")
	unit := f.indent
	if strings.Contains(unit, "\t") || unit == "" {
		unit = "\t"
	}
	f.attributes(&out, item.Attributes, f.indent+unit, unit)
	out.WriteString(f.newline + f.indent + "</item>")
	return out.String()
}

func (f *File) attributes(out *strings.Builder, attributes []Attribute, indent, unit string) {
	for _, attribute := range attributes {
		out.WriteString(f.newline + indent + "<attribute")
		f.attr(out, "key", attribute.Key)
		f.attr(out, "value", attribute.Value)
		if len(attribute.Children) == 0 {
			out.WriteString(f.selfEnd)
			continue
		}
		out.WriteString(">")
		f.attributes(out, attribute.Children, indent+unit, unit)
		out.WriteString(f.newline + indent + "</attribute>")
	}
}

func (f *File) attr(out *strings.Builder, key, value string) {
	out.WriteString(" " + key + `="`)
	for _, char := range value {
		switch {
		case char == '&':
			out.WriteString("&amp;")
		case char == '<':
			out.WriteString("&lt;")
		case char == '>':
			out.WriteString("&gt;")
		case char == '"':
			out.WriteString("&quot;")
		case char < 0x20 && char != '\t':
			out.WriteString("&#" + strconv.Itoa(int(char)) + ";")
		case f.latin1 && char >= 0x80:
			if char < 0x100 {
				out.WriteByte(byte(char))
			} else {
				out.WriteString("&#" + strconv.Itoa(int(char)) + ";")
			}
		default:
			out.WriteRune(char)
		}
	}
	out.WriteString(`"`)
}
