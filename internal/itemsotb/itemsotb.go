// Package itemsotb reads, updates and writes items.otb, the server item
// list that maps server ids to client ids (OTServ, TFS and their forks).
// The file is kept as parsed: items keep unknown attributes and flag bits,
// and an unchanged file encodes to the same bytes. Only what the client
// stores is ever changed (see Item.Reload); server data is never touched.
//
// The format follows ItemEditor (github.com/ottools/ItemEditor): a node file
// (see otbnode) whose root holds the version and whose children are items.
// An item node's type is its group; its properties are a uint32 of flags
// followed by attributes (type byte, uint16 length, data).
package itemsotb

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/otbnode"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Group is the item group (node type).
type Group uint8

const (
	GroupNone       Group = 0
	GroupGround     Group = 1
	GroupContainer  Group = 2
	GroupSplash     Group = 11
	GroupFluid      Group = 12
	GroupDeprecated Group = 14
)

// Item flags (ItemEditor ServerItemFlag).
const (
	FlagUnpassable        = 1 << 0
	FlagBlockMissiles     = 1 << 1
	FlagBlockPathfinder   = 1 << 2
	FlagHasElevation      = 1 << 3
	FlagMultiUse          = 1 << 4
	FlagPickupable        = 1 << 5
	FlagMovable           = 1 << 6
	FlagStackable         = 1 << 7
	FlagStackOrder        = 1 << 13
	FlagReadable          = 1 << 14
	FlagRotatable         = 1 << 15
	FlagHangable          = 1 << 16
	FlagHookSouth         = 1 << 17
	FlagHookEast          = 1 << 18
	FlagAllowDistanceRead = 1 << 20
	FlagClientCharges     = 1 << 22
	FlagIgnoreLook        = 1 << 23
	FlagIsAnimation       = 1 << 24
	FlagFullGround        = 1 << 25
	FlagForceUse          = 1 << 26
)

// clientFlags are the flags Reload derives from the client. Other bits
// (floor change, can not decay, distance read...) are server data.
const clientFlags = FlagUnpassable | FlagBlockMissiles | FlagBlockPathfinder | FlagHasElevation |
	FlagMultiUse | FlagPickupable | FlagMovable | FlagStackable | FlagStackOrder | FlagReadable |
	FlagRotatable | FlagHangable | FlagHookSouth | FlagHookEast | FlagClientCharges | FlagIgnoreLook |
	FlagIsAnimation | FlagFullGround | FlagForceUse

// Item attributes.
const (
	AttrServerID          = 0x10
	AttrClientID          = 0x11
	AttrName              = 0x12
	AttrGroundSpeed       = 0x14
	AttrSpriteHash        = 0x20
	AttrMinimapColor      = 0x21
	AttrMaxReadWriteChars = 0x22
	AttrMaxReadChars      = 0x23
	AttrLight             = 0x2A
	AttrStackOrder        = 0x2B
	AttrTradeAs           = 0x2D
)

// attrOrder is the order ItemEditor writes attributes in. Reload writes
// these first, then any other attribute the item had.
var attrOrder = []byte{AttrServerID, AttrClientID, AttrSpriteHash, AttrMinimapColor, AttrMaxReadWriteChars,
	AttrMaxReadChars, AttrLight, AttrGroundSpeed, AttrStackOrder, AttrTradeAs, AttrName}

const rootAttrVersion = 0x01

// Attr is one item attribute.
type Attr struct {
	Type byte
	Data []byte
}

// Item is one server item.
type Item struct {
	Group Group
	Flags uint32
	Attrs []Attr
	// tail holds property bytes after the attributes that did not parse.
	tail []byte
}

// File is a parsed items.otb.
type File struct {
	ident     [4]byte
	rootType  byte
	rootFlags uint32
	// rootAttrs are the root attributes, the version among them.
	rootAttrs []Attr
	rootTail  []byte
	Items     []*Item
}

// New returns an empty items.otb with the given version, written like
// ItemEditor writes it.
func New(major, minor, build uint32) *File {
	v := binary.LittleEndian.AppendUint32(nil, major)
	v = binary.LittleEndian.AppendUint32(v, minor)
	v = binary.LittleEndian.AppendUint32(v, build)
	csd := make([]byte, 128)
	copy(csd, fmt.Sprintf("OTB %d.%d.%d", major, minor, build))
	return &File{rootAttrs: []Attr{{rootAttrVersion, append(v, csd...)}}}
}

// Parse reads an items.otb.
func Parse(data []byte) (*File, error) {
	root, ident, err := otbnode.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("items.otb: %w", err)
	}
	if len(root.Props) < 4 {
		return nil, fmt.Errorf("items.otb: root node without flags")
	}
	f := &File{ident: ident, rootType: root.Type, rootFlags: binary.LittleEndian.Uint32(root.Props)}
	f.rootAttrs, f.rootTail = readAttrs(root.Props[4:])
	for _, n := range root.Children {
		if len(n.Children) > 0 {
			return nil, fmt.Errorf("items.otb: item node has child nodes")
		}
		if len(n.Props) < 4 {
			return nil, fmt.Errorf("items.otb: item without flags")
		}
		it := &Item{Group: Group(n.Type), Flags: binary.LittleEndian.Uint32(n.Props)}
		it.Attrs, it.tail = readAttrs(n.Props[4:])
		f.Items = append(f.Items, it)
	}
	return f, nil
}

// readAttrs reads the attributes in b; bytes that do not form a whole
// attribute are returned as the tail, so they are written back unchanged.
func readAttrs(b []byte) ([]Attr, []byte) {
	var out []Attr
	for len(b) >= 3 {
		n := int(binary.LittleEndian.Uint16(b[1:]))
		if 3+n > len(b) {
			break
		}
		out = append(out, Attr{Type: b[0], Data: slices.Clone(b[3 : 3+n])})
		b = b[3+n:]
	}
	if len(b) > 0 {
		return out, slices.Clone(b)
	}
	return out, nil
}

// Encode writes the file.
func (f *File) Encode() ([]byte, error) {
	var buf bytes.Buffer
	w := otbnode.NewWriter(&buf, f.ident)
	w.Start(f.rootType)
	w.U32(f.rootFlags)
	writeAttrs(w, f.rootAttrs)
	w.Bytes(f.rootTail)
	for _, it := range f.Items {
		w.Start(byte(it.Group))
		w.U32(it.Flags)
		writeAttrs(w, it.Attrs)
		w.Bytes(it.tail)
		w.End()
	}
	w.End()
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAttrs(w *otbnode.Writer, attrs []Attr) {
	for _, a := range attrs {
		w.U8(a.Type)
		w.U16(uint16(len(a.Data)))
		w.Bytes(a.Data)
	}
}

// Version returns the major, minor (client) and build version.
func (f *File) Version() (major, minor, build uint32) {
	for _, a := range f.rootAttrs {
		if a.Type == rootAttrVersion && len(a.Data) >= 12 {
			return binary.LittleEndian.Uint32(a.Data), binary.LittleEndian.Uint32(a.Data[4:]), binary.LittleEndian.Uint32(a.Data[8:])
		}
	}
	return 0, 0, 0
}

// Attr returns the data of an attribute, or nil.
func (it *Item) Attr(t byte) []byte {
	for _, a := range it.Attrs {
		if a.Type == t {
			return a.Data
		}
	}
	return nil
}

func (it *Item) u16(t byte) uint16 {
	if d := it.Attr(t); len(d) >= 2 {
		return binary.LittleEndian.Uint16(d)
	}
	return 0
}

// ServerID returns the server id (0 when missing).
func (it *Item) ServerID() uint16 { return it.u16(AttrServerID) }

// ClientID returns the client id (0 when missing, as for deprecated items).
func (it *Item) ClientID() uint16 { return it.u16(AttrClientID) }

// Deprecated reports whether the item is a deprecated placeholder, which
// has no client id.
func (it *Item) Deprecated() bool { return it.Group == GroupDeprecated }

// SetClientID changes the client id.
func (it *Item) SetClientID(id uint16) { it.set(AttrClientID, u16(id)) }

func (it *Item) set(t byte, data []byte) {
	for i := range it.Attrs {
		if it.Attrs[i].Type == t {
			it.Attrs[i].Data = data
			return
		}
	}
	it.Attrs = append(it.Attrs, Attr{Type: t, Data: data})
}

func u16(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }

// NewItem returns an item for a new client item; Reload fills it.
func NewItem(serverID, clientID uint16) *Item {
	return &Item{Flags: FlagMovable, Attrs: []Attr{{AttrServerID, u16(serverID)}, {AttrClientID, u16(clientID)}}}
}

// Clone returns a deep copy.
func (it *Item) Clone() *Item {
	c := *it
	c.Attrs = make([]Attr, len(it.Attrs))
	for i, a := range it.Attrs {
		c.Attrs[i] = Attr{a.Type, slices.Clone(a.Data)}
	}
	c.tail = slices.Clone(it.tail)
	return &c
}

// Equal reports whether two items encode the same.
func (it *Item) Equal(o *Item) bool {
	return it.Group == o.Group && it.Flags == o.Flags && bytes.Equal(it.tail, o.tail) &&
		slices.EqualFunc(it.Attrs, o.Attrs, func(a, b Attr) bool { return a.Type == b.Type && bytes.Equal(a.Data, b.Data) })
}

// Reload sets what the client stores of the item from t, like ItemEditor's
// "reload item attributes": group, client flags, ground speed, stack order,
// light, minimap color, text lengths, market trade id and (when hash is not
// nil) the sprite hash. The name is only set when the client has one.
// Server ids, unknown attributes and server-only flags are kept.
func (it *Item) Reload(t *thing.Thing, hash *[16]byte) {
	p := &t.Props
	switch {
	case p.Ground:
		it.Group = GroupGround
	case p.Container:
		it.Group = GroupContainer
	case p.FluidContainer:
		it.Group = GroupFluid
	case p.Fluid:
		it.Group = GroupSplash
	default:
		// Groups of old item lists (door, key, teleport...) are server data.
		switch it.Group {
		case GroupGround, GroupContainer, GroupFluid, GroupSplash:
			it.Group = GroupNone
		}
	}

	var f uint32
	set := func(on bool, bit uint32) {
		if on {
			f |= bit
		}
	}
	set(p.Unpassable, FlagUnpassable)
	set(p.BlockMissile, FlagBlockMissiles)
	set(p.BlockPathfind, FlagBlockPathfinder)
	set(p.HasElevation, FlagHasElevation)
	set(p.MultiUse, FlagMultiUse)
	set(p.ForceUse, FlagForceUse)
	set(p.Pickupable, FlagPickupable)
	set(!p.Unmoveable, FlagMovable)
	set(p.Stackable, FlagStackable)
	set(p.Writable || p.WritableOnce || (p.LensHelp && p.LensHelpValue == 1112), FlagReadable)
	set(p.Rotatable, FlagRotatable)
	set(p.Hangable, FlagHangable)
	set(p.HookSouth, FlagHookSouth)
	set(p.HookEast, FlagHookEast)
	set(p.HasCharges, FlagClientCharges)
	set(p.IgnoreLook, FlagIgnoreLook)
	set(p.FullGround, FlagFullGround)
	set(len(t.FrameGroups) > 0 && t.FrameGroups[0].Frames > 1, FlagIsAnimation)
	var order byte
	switch {
	case p.GroundBorder:
		order = 1
	case p.OnBottom:
		order = 2
	case p.OnTop:
		order = 3
	}
	set(order != 0, FlagStackOrder)
	it.Flags = it.Flags&^clientFlags | f

	derived := map[byte][]byte{}
	put := func(on bool, t byte, data []byte) {
		if on {
			derived[t] = data
		} else {
			derived[t] = nil
		}
	}
	put(p.Ground, AttrGroundSpeed, u16(p.GroundSpeed))
	put(order != 0, AttrStackOrder, []byte{order})
	put(p.HasLight && (p.LightLevel != 0 || p.LightColor != 0), AttrLight, append(u16(p.LightLevel), u16(p.LightColor)...))
	put(p.MiniMap && p.MiniMapColor != 0, AttrMinimapColor, u16(p.MiniMapColor))
	put(p.Writable && p.MaxTextLength != 0, AttrMaxReadWriteChars, u16(p.MaxTextLength))
	put(p.WritableOnce && p.MaxReadLength != 0, AttrMaxReadChars, u16(p.MaxReadLength))
	put(p.IsMarket && p.Market.TradeAs != 0, AttrTradeAs, u16(p.Market.TradeAs))
	if t.Name != "" {
		derived[AttrName] = []byte(t.Name)
	}
	if hash != nil {
		derived[AttrSpriteHash] = slices.Clone(hash[:])
	}

	// Attributes in ItemEditor order, then the ones Reload does not know.
	var attrs []Attr
	for _, typ := range attrOrder {
		data, ok := derived[typ]
		if !ok {
			data = it.Attr(typ)
		}
		if data != nil {
			attrs = append(attrs, Attr{typ, data})
		}
	}
	for _, a := range it.Attrs {
		if !slices.Contains(attrOrder, a.Type) {
			attrs = append(attrs, a)
		}
	}
	it.Attrs = attrs
}

// SpriteSize is the only sprite size SpriteHash supports.
const SpriteSize = 32

// SpriteHash computes ItemEditor's sprite hash of an item: an MD5 over the
// sprites of its first frame and pattern, each as 32x32 BGR0 pixels, bottom
// row first, with transparent pixels in 0x111111. sprite returns the
// compressed spr data of an index into the frame group's sprites (nil for
// an empty sprite); transparent tells whether the data has alpha.
func SpriteHash(g *thing.FrameGroup, transparent bool, sprite func(i int) ([]byte, error)) ([16]byte, error) {
	h := md5.New()
	rgb := make([]byte, SpriteSize*SpriteSize*3)
	out := make([]byte, SpriteSize*SpriteSize*4)
	w, hh := int(g.Width), int(g.Height)
	for l := 0; l < int(g.Layers); l++ {
		for y := 0; y < hh; y++ {
			for x := 0; x < w; x++ {
				i := x + y*w + l*w*hh
				var data []byte
				if i < len(g.Sprites) && g.Sprites[i] != 0 {
					var err error
					if data, err = sprite(i); err != nil {
						return [16]byte{}, err
					}
				}
				decodeRGB(rgb, data, transparent)
				for py := 0; py < SpriteSize; py++ {
					src := (SpriteSize - py - 1) * SpriteSize * 3
					for px := 0; px < SpriteSize; px++ {
						o := SpriteSize*4*py + px*4
						s := src + px*3
						out[o], out[o+1], out[o+2], out[o+3] = rgb[s+2], rgb[s+1], rgb[s], 0
					}
				}
				h.Write(out)
			}
		}
	}
	var sum [16]byte
	h.Sum(sum[:0])
	return sum, nil
}

// decodeRGB decodes spr RLE data to 32x32 RGB the way ItemEditor does:
// skipped pixels and pixels past the data are 0x11, colored pixels keep
// their color whatever their alpha.
func decodeRGB(dst, data []byte, transparent bool) {
	for i := range dst {
		dst[i] = 0x11
	}
	channels := 3
	if transparent {
		channels = 4
	}
	n := SpriteSize * SpriteSize
	pos, pixel := 0, 0
	for pos+4 <= len(data) {
		pixel += int(binary.LittleEndian.Uint16(data[pos:]))
		colored := int(binary.LittleEndian.Uint16(data[pos+2:]))
		pos += 4
		for k := 0; k < colored && pixel < n && pos+channels <= len(data); k++ {
			copy(dst[pixel*3:pixel*3+3], data[pos:pos+3])
			pos += channels
			pixel++
		}
	}
}
