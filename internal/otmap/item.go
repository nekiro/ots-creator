package otmap

import (
	"slices"
)

// Item is one item on the map. Subtype is the count, charges or fluid type.
type Item struct {
	ID       uint16   `json:"id"`
	Subtype  uint16   `json:"subtype"`
	Selected bool     `json:"selected"`
	Ext      *ItemExt `json:"ext,omitempty"`
}

// ItemExt holds attributes few items have.
type ItemExt struct {
	ActionID    uint16    `json:"actionId,omitempty"`
	UniqueID    uint16    `json:"uniqueId,omitempty"`
	Text        string    `json:"text,omitempty"`
	Description string    `json:"description,omitempty"`
	Dest        *Position `json:"dest,omitempty"`
	DepotID     uint16    `json:"depotId,omitempty"`
	DoorID      uint8     `json:"doorId,omitempty"`
	// Items are the contents of a container.
	Items []Item `json:"items,omitempty"`
	// Attrs is the OTBM attribute map (custom attributes).
	Attrs []Attr `json:"attrs,omitempty"`
	// Raw keeps attributes the editor does not edit (written date, duration,
	// decay state...) to write them back unchanged.
	Raw []RawAttr `json:"raw,omitempty"`
}

// Attribute value types of the OTBM attribute map.
const (
	AttrString  uint8 = 1
	AttrInteger uint8 = 2
	AttrFloat   uint8 = 3
	AttrBoolean uint8 = 4
	AttrDouble  uint8 = 5
)

// Attr is one custom attribute.
type Attr struct {
	Key   string  `json:"key"`
	Type  uint8   `json:"type"`
	Str   string  `json:"str,omitempty"`
	Num   float64 `json:"num,omitempty"`
	Int   int32   `json:"int,omitempty"`
	Value bool    `json:"value,omitempty"`
}

// RawAttr is an OTBM item attribute kept as bytes.
type RawAttr struct {
	Code byte   `json:"code"`
	Data []byte `json:"data"`
}

func (e *ItemExt) empty() bool {
	return e == nil || (e.ActionID == 0 && e.UniqueID == 0 && e.Text == "" && e.Description == "" && e.Dest == nil &&
		e.DepotID == 0 && e.DoorID == 0 && len(e.Items) == 0 && len(e.Attrs) == 0 && len(e.Raw) == 0)
}

// Ensure returns the item's ext, creating it.
func (it *Item) Ensure() *ItemExt {
	if it.Ext == nil {
		it.Ext = &ItemExt{}
	}
	return it.Ext
}

// Compact drops an empty ext.
func (it *Item) Compact() {
	if it.Ext.empty() {
		it.Ext = nil
	}
}

// ActionID returns the action id.
func (it *Item) ActionID() uint16 {
	if it.Ext == nil {
		return 0
	}
	return it.Ext.ActionID
}

// UniqueID returns the unique id.
func (it *Item) UniqueID() uint16 {
	if it.Ext == nil {
		return 0
	}
	return it.Ext.UniqueID
}

// Complex reports whether the item has attributes besides its id and
// subtype (the eraser can keep those).
func (it *Item) Complex() bool { return !it.Ext.empty() }

// Clone returns a deep copy.
func (it Item) Clone() Item {
	if it.Ext == nil {
		return it
	}
	ext := *it.Ext
	if ext.Dest != nil {
		dest := *ext.Dest
		ext.Dest = &dest
	}
	if ext.Items != nil {
		ext.Items = cloneItems(ext.Items)
	}
	ext.Attrs = slices.Clone(ext.Attrs)
	if ext.Raw != nil {
		raw := make([]RawAttr, len(ext.Raw))
		for i, attr := range ext.Raw {
			raw[i] = RawAttr{Code: attr.Code, Data: slices.Clone(attr.Data)}
		}
		ext.Raw = raw
	}
	it.Ext = &ext
	return it
}

func cloneItems(items []Item) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, len(items))
	for i, item := range items {
		out[i] = item.Clone()
	}
	return out
}
