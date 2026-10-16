package dat

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/thing"
)

// attr is a version-independent flag identifier.
type attr uint8

const (
	attrGround attr = iota + 1
	attrGroundBorder
	attrOnBottom
	attrOnTop
	attrContainer
	attrStackable
	attrForceUse
	attrMultiUse
	attrHasCharges
	attrWritable
	attrWritableOnce
	attrFluidContainer
	attrFluid
	attrUnpassable
	attrUnmoveable
	attrBlockMissile
	attrBlockPathfind
	attrNoMoveAnimation
	attrPickupable
	attrHangable
	attrHookSouth
	attrHookEast
	attrRotatable
	attrLight
	attrDontHide
	attrTranslucent
	attrFloorChange
	attrOffset
	attrElevation
	attrLyingObject
	attrAnimateAlways
	attrMiniMap
	attrLensHelp
	attrFullGround
	attrIgnoreLook
	attrCloth
	attrMarket
	attrDefaultAction
	attrWrappable
	attrUnwrappable
	attrTopEffect
	attrUsable
	attrBones
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

// Name returns a human readable name of the table.
func (t *FlagTable) Name() string { return t.name }

// ReadProperties reads flags until LastFlag into props.
func (t *FlagTable) ReadProperties(r *binio.Reader, props *thing.Properties) error {
	previous := -1
	for {
		flagByte := r.U8()
		if r.Err() != nil {
			return r.Err()
		}
		if flagByte == LastFlag {
			return nil
		}
		attribute, ok := t.byByte[flagByte]
		if !ok {
			return fmt.Errorf("unknown flag 0x%02X (previous 0x%02X) for format %s at offset %d", flagByte, previous, t.name, r.Pos()-1)
		}
		spec := attrSpecs[attribute]
		*spec.flag(props) = true
		if spec.read != nil {
			spec.read(t, r, props)
		}
		previous = int(flagByte)
	}
}

// WriteProperties writes every set flag that the table supports, in
// ascending flag order, followed by LastFlag.
func (t *FlagTable) WriteProperties(w *binio.Writer, props *thing.Properties) {
	for _, flagByte := range t.order {
		attribute := t.byByte[flagByte]
		if !attribute.isSet(props) {
			continue
		}
		w.U8(flagByte)
		if write := attrSpecs[attribute].write; write != nil {
			write(t, w, props)
		}
	}
	w.U8(LastFlag)
}

// Supported lists json names of properties the table can store.
func (t *FlagTable) Supported() []string {
	out := make([]string, 0, len(t.byAttr))
	for attribute := attrGround; attribute <= attrBones; attribute++ {
		if _, ok := t.byAttr[attribute]; ok {
			out = append(out, attribute.name())
		}
	}
	return out
}

// Unsupported lists json names of properties set in props that the table
// cannot store. Callers show them as warnings before compiling to an older
// format.
func (t *FlagTable) Unsupported(props *thing.Properties) []string {
	var out []string
	for attribute := attrGround; attribute <= attrBones; attribute++ {
		if _, ok := t.byAttr[attribute]; !ok && attribute.isSet(props) {
			out = append(out, attribute.name())
		}
	}
	return append(out, props.AssetOnly()...)
}

// isSet reports whether the attribute is set in props.
func (a attr) isSet(props *thing.Properties) bool { return *attrSpecs[a].flag(props) }

// name returns the json name of the property behind the attribute.
func (a attr) name() string { return attrSpecs[a].name }

// attrSpec describes how an attribute maps to thing.Properties.
type attrSpec struct {
	name string // json name of the property
	// flag returns the property that tells whether the attribute is set.
	flag func(*thing.Properties) *bool
	// read and write handle the payload after the flag byte; nil when the
	// attribute has none.
	read  func(*FlagTable, *binio.Reader, *thing.Properties)
	write func(*FlagTable, *binio.Writer, *thing.Properties)
}

// boolAttr is an attribute without payload.
func boolAttr(name string, flag func(*thing.Properties) *bool) attrSpec {
	return attrSpec{name: name, flag: flag}
}

// valueAttr is an attribute with one uint16 payload.
func valueAttr(name string, fields func(*thing.Properties) (set *bool, value *uint16)) attrSpec {
	return attrSpec{
		name: name,
		flag: func(props *thing.Properties) *bool {
			set, _ := fields(props)
			return set
		},
		read: func(_ *FlagTable, r *binio.Reader, props *thing.Properties) {
			_, value := fields(props)
			*value = r.U16()
		},
		write: func(_ *FlagTable, w *binio.Writer, props *thing.Properties) {
			_, value := fields(props)
			w.U16(*value)
		},
	}
}

// boneOrder is the order of the bone offsets in the file.
var boneOrder = []thing.Direction{thing.North, thing.South, thing.East, thing.West}

var attrSpecs = [...]attrSpec{
	attrGround: valueAttr("ground", func(props *thing.Properties) (*bool, *uint16) {
		return &props.Ground, &props.GroundSpeed
	}),
	attrGroundBorder: boolAttr("groundBorder", func(props *thing.Properties) *bool { return &props.GroundBorder }),
	attrOnBottom:     boolAttr("onBottom", func(props *thing.Properties) *bool { return &props.OnBottom }),
	attrOnTop:        boolAttr("onTop", func(props *thing.Properties) *bool { return &props.OnTop }),
	attrContainer:    boolAttr("container", func(props *thing.Properties) *bool { return &props.Container }),
	attrStackable:    boolAttr("stackable", func(props *thing.Properties) *bool { return &props.Stackable }),
	attrForceUse:     boolAttr("forceUse", func(props *thing.Properties) *bool { return &props.ForceUse }),
	attrMultiUse:     boolAttr("multiUse", func(props *thing.Properties) *bool { return &props.MultiUse }),
	attrHasCharges:   boolAttr("hasCharges", func(props *thing.Properties) *bool { return &props.HasCharges }),
	attrWritable: valueAttr("writable", func(props *thing.Properties) (*bool, *uint16) {
		return &props.Writable, &props.MaxTextLength
	}),
	attrWritableOnce: valueAttr("writableOnce", func(props *thing.Properties) (*bool, *uint16) {
		return &props.WritableOnce, &props.MaxReadLength
	}),
	attrFluidContainer:  boolAttr("fluidContainer", func(props *thing.Properties) *bool { return &props.FluidContainer }),
	attrFluid:           boolAttr("fluid", func(props *thing.Properties) *bool { return &props.Fluid }),
	attrUnpassable:      boolAttr("unpassable", func(props *thing.Properties) *bool { return &props.Unpassable }),
	attrUnmoveable:      boolAttr("unmoveable", func(props *thing.Properties) *bool { return &props.Unmoveable }),
	attrBlockMissile:    boolAttr("blockMissile", func(props *thing.Properties) *bool { return &props.BlockMissile }),
	attrBlockPathfind:   boolAttr("blockPathfind", func(props *thing.Properties) *bool { return &props.BlockPathfind }),
	attrNoMoveAnimation: boolAttr("noMoveAnimation", func(props *thing.Properties) *bool { return &props.NoMoveAnimation }),
	attrPickupable:      boolAttr("pickupable", func(props *thing.Properties) *bool { return &props.Pickupable }),
	attrHangable:        boolAttr("hangable", func(props *thing.Properties) *bool { return &props.Hangable }),
	attrHookSouth:       boolAttr("hookSouth", func(props *thing.Properties) *bool { return &props.HookSouth }),
	attrHookEast:        boolAttr("hookEast", func(props *thing.Properties) *bool { return &props.HookEast }),
	attrRotatable:       boolAttr("rotatable", func(props *thing.Properties) *bool { return &props.Rotatable }),
	attrLight: {
		name: "hasLight",
		flag: func(props *thing.Properties) *bool { return &props.HasLight },
		read: func(_ *FlagTable, r *binio.Reader, props *thing.Properties) {
			props.LightLevel = r.U16()
			props.LightColor = r.U16()
		},
		write: func(_ *FlagTable, w *binio.Writer, props *thing.Properties) {
			w.U16(props.LightLevel)
			w.U16(props.LightColor)
		},
	},
	attrDontHide:    boolAttr("dontHide", func(props *thing.Properties) *bool { return &props.DontHide }),
	attrTranslucent: boolAttr("translucent", func(props *thing.Properties) *bool { return &props.Translucent }),
	attrFloorChange: boolAttr("floorChange", func(props *thing.Properties) *bool { return &props.FloorChange }),
	attrOffset: {
		name: "hasOffset",
		flag: func(props *thing.Properties) *bool { return &props.HasOffset },
		read: func(table *FlagTable, r *binio.Reader, props *thing.Properties) {
			if table.offsetHasPayload {
				props.OffsetX = r.I16()
				props.OffsetY = r.I16()
			} else {
				props.OffsetX, props.OffsetY = 8, 8
			}
		},
		write: func(table *FlagTable, w *binio.Writer, props *thing.Properties) {
			if table.offsetHasPayload {
				w.I16(props.OffsetX)
				w.I16(props.OffsetY)
			}
		},
	},
	attrElevation: valueAttr("hasElevation", func(props *thing.Properties) (*bool, *uint16) {
		return &props.HasElevation, &props.Elevation
	}),
	attrLyingObject:   boolAttr("lyingObject", func(props *thing.Properties) *bool { return &props.LyingObject }),
	attrAnimateAlways: boolAttr("animateAlways", func(props *thing.Properties) *bool { return &props.AnimateAlways }),
	attrMiniMap: valueAttr("miniMap", func(props *thing.Properties) (*bool, *uint16) {
		return &props.MiniMap, &props.MiniMapColor
	}),
	attrLensHelp: valueAttr("lensHelp", func(props *thing.Properties) (*bool, *uint16) {
		return &props.LensHelp, &props.LensHelpValue
	}),
	attrFullGround: boolAttr("fullGround", func(props *thing.Properties) *bool { return &props.FullGround }),
	attrIgnoreLook: boolAttr("ignoreLook", func(props *thing.Properties) *bool { return &props.IgnoreLook }),
	attrCloth: valueAttr("cloth", func(props *thing.Properties) (*bool, *uint16) {
		return &props.Cloth, &props.ClothSlot
	}),
	attrMarket: {
		name: "isMarket",
		flag: func(props *thing.Properties) *bool { return &props.IsMarket },
		read: func(_ *FlagTable, r *binio.Reader, props *thing.Properties) {
			props.Market.Category = r.U16()
			props.Market.TradeAs = r.U16()
			props.Market.ShowAs = r.U16()
			props.Market.Name = r.Latin1(int(r.U16()))
			props.Market.RestrictProfession = r.U16()
			props.Market.RestrictLevel = r.U16()
		},
		write: func(_ *FlagTable, w *binio.Writer, props *thing.Properties) {
			name := binio.EncodeLatin1(props.Market.Name)
			w.U16(props.Market.Category)
			w.U16(props.Market.TradeAs)
			w.U16(props.Market.ShowAs)
			w.U16(uint16(len(name)))
			w.Write(name)
			w.U16(props.Market.RestrictProfession)
			w.U16(props.Market.RestrictLevel)
		},
	},
	attrDefaultAction: valueAttr("hasDefaultAction", func(props *thing.Properties) (*bool, *uint16) {
		return &props.HasDefaultAction, &props.DefaultAction
	}),
	attrWrappable:   boolAttr("wrappable", func(props *thing.Properties) *bool { return &props.Wrappable }),
	attrUnwrappable: boolAttr("unwrappable", func(props *thing.Properties) *bool { return &props.Unwrappable }),
	attrTopEffect:   boolAttr("topEffect", func(props *thing.Properties) *bool { return &props.TopEffect }),
	attrUsable:      boolAttr("usable", func(props *thing.Properties) *bool { return &props.Usable }),
	attrBones: {
		name: "hasBones",
		flag: func(props *thing.Properties) *bool { return &props.HasBones },
		read: func(_ *FlagTable, r *binio.Reader, props *thing.Properties) {
			for _, direction := range boneOrder {
				props.Bones[direction].X = r.I16()
				props.Bones[direction].Y = r.I16()
			}
		},
		write: func(_ *FlagTable, w *binio.Writer, props *thing.Properties) {
			for _, direction := range boneOrder {
				w.I16(props.Bones[direction].X)
				w.I16(props.Bones[direction].Y)
			}
		},
	},
}
