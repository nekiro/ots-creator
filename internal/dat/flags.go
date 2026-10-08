package dat

import (
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/thing"
)

// attr is a version-independent flag identifier.
type attr uint8

const (
	aGround attr = iota + 1
	aGroundBorder
	aOnBottom
	aOnTop
	aContainer
	aStackable
	aForceUse
	aMultiUse
	aHasCharges
	aWritable
	aWritableOnce
	aFluidContainer
	aFluid
	aUnpassable
	aUnmoveable
	aBlockMissile
	aBlockPathfind
	aNoMoveAnimation
	aPickupable
	aHangable
	aHookSouth
	aHookEast
	aRotatable
	aLight
	aDontHide
	aTranslucent
	aFloorChange
	aOffset
	aElevation
	aLyingObject
	aAnimateAlways
	aMiniMap
	aLensHelp
	aFullGround
	aIgnoreLook
	aCloth
	aMarket
	aDefaultAction
	aWrappable
	aUnwrappable
	aTopEffect
	aUsable
	aBones
)

// LastFlag terminates a flag list.
const LastFlag = 0xFF

// FlagTable maps flag bytes of one format to attributes.
type FlagTable struct {
	name string
	// offsetHasPayload is false for 7.10-7.50 where the offset flag means (8,8).
	offsetHasPayload bool
	byByte           map[byte]attr
	byAttr           map[attr]byte
	order            []byte // flag bytes in ascending order, used for writing
}

func newTable(name string, offsetPayload bool, m map[byte]attr) *FlagTable {
	t := &FlagTable{name: name, offsetHasPayload: offsetPayload, byByte: m, byAttr: map[attr]byte{}}
	for b, a := range m {
		t.byAttr[a] = b
		t.order = append(t.order, b)
	}
	slices.Sort(t.order)
	return t
}

var table1 = newTable("7.10-7.30", false, map[byte]attr{
	0x00: aGround, 0x01: aOnBottom, 0x02: aOnTop, 0x03: aContainer, 0x04: aStackable,
	0x05: aMultiUse, 0x06: aForceUse, 0x07: aWritable, 0x08: aWritableOnce,
	0x09: aFluidContainer, 0x0A: aFluid, 0x0B: aUnpassable, 0x0C: aUnmoveable,
	0x0D: aBlockMissile, 0x0E: aBlockPathfind, 0x0F: aPickupable, 0x10: aLight,
	0x11: aFloorChange, 0x12: aFullGround, 0x13: aElevation, 0x14: aOffset,
	0x16: aMiniMap, 0x17: aRotatable, 0x18: aLyingObject, 0x19: aAnimateAlways,
	0x1A: aLensHelp, 0x24: aWrappable, 0x25: aUnwrappable, 0x26: aTopEffect,
})

var table2 = newTable("7.40-7.50", false, map[byte]attr{
	0x00: aGround, 0x01: aOnBottom, 0x02: aOnTop, 0x03: aContainer, 0x04: aStackable,
	0x05: aMultiUse, 0x06: aForceUse, 0x07: aWritable, 0x08: aWritableOnce,
	0x09: aFluidContainer, 0x0A: aFluid, 0x0B: aUnpassable, 0x0C: aUnmoveable,
	0x0D: aBlockMissile, 0x0E: aBlockPathfind, 0x0F: aPickupable, 0x10: aLight,
	0x11: aFloorChange, 0x12: aFullGround, 0x13: aElevation, 0x14: aOffset,
	0x16: aMiniMap, 0x17: aRotatable, 0x18: aLyingObject, 0x19: aHangable,
	0x1A: aHookSouth, 0x1B: aHookEast, 0x1C: aAnimateAlways, 0x1D: aLensHelp,
	0x24: aWrappable, 0x25: aUnwrappable, 0x26: aTopEffect,
})

var table3 = newTable("7.55-7.72", true, map[byte]attr{
	0x00: aGround, 0x01: aGroundBorder, 0x02: aOnBottom, 0x03: aOnTop, 0x04: aContainer,
	0x05: aStackable, 0x06: aForceUse, 0x07: aMultiUse, 0x08: aWritable, 0x09: aWritableOnce,
	0x0A: aFluidContainer, 0x0B: aFluid, 0x0C: aUnpassable, 0x0D: aUnmoveable,
	0x0E: aBlockMissile, 0x0F: aBlockPathfind, 0x10: aPickupable, 0x11: aHangable,
	0x12: aHookSouth, 0x13: aHookEast, 0x14: aRotatable, 0x15: aLight, 0x17: aFloorChange,
	0x18: aOffset, 0x19: aElevation, 0x1A: aLyingObject, 0x1B: aAnimateAlways,
	0x1C: aMiniMap, 0x1D: aLensHelp, 0x1E: aFullGround,
})

var table4 = newTable("7.80-8.54", true, map[byte]attr{
	0x00: aGround, 0x01: aGroundBorder, 0x02: aOnBottom, 0x03: aOnTop, 0x04: aContainer,
	0x05: aStackable, 0x06: aForceUse, 0x07: aMultiUse, 0x08: aHasCharges, 0x09: aWritable,
	0x0A: aWritableOnce, 0x0B: aFluidContainer, 0x0C: aFluid, 0x0D: aUnpassable,
	0x0E: aUnmoveable, 0x0F: aBlockMissile, 0x10: aBlockPathfind, 0x11: aPickupable,
	0x12: aHangable, 0x13: aHookSouth, 0x14: aHookEast, 0x15: aRotatable, 0x16: aLight,
	0x17: aDontHide, 0x18: aFloorChange, 0x19: aOffset, 0x1A: aElevation, 0x1B: aLyingObject,
	0x1C: aAnimateAlways, 0x1D: aMiniMap, 0x1E: aLensHelp, 0x1F: aFullGround, 0x20: aIgnoreLook,
	0x24: aWrappable, 0x25: aUnwrappable, 0x27: aBones,
})

var table5 = newTable("8.60-9.86", true, map[byte]attr{
	0x00: aGround, 0x01: aGroundBorder, 0x02: aOnBottom, 0x03: aOnTop, 0x04: aContainer,
	0x05: aStackable, 0x06: aForceUse, 0x07: aMultiUse, 0x08: aWritable, 0x09: aWritableOnce,
	0x0A: aFluidContainer, 0x0B: aFluid, 0x0C: aUnpassable, 0x0D: aUnmoveable,
	0x0E: aBlockMissile, 0x0F: aBlockPathfind, 0x10: aPickupable, 0x11: aHangable,
	0x12: aHookSouth, 0x13: aHookEast, 0x14: aRotatable, 0x15: aLight, 0x16: aDontHide,
	0x17: aTranslucent, 0x18: aOffset, 0x19: aElevation, 0x1A: aLyingObject,
	0x1B: aAnimateAlways, 0x1C: aMiniMap, 0x1D: aLensHelp, 0x1E: aFullGround,
	0x1F: aIgnoreLook, 0x20: aCloth, 0x21: aMarket, 0x27: aBones,
})

var table6 = newTable("10.10+", true, map[byte]attr{
	0x00: aGround, 0x01: aGroundBorder, 0x02: aOnBottom, 0x03: aOnTop, 0x04: aContainer,
	0x05: aStackable, 0x06: aForceUse, 0x07: aMultiUse, 0x08: aWritable, 0x09: aWritableOnce,
	0x0A: aFluidContainer, 0x0B: aFluid, 0x0C: aUnpassable, 0x0D: aUnmoveable,
	0x0E: aBlockMissile, 0x0F: aBlockPathfind, 0x10: aNoMoveAnimation, 0x11: aPickupable,
	0x12: aHangable, 0x13: aHookSouth, 0x14: aHookEast, 0x15: aRotatable, 0x16: aLight,
	0x17: aDontHide, 0x18: aTranslucent, 0x19: aOffset, 0x1A: aElevation, 0x1B: aLyingObject,
	0x1C: aAnimateAlways, 0x1D: aMiniMap, 0x1E: aLensHelp, 0x1F: aFullGround, 0x20: aIgnoreLook,
	0x21: aCloth, 0x22: aMarket, 0x23: aDefaultAction, 0x24: aWrappable, 0x25: aUnwrappable,
	0x26: aTopEffect, 0x27: aBones, 0xFE: aUsable,
})

// OBDTable is the flag table of OBD v2/v3 files. It is the 10.10+ table
// plus three private flags, without bones.
var OBDTable = newTable("obd", true, map[byte]attr{
	0x00: aGround, 0x01: aGroundBorder, 0x02: aOnBottom, 0x03: aOnTop, 0x04: aContainer,
	0x05: aStackable, 0x06: aForceUse, 0x07: aMultiUse, 0x08: aWritable, 0x09: aWritableOnce,
	0x0A: aFluidContainer, 0x0B: aFluid, 0x0C: aUnpassable, 0x0D: aUnmoveable,
	0x0E: aBlockMissile, 0x0F: aBlockPathfind, 0x10: aNoMoveAnimation, 0x11: aPickupable,
	0x12: aHangable, 0x13: aHookSouth, 0x14: aHookEast, 0x15: aRotatable, 0x16: aLight,
	0x17: aDontHide, 0x18: aTranslucent, 0x19: aOffset, 0x1A: aElevation, 0x1B: aLyingObject,
	0x1C: aAnimateAlways, 0x1D: aMiniMap, 0x1E: aLensHelp, 0x1F: aFullGround, 0x20: aIgnoreLook,
	0x21: aCloth, 0x22: aMarket, 0x23: aDefaultAction, 0x24: aWrappable, 0x25: aUnwrappable,
	0x26: aTopEffect, 0xFC: aHasCharges, 0xFD: aFloorChange, 0xFE: aUsable,
})

// Table returns the flag table for a metadata format (1..6, see
// client.MetadataFormat).
func Table(format int) *FlagTable {
	switch format {
	case 1:
		return table1
	case 2:
		return table2
	case 3:
		return table3
	case 4:
		return table4
	case 5:
		return table5
	}
	return table6
}

// Name returns a human readable name of the table.
func (t *FlagTable) Name() string { return t.name }

// ReadProperties reads flags until LastFlag into p.
func (t *FlagTable) ReadProperties(r *binio.Reader, p *thing.Properties) error {
	prev := -1
	for {
		b := r.U8()
		if r.Err() != nil {
			return r.Err()
		}
		if b == LastFlag {
			return nil
		}
		a, ok := t.byByte[b]
		if !ok {
			return fmt.Errorf("unknown flag 0x%02X (previous 0x%02X) for format %s at offset %d", b, prev, t.name, r.Pos()-1)
		}
		t.readAttr(r, a, p)
		prev = int(b)
	}
}

func (t *FlagTable) readAttr(r *binio.Reader, a attr, p *thing.Properties) {
	switch a {
	case aGround:
		p.Ground, p.GroundSpeed = true, r.U16()
	case aGroundBorder:
		p.GroundBorder = true
	case aOnBottom:
		p.OnBottom = true
	case aOnTop:
		p.OnTop = true
	case aContainer:
		p.Container = true
	case aStackable:
		p.Stackable = true
	case aForceUse:
		p.ForceUse = true
	case aMultiUse:
		p.MultiUse = true
	case aHasCharges:
		p.HasCharges = true
	case aWritable:
		p.Writable, p.MaxTextLength = true, r.U16()
	case aWritableOnce:
		p.WritableOnce, p.MaxReadLength = true, r.U16()
	case aFluidContainer:
		p.FluidContainer = true
	case aFluid:
		p.Fluid = true
	case aUnpassable:
		p.Unpassable = true
	case aUnmoveable:
		p.Unmoveable = true
	case aBlockMissile:
		p.BlockMissile = true
	case aBlockPathfind:
		p.BlockPathfind = true
	case aNoMoveAnimation:
		p.NoMoveAnimation = true
	case aPickupable:
		p.Pickupable = true
	case aHangable:
		p.Hangable = true
	case aHookSouth:
		p.HookSouth = true
	case aHookEast:
		p.HookEast = true
	case aRotatable:
		p.Rotatable = true
	case aLight:
		p.HasLight = true
		p.LightLevel = r.U16()
		p.LightColor = r.U16()
	case aDontHide:
		p.DontHide = true
	case aTranslucent:
		p.Translucent = true
	case aFloorChange:
		p.FloorChange = true
	case aOffset:
		p.HasOffset = true
		if t.offsetHasPayload {
			p.OffsetX = r.I16()
			p.OffsetY = r.I16()
		} else {
			p.OffsetX, p.OffsetY = 8, 8
		}
	case aElevation:
		p.HasElevation, p.Elevation = true, r.U16()
	case aLyingObject:
		p.LyingObject = true
	case aAnimateAlways:
		p.AnimateAlways = true
	case aMiniMap:
		p.MiniMap, p.MiniMapColor = true, r.U16()
	case aLensHelp:
		p.LensHelp, p.LensHelpValue = true, r.U16()
	case aFullGround:
		p.FullGround = true
	case aIgnoreLook:
		p.IgnoreLook = true
	case aCloth:
		p.Cloth, p.ClothSlot = true, r.U16()
	case aMarket:
		p.IsMarket = true
		p.Market.Category = r.U16()
		p.Market.TradeAs = r.U16()
		p.Market.ShowAs = r.U16()
		p.Market.Name = r.Latin1(int(r.U16()))
		p.Market.RestrictProfession = r.U16()
		p.Market.RestrictLevel = r.U16()
	case aDefaultAction:
		p.HasDefaultAction, p.DefaultAction = true, r.U16()
	case aWrappable:
		p.Wrappable = true
	case aUnwrappable:
		p.Unwrappable = true
	case aTopEffect:
		p.TopEffect = true
	case aUsable:
		p.Usable = true
	case aBones:
		p.HasBones = true
		// File order is north, south, east, west.
		for _, d := range []thing.Direction{thing.North, thing.South, thing.East, thing.West} {
			p.Bones[d].X = r.I16()
			p.Bones[d].Y = r.I16()
		}
	}
}

// has reports whether the attribute is set in p.
func has(a attr, p *thing.Properties) bool {
	switch a {
	case aGround:
		return p.Ground
	case aGroundBorder:
		return p.GroundBorder
	case aOnBottom:
		return p.OnBottom
	case aOnTop:
		return p.OnTop
	case aContainer:
		return p.Container
	case aStackable:
		return p.Stackable
	case aForceUse:
		return p.ForceUse
	case aMultiUse:
		return p.MultiUse
	case aHasCharges:
		return p.HasCharges
	case aWritable:
		return p.Writable
	case aWritableOnce:
		return p.WritableOnce
	case aFluidContainer:
		return p.FluidContainer
	case aFluid:
		return p.Fluid
	case aUnpassable:
		return p.Unpassable
	case aUnmoveable:
		return p.Unmoveable
	case aBlockMissile:
		return p.BlockMissile
	case aBlockPathfind:
		return p.BlockPathfind
	case aNoMoveAnimation:
		return p.NoMoveAnimation
	case aPickupable:
		return p.Pickupable
	case aHangable:
		return p.Hangable
	case aHookSouth:
		return p.HookSouth
	case aHookEast:
		return p.HookEast
	case aRotatable:
		return p.Rotatable
	case aLight:
		return p.HasLight
	case aDontHide:
		return p.DontHide
	case aTranslucent:
		return p.Translucent
	case aFloorChange:
		return p.FloorChange
	case aOffset:
		return p.HasOffset
	case aElevation:
		return p.HasElevation
	case aLyingObject:
		return p.LyingObject
	case aAnimateAlways:
		return p.AnimateAlways
	case aMiniMap:
		return p.MiniMap
	case aLensHelp:
		return p.LensHelp
	case aFullGround:
		return p.FullGround
	case aIgnoreLook:
		return p.IgnoreLook
	case aCloth:
		return p.Cloth
	case aMarket:
		return p.IsMarket
	case aDefaultAction:
		return p.HasDefaultAction
	case aWrappable:
		return p.Wrappable
	case aUnwrappable:
		return p.Unwrappable
	case aTopEffect:
		return p.TopEffect
	case aUsable:
		return p.Usable
	case aBones:
		return p.HasBones
	}
	return false
}

// WriteProperties writes every set flag that the table supports, in
// ascending flag order, followed by LastFlag.
func (t *FlagTable) WriteProperties(w *binio.Writer, p *thing.Properties) {
	for _, b := range t.order {
		a := t.byByte[b]
		if !has(a, p) {
			continue
		}
		w.U8(b)
		t.writePayload(w, a, p)
	}
	w.U8(LastFlag)
}

func (t *FlagTable) writePayload(w *binio.Writer, a attr, p *thing.Properties) {
	switch a {
	case aGround:
		w.U16(p.GroundSpeed)
	case aWritable:
		w.U16(p.MaxTextLength)
	case aWritableOnce:
		w.U16(p.MaxReadLength)
	case aLight:
		w.U16(p.LightLevel)
		w.U16(p.LightColor)
	case aOffset:
		if t.offsetHasPayload {
			w.I16(p.OffsetX)
			w.I16(p.OffsetY)
		}
	case aElevation:
		w.U16(p.Elevation)
	case aMiniMap:
		w.U16(p.MiniMapColor)
	case aLensHelp:
		w.U16(p.LensHelpValue)
	case aCloth:
		w.U16(p.ClothSlot)
	case aMarket:
		name := binio.EncodeLatin1(p.Market.Name)
		w.U16(p.Market.Category)
		w.U16(p.Market.TradeAs)
		w.U16(p.Market.ShowAs)
		w.U16(uint16(len(name)))
		w.Write(name)
		w.U16(p.Market.RestrictProfession)
		w.U16(p.Market.RestrictLevel)
	case aDefaultAction:
		w.U16(p.DefaultAction)
	case aBones:
		for _, d := range []thing.Direction{thing.North, thing.South, thing.East, thing.West} {
			w.I16(p.Bones[d].X)
			w.I16(p.Bones[d].Y)
		}
	}
}

// Supported lists json names of properties the table can store.
func (t *FlagTable) Supported() []string {
	out := make([]string, 0, len(t.byAttr))
	for a := aGround; a <= aBones; a++ {
		if _, ok := t.byAttr[a]; ok {
			out = append(out, attrNames[a])
		}
	}
	return out
}

// Unsupported lists json names of properties set in p that the table cannot
// store. Callers show them as warnings before compiling to an older format.
func (t *FlagTable) Unsupported(p *thing.Properties) []string {
	var out []string
	for a := aGround; a <= aBones; a++ {
		if _, ok := t.byAttr[a]; !ok && has(a, p) {
			out = append(out, attrNames[a])
		}
	}
	return out
}

var attrNames = map[attr]string{
	aGround: "ground", aGroundBorder: "groundBorder", aOnBottom: "onBottom", aOnTop: "onTop",
	aContainer: "container", aStackable: "stackable", aForceUse: "forceUse", aMultiUse: "multiUse",
	aHasCharges: "hasCharges", aWritable: "writable", aWritableOnce: "writableOnce",
	aFluidContainer: "fluidContainer", aFluid: "fluid", aUnpassable: "unpassable",
	aUnmoveable: "unmoveable", aBlockMissile: "blockMissile", aBlockPathfind: "blockPathfind",
	aNoMoveAnimation: "noMoveAnimation", aPickupable: "pickupable", aHangable: "hangable",
	aHookSouth: "hookSouth", aHookEast: "hookEast", aRotatable: "rotatable", aLight: "hasLight",
	aDontHide: "dontHide", aTranslucent: "translucent", aFloorChange: "floorChange",
	aOffset: "hasOffset", aElevation: "hasElevation", aLyingObject: "lyingObject",
	aAnimateAlways: "animateAlways", aMiniMap: "miniMap", aLensHelp: "lensHelp",
	aFullGround: "fullGround", aIgnoreLook: "ignoreLook", aCloth: "cloth", aMarket: "isMarket",
	aDefaultAction: "hasDefaultAction", aWrappable: "wrappable", aUnwrappable: "unwrappable",
	aTopEffect: "topEffect", aUsable: "usable", aBones: "hasBones",
}
