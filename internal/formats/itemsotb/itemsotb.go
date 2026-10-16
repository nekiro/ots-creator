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

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/otbnode"
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
	FlagFloorChangeDown   = 1 << 8
	FlagFloorChangeNorth  = 1 << 9
	FlagFloorChangeEast   = 1 << 10
	FlagFloorChangeSouth  = 1 << 11
	FlagFloorChangeWest   = 1 << 12
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

// FlagFloorChange is any of the floor change flags.
const FlagFloorChange = FlagFloorChangeDown | FlagFloorChangeNorth | FlagFloorChangeEast |
	FlagFloorChangeSouth | FlagFloorChangeWest

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

// versionDescriptionSize is the size of the zero padded text after the
// version numbers in the root version attribute (ItemEditor's CSDVersion).
const versionDescriptionSize = 128

// readableLensHelp is the lens help value of readable items (books, signs).
const readableLensHelp = 1112

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
	version := binary.LittleEndian.AppendUint32(nil, major)
	version = binary.LittleEndian.AppendUint32(version, minor)
	version = binary.LittleEndian.AppendUint32(version, build)
	description := make([]byte, versionDescriptionSize)
	copy(description, fmt.Sprintf("OTB %d.%d.%d", major, minor, build))
	return &File{rootAttrs: []Attr{{rootAttrVersion, append(version, description...)}}}
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
	file := &File{ident: ident, rootType: root.Type, rootFlags: binary.LittleEndian.Uint32(root.Props)}
	file.rootAttrs, file.rootTail = readAttrs(root.Props[4:])
	for _, node := range root.Children {
		if len(node.Children) > 0 {
			return nil, fmt.Errorf("items.otb: item node has child nodes")
		}
		if len(node.Props) < 4 {
			return nil, fmt.Errorf("items.otb: item without flags")
		}
		item := &Item{Group: Group(node.Type), Flags: binary.LittleEndian.Uint32(node.Props)}
		item.Attrs, item.tail = readAttrs(node.Props[4:])
		file.Items = append(file.Items, item)
	}
	return file, nil
}

// readAttrs reads the attributes in props; bytes that do not form a whole
// attribute are returned as the tail, so they are written back unchanged.
func readAttrs(props []byte) ([]Attr, []byte) {
	var out []Attr
	for len(props) >= 3 {
		length := int(binary.LittleEndian.Uint16(props[1:]))
		if 3+length > len(props) {
			break
		}
		out = append(out, Attr{Type: props[0], Data: slices.Clone(props[3 : 3+length])})
		props = props[3+length:]
	}
	if len(props) > 0 {
		return out, slices.Clone(props)
	}
	return out, nil
}

// Encode writes the file.
func (f *File) Encode() ([]byte, error) {
	var buf bytes.Buffer
	writer := otbnode.NewWriter(&buf, f.ident)
	writer.Start(f.rootType)
	writer.U32(f.rootFlags)
	writeAttrs(writer, f.rootAttrs)
	writer.Bytes(f.rootTail)
	for _, item := range f.Items {
		writer.Start(byte(item.Group))
		writer.U32(item.Flags)
		writeAttrs(writer, item.Attrs)
		writer.Bytes(item.tail)
		writer.End()
	}
	writer.End()
	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAttrs(writer *otbnode.Writer, attrs []Attr) {
	for _, attr := range attrs {
		writer.U8(attr.Type)
		writer.U16(uint16(len(attr.Data)))
		writer.Bytes(attr.Data)
	}
}

// Version returns the major, minor (client) and build version.
func (f *File) Version() (major, minor, build uint32) {
	for _, attr := range f.rootAttrs {
		if attr.Type == rootAttrVersion && len(attr.Data) >= 12 {
			return binary.LittleEndian.Uint32(attr.Data), binary.LittleEndian.Uint32(attr.Data[4:]), binary.LittleEndian.Uint32(attr.Data[8:])
		}
	}
	return 0, 0, 0
}

// Attr returns the data of an attribute, or nil.
func (it *Item) Attr(attrType byte) []byte {
	for _, attr := range it.Attrs {
		if attr.Type == attrType {
			return attr.Data
		}
	}
	return nil
}

func (it *Item) u16(attrType byte) uint16 {
	if data := it.Attr(attrType); len(data) >= 2 {
		return binary.LittleEndian.Uint16(data)
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

func (it *Item) set(attrType byte, data []byte) {
	for i := range it.Attrs {
		if it.Attrs[i].Type == attrType {
			it.Attrs[i].Data = data
			return
		}
	}
	it.Attrs = append(it.Attrs, Attr{Type: attrType, Data: data})
}

func u16(value uint16) []byte { return binary.LittleEndian.AppendUint16(nil, value) }

// NewItem returns an item for a new client item; Reload fills it.
func NewItem(serverID, clientID uint16) *Item {
	return &Item{Flags: FlagMovable, Attrs: []Attr{{AttrServerID, u16(serverID)}, {AttrClientID, u16(clientID)}}}
}

// Clone returns a deep copy.
func (it *Item) Clone() *Item {
	clone := *it
	clone.Attrs = make([]Attr, len(it.Attrs))
	for i, attr := range it.Attrs {
		clone.Attrs[i] = Attr{attr.Type, slices.Clone(attr.Data)}
	}
	clone.tail = slices.Clone(it.tail)
	return &clone
}

// Equal reports whether two items encode the same.
func (it *Item) Equal(other *Item) bool {
	return it.Group == other.Group && it.Flags == other.Flags && bytes.Equal(it.tail, other.tail) &&
		slices.EqualFunc(it.Attrs, other.Attrs, func(left, right Attr) bool {
			return left.Type == right.Type && bytes.Equal(left.Data, right.Data)
		})
}

// Reload sets what the client stores of the item from client, like
// ItemEditor's "reload item attributes": group, client flags, ground speed,
// stack order, light, minimap color, text lengths, market trade id and
// (when hash is not nil) the sprite hash. The name is only set when the
// client has one. Server ids, unknown attributes and server-only flags are
// kept.
func (it *Item) Reload(client *thing.Thing, hash *[16]byte) {
	it.reloadGroup(&client.Props)
	order := stackOrder(&client.Props)
	it.Flags = it.Flags&^clientFlags | flagsOf(client, order)
	it.mergeAttrs(derivedAttrs(client, order, hash))
}

// reloadGroup sets the group the client knows of; groups of old item lists
// (door, key, teleport...) are server data and are kept.
func (it *Item) reloadGroup(props *thing.Properties) {
	switch {
	case props.Ground:
		it.Group = GroupGround
	case props.Container:
		it.Group = GroupContainer
	case props.FluidContainer:
		it.Group = GroupFluid
	case props.Fluid:
		it.Group = GroupSplash
	default:
		switch it.Group {
		case GroupGround, GroupContainer, GroupFluid, GroupSplash:
			it.Group = GroupNone
		}
	}
}

// stackOrder returns the items.otb stack order: 1 border, 2 bottom, 3 top
// and 0 for none.
func stackOrder(props *thing.Properties) byte {
	switch {
	case props.GroundBorder:
		return 1
	case props.OnBottom:
		return 2
	case props.OnTop:
		return 3
	}
	return 0
}

// flagsOf returns the client flags of an item with the given stack order.
func flagsOf(client *thing.Thing, order byte) uint32 {
	props := &client.Props
	var flags uint32
	set := func(on bool, bit uint32) {
		if on {
			flags |= bit
		}
	}
	set(props.Unpassable, FlagUnpassable)
	set(props.BlockMissile, FlagBlockMissiles)
	set(props.BlockPathfind, FlagBlockPathfinder)
	set(props.HasElevation, FlagHasElevation)
	set(props.MultiUse, FlagMultiUse)
	set(props.ForceUse, FlagForceUse)
	set(props.Pickupable, FlagPickupable)
	set(!props.Unmoveable, FlagMovable)
	set(props.Stackable, FlagStackable)
	set(props.Writable || props.WritableOnce || (props.LensHelp && props.LensHelpValue == readableLensHelp), FlagReadable)
	set(props.Rotatable, FlagRotatable)
	set(props.Hangable, FlagHangable)
	set(props.HookSouth, FlagHookSouth)
	set(props.HookEast, FlagHookEast)
	set(props.HasCharges, FlagClientCharges)
	set(props.IgnoreLook, FlagIgnoreLook)
	set(props.FullGround, FlagFullGround)
	set(len(client.FrameGroups) > 0 && client.FrameGroups[0].Frames > 1, FlagIsAnimation)
	set(order != 0, FlagStackOrder)
	return flags
}

// derivedAttrs returns the attributes Reload sets from the client; a nil
// value removes the attribute.
func derivedAttrs(client *thing.Thing, order byte, hash *[16]byte) map[byte][]byte {
	props := &client.Props
	derived := map[byte][]byte{}
	put := func(on bool, attrType byte, data []byte) {
		if on {
			derived[attrType] = data
		} else {
			derived[attrType] = nil
		}
	}
	put(props.Ground, AttrGroundSpeed, u16(props.GroundSpeed))
	put(order != 0, AttrStackOrder, []byte{order})
	put(props.HasLight && (props.LightLevel != 0 || props.LightColor != 0), AttrLight, append(u16(props.LightLevel), u16(props.LightColor)...))
	put(props.MiniMap && props.MiniMapColor != 0, AttrMinimapColor, u16(props.MiniMapColor))
	put(props.Writable && props.MaxTextLength != 0, AttrMaxReadWriteChars, u16(props.MaxTextLength))
	put(props.WritableOnce && props.MaxReadLength != 0, AttrMaxReadChars, u16(props.MaxReadLength))
	put(props.IsMarket && props.Market.TradeAs != 0, AttrTradeAs, u16(props.Market.TradeAs))
	if client.Name != "" {
		derived[AttrName] = []byte(client.Name)
	}
	if hash != nil {
		derived[AttrSpriteHash] = slices.Clone(hash[:])
	}
	return derived
}

// mergeAttrs replaces the attributes with derived ones, in ItemEditor order,
// followed by the attributes Reload does not know.
func (it *Item) mergeAttrs(derived map[byte][]byte) {
	var attrs []Attr
	for _, attrType := range attrOrder {
		data, ok := derived[attrType]
		if !ok {
			data = it.Attr(attrType)
		}
		if data != nil {
			attrs = append(attrs, Attr{attrType, data})
		}
	}
	for _, attr := range it.Attrs {
		if !slices.Contains(attrOrder, attr.Type) {
			attrs = append(attrs, attr)
		}
	}
	it.Attrs = attrs
}

// SpriteSize is the only sprite size SpriteHash supports.
const SpriteSize = 32

// transparentChannel is the value of each color channel of a transparent
// pixel in the sprite hash.
const transparentChannel = 0x11

// SpriteHash computes ItemEditor's sprite hash of an item: an MD5 over the
// sprites of its first frame and pattern, each as 32x32 BGR0 pixels, bottom
// row first, with transparent pixels in 0x111111. sprite returns the
// compressed spr data of an index into the frame group's sprites (nil for
// an empty sprite); transparent tells whether the data has alpha.
func SpriteHash(group *thing.FrameGroup, transparent bool, sprite func(i int) ([]byte, error)) ([16]byte, error) {
	hasher := md5.New()
	rgb := make([]byte, SpriteSize*SpriteSize*3)
	hashed := make([]byte, SpriteSize*SpriteSize*4)
	width, height := int(group.Width), int(group.Height)
	for layer := 0; layer < int(group.Layers); layer++ {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				spriteIndex := x + y*width + layer*width*height
				var data []byte
				if spriteIndex < len(group.Sprites) && group.Sprites[spriteIndex] != 0 {
					var err error
					if data, err = sprite(spriteIndex); err != nil {
						return [16]byte{}, err
					}
				}
				decodeRGB(rgb, data, transparent)
				for row := 0; row < SpriteSize; row++ {
					srcRow := (SpriteSize - row - 1) * SpriteSize * 3
					for column := 0; column < SpriteSize; column++ {
						dst := SpriteSize*4*row + column*4
						src := srcRow + column*3
						hashed[dst], hashed[dst+1], hashed[dst+2], hashed[dst+3] = rgb[src+2], rgb[src+1], rgb[src], 0
					}
				}
				hasher.Write(hashed)
			}
		}
	}
	var sum [16]byte
	hasher.Sum(sum[:0])
	return sum, nil
}

// decodeRGB decodes spr RLE data to 32x32 RGB the way ItemEditor does:
// skipped pixels and pixels past the data are 0x11, colored pixels keep
// their color whatever their alpha.
func decodeRGB(dst, data []byte, transparent bool) {
	for i := range dst {
		dst[i] = transparentChannel
	}
	channels := 3
	if transparent {
		channels = 4
	}
	pixelCount := SpriteSize * SpriteSize
	pos, pixel := 0, 0
	for pos+4 <= len(data) {
		pixel += int(binary.LittleEndian.Uint16(data[pos:]))
		colored := int(binary.LittleEndian.Uint16(data[pos+2:]))
		pos += 4
		for i := 0; i < colored && pixel < pixelCount && pos+channels <= len(data); i++ {
			copy(dst[pixel*3:pixel*3+3], data[pos:pos+3])
			pos += channels
			pixel++
		}
	}
}
