package dat

import "slices"

func newTable(name string, offsetHasPayload bool, byByte map[byte]attr) *FlagTable {
	table := &FlagTable{name: name, offsetHasPayload: offsetHasPayload, byByte: byByte, byAttr: map[attr]byte{}}
	for flagByte, attribute := range byByte {
		table.byAttr[attribute] = flagByte
		table.order = append(table.order, flagByte)
	}
	slices.Sort(table.order)
	return table
}

var table1 = newTable("7.10-7.30", false, map[byte]attr{
	0x00: attrGround, 0x01: attrOnBottom, 0x02: attrOnTop, 0x03: attrContainer, 0x04: attrStackable,
	0x05: attrMultiUse, 0x06: attrForceUse, 0x07: attrWritable, 0x08: attrWritableOnce,
	0x09: attrFluidContainer, 0x0A: attrFluid, 0x0B: attrUnpassable, 0x0C: attrUnmoveable,
	0x0D: attrBlockMissile, 0x0E: attrBlockPathfind, 0x0F: attrPickupable, 0x10: attrLight,
	0x11: attrFloorChange, 0x12: attrFullGround, 0x13: attrElevation, 0x14: attrOffset,
	0x16: attrMiniMap, 0x17: attrRotatable, 0x18: attrLyingObject, 0x19: attrAnimateAlways,
	0x1A: attrLensHelp, 0x24: attrWrappable, 0x25: attrUnwrappable, 0x26: attrTopEffect,
})

var table2 = newTable("7.40-7.50", false, map[byte]attr{
	0x00: attrGround, 0x01: attrOnBottom, 0x02: attrOnTop, 0x03: attrContainer, 0x04: attrStackable,
	0x05: attrMultiUse, 0x06: attrForceUse, 0x07: attrWritable, 0x08: attrWritableOnce,
	0x09: attrFluidContainer, 0x0A: attrFluid, 0x0B: attrUnpassable, 0x0C: attrUnmoveable,
	0x0D: attrBlockMissile, 0x0E: attrBlockPathfind, 0x0F: attrPickupable, 0x10: attrLight,
	0x11: attrFloorChange, 0x12: attrFullGround, 0x13: attrElevation, 0x14: attrOffset,
	0x16: attrMiniMap, 0x17: attrRotatable, 0x18: attrLyingObject, 0x19: attrHangable,
	0x1A: attrHookSouth, 0x1B: attrHookEast, 0x1C: attrAnimateAlways, 0x1D: attrLensHelp,
	0x24: attrWrappable, 0x25: attrUnwrappable, 0x26: attrTopEffect,
})

var table3 = newTable("7.55-7.72", true, map[byte]attr{
	0x00: attrGround, 0x01: attrGroundBorder, 0x02: attrOnBottom, 0x03: attrOnTop, 0x04: attrContainer,
	0x05: attrStackable, 0x06: attrForceUse, 0x07: attrMultiUse, 0x08: attrWritable, 0x09: attrWritableOnce,
	0x0A: attrFluidContainer, 0x0B: attrFluid, 0x0C: attrUnpassable, 0x0D: attrUnmoveable,
	0x0E: attrBlockMissile, 0x0F: attrBlockPathfind, 0x10: attrPickupable, 0x11: attrHangable,
	0x12: attrHookSouth, 0x13: attrHookEast, 0x14: attrRotatable, 0x15: attrLight, 0x17: attrFloorChange,
	0x18: attrOffset, 0x19: attrElevation, 0x1A: attrLyingObject, 0x1B: attrAnimateAlways,
	0x1C: attrMiniMap, 0x1D: attrLensHelp, 0x1E: attrFullGround,
})

var table4 = newTable("7.80-8.54", true, map[byte]attr{
	0x00: attrGround, 0x01: attrGroundBorder, 0x02: attrOnBottom, 0x03: attrOnTop, 0x04: attrContainer,
	0x05: attrStackable, 0x06: attrForceUse, 0x07: attrMultiUse, 0x08: attrHasCharges, 0x09: attrWritable,
	0x0A: attrWritableOnce, 0x0B: attrFluidContainer, 0x0C: attrFluid, 0x0D: attrUnpassable,
	0x0E: attrUnmoveable, 0x0F: attrBlockMissile, 0x10: attrBlockPathfind, 0x11: attrPickupable,
	0x12: attrHangable, 0x13: attrHookSouth, 0x14: attrHookEast, 0x15: attrRotatable, 0x16: attrLight,
	0x17: attrDontHide, 0x18: attrFloorChange, 0x19: attrOffset, 0x1A: attrElevation, 0x1B: attrLyingObject,
	0x1C: attrAnimateAlways, 0x1D: attrMiniMap, 0x1E: attrLensHelp, 0x1F: attrFullGround, 0x20: attrIgnoreLook,
	0x24: attrWrappable, 0x25: attrUnwrappable, 0x27: attrBones,
})

var table5 = newTable("8.60-9.86", true, map[byte]attr{
	0x00: attrGround, 0x01: attrGroundBorder, 0x02: attrOnBottom, 0x03: attrOnTop, 0x04: attrContainer,
	0x05: attrStackable, 0x06: attrForceUse, 0x07: attrMultiUse, 0x08: attrWritable, 0x09: attrWritableOnce,
	0x0A: attrFluidContainer, 0x0B: attrFluid, 0x0C: attrUnpassable, 0x0D: attrUnmoveable,
	0x0E: attrBlockMissile, 0x0F: attrBlockPathfind, 0x10: attrPickupable, 0x11: attrHangable,
	0x12: attrHookSouth, 0x13: attrHookEast, 0x14: attrRotatable, 0x15: attrLight, 0x16: attrDontHide,
	0x17: attrTranslucent, 0x18: attrOffset, 0x19: attrElevation, 0x1A: attrLyingObject,
	0x1B: attrAnimateAlways, 0x1C: attrMiniMap, 0x1D: attrLensHelp, 0x1E: attrFullGround,
	0x1F: attrIgnoreLook, 0x20: attrCloth, 0x21: attrMarket, 0x27: attrBones,
})

var table6 = newTable("10.10+", true, map[byte]attr{
	0x00: attrGround, 0x01: attrGroundBorder, 0x02: attrOnBottom, 0x03: attrOnTop, 0x04: attrContainer,
	0x05: attrStackable, 0x06: attrForceUse, 0x07: attrMultiUse, 0x08: attrWritable, 0x09: attrWritableOnce,
	0x0A: attrFluidContainer, 0x0B: attrFluid, 0x0C: attrUnpassable, 0x0D: attrUnmoveable,
	0x0E: attrBlockMissile, 0x0F: attrBlockPathfind, 0x10: attrNoMoveAnimation, 0x11: attrPickupable,
	0x12: attrHangable, 0x13: attrHookSouth, 0x14: attrHookEast, 0x15: attrRotatable, 0x16: attrLight,
	0x17: attrDontHide, 0x18: attrTranslucent, 0x19: attrOffset, 0x1A: attrElevation, 0x1B: attrLyingObject,
	0x1C: attrAnimateAlways, 0x1D: attrMiniMap, 0x1E: attrLensHelp, 0x1F: attrFullGround, 0x20: attrIgnoreLook,
	0x21: attrCloth, 0x22: attrMarket, 0x23: attrDefaultAction, 0x24: attrWrappable, 0x25: attrUnwrappable,
	0x26: attrTopEffect, 0x27: attrBones, 0xFE: attrUsable,
})

// OBDTable is the flag table of OBD v2/v3 files. It is the 10.10+ table
// plus three private flags, without bones.
var OBDTable = newTable("obd", true, map[byte]attr{
	0x00: attrGround, 0x01: attrGroundBorder, 0x02: attrOnBottom, 0x03: attrOnTop, 0x04: attrContainer,
	0x05: attrStackable, 0x06: attrForceUse, 0x07: attrMultiUse, 0x08: attrWritable, 0x09: attrWritableOnce,
	0x0A: attrFluidContainer, 0x0B: attrFluid, 0x0C: attrUnpassable, 0x0D: attrUnmoveable,
	0x0E: attrBlockMissile, 0x0F: attrBlockPathfind, 0x10: attrNoMoveAnimation, 0x11: attrPickupable,
	0x12: attrHangable, 0x13: attrHookSouth, 0x14: attrHookEast, 0x15: attrRotatable, 0x16: attrLight,
	0x17: attrDontHide, 0x18: attrTranslucent, 0x19: attrOffset, 0x1A: attrElevation, 0x1B: attrLyingObject,
	0x1C: attrAnimateAlways, 0x1D: attrMiniMap, 0x1E: attrLensHelp, 0x1F: attrFullGround, 0x20: attrIgnoreLook,
	0x21: attrCloth, 0x22: attrMarket, 0x23: attrDefaultAction, 0x24: attrWrappable, 0x25: attrUnwrappable,
	0x26: attrTopEffect, 0xFC: attrHasCharges, 0xFD: attrFloorChange, 0xFE: attrUsable,
})

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
