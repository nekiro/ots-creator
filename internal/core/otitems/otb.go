package otitems

import (
	"encoding/binary"

	"github.com/nekiro/ots-creator/internal/formats/itemsotb"
)

// ParseOTB reads an items.otb.
func ParseOTB(data []byte) (*DB, error) {
	file, err := itemsotb.Parse(data)
	if err != nil {
		return nil, err
	}
	return FromOTB(file), nil
}

// FromOTB builds a database from a parsed items.otb. Deprecated items and
// items without a server id are left out.
func FromOTB(file *itemsotb.File) *DB {
	db := &DB{Mode: ModeOTB}
	db.MajorVersion, db.MinorVersion, db.BuildNumber = file.Version()
	for _, item := range file.Items {
		if item.Deprecated() || item.ServerID() == 0 {
			continue
		}
		db.set(typeFromOTB(item))
	}
	db.index()
	return db
}

func typeFromOTB(item *itemsotb.Item) *Type {
	group := Group(item.Group)
	itemType := &Type{
		ID:           item.ServerID(),
		ClientID:     item.ClientID(),
		Group:        group,
		Flags:        flagsFromOTB(item.Flags, group),
		Name:         string(item.Attr(itemsotb.AttrName)),
		Speed:        attrU16(item, itemsotb.AttrGroundSpeed),
		MinimapColor: attrU16(item, itemsotb.AttrMinimapColor),
	}
	if light := item.Attr(itemsotb.AttrLight); len(light) == 4 {
		itemType.LightLevel = uint8(binary.LittleEndian.Uint16(light))
		itemType.LightColor = uint8(binary.LittleEndian.Uint16(light[2:]))
	}
	// The top order only counts for items flagged always on top; such an
	// item without the attribute is a border.
	if item.Flags&itemsotb.FlagStackOrder != 0 {
		itemType.TopOrder = 1
		if order := item.Attr(itemsotb.AttrStackOrder); len(order) == 1 {
			itemType.TopOrder = order[0]
		}
	}
	return itemType
}

// attrU16 returns a two byte attribute, or 0 when it is missing or has
// another size.
func attrU16(item *itemsotb.Item, attr byte) uint16 {
	if data := item.Attr(attr); len(data) == 2 {
		return binary.LittleEndian.Uint16(data)
	}
	return 0
}

var otbFlagMap = []struct {
	otbFlag uint32
	flag    Flags
}{
	{itemsotb.FlagUnpassable, FlagUnpassable},
	{itemsotb.FlagBlockMissiles, FlagBlockMissile},
	{itemsotb.FlagBlockPathfinder, FlagBlockPath},
	{itemsotb.FlagHasElevation, FlagElevation},
	{itemsotb.FlagPickupable, FlagPickupable},
	{itemsotb.FlagMovable, FlagMoveable},
	{itemsotb.FlagStackable, FlagStackable},
	{itemsotb.FlagReadable, FlagReadable},
	{itemsotb.FlagRotatable, FlagRotatable},
	{itemsotb.FlagHangable, FlagHangable},
	{itemsotb.FlagHookSouth, FlagHookSouth},
	{itemsotb.FlagHookEast, FlagHookEast},
	{itemsotb.FlagClientCharges, FlagCharges},
	{itemsotb.FlagIgnoreLook, FlagIgnoreLook},
}

var groupFlags = map[Group]Flags{
	GroupGround:    FlagGround,
	GroupContainer: FlagContainer,
	GroupSplash:    FlagSplash,
	GroupFluid:     FlagFluidContainer,
	GroupWritable:  FlagWritable | FlagReadable,
	GroupCharges:   FlagCharges,
}

func flagsFromOTB(otbFlags uint32, group Group) Flags {
	flags := groupFlags[group]
	for _, pair := range otbFlagMap {
		if otbFlags&pair.otbFlag != 0 {
			flags |= pair.flag
		}
	}
	if otbFlags&itemsotb.FlagFloorChange != 0 {
		flags |= FlagFloorChange
	}
	return flags
}
