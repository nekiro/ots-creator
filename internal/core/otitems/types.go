package otitems

import (
	"strconv"
)

type Group uint8

const (
	GroupNone Group = iota
	GroupGround
	GroupContainer
	GroupWeapon
	GroupAmmunition
	GroupArmor
	GroupCharges
	GroupTeleport
	GroupMagicField
	GroupWritable
	GroupKey
	GroupSplash
	GroupFluid
	GroupDoor
	GroupDeprecated
)

type Kind uint8

const (
	KindNone Kind = iota
	KindDepot
	KindMailbox
	KindTrashHolder
	KindContainer
	KindDoor
	KindMagicField
	KindTeleport
	KindBed
	KindKey
	KindPodium
	KindRune
)

var kindNames = map[string]Kind{
	"depot": KindDepot, "mailbox": KindMailbox, "trashholder": KindTrashHolder, "container": KindContainer,
	"door": KindDoor, "magicfield": KindMagicField, "teleport": KindTeleport, "bed": KindBed, "key": KindKey,
	"podium": KindPodium, "rune": KindRune,
}

func (k Kind) String() string {
	for name, kind := range kindNames {
		if kind == k {
			return name
		}
	}
	return ""
}

type Flags uint32

const (
	FlagGround Flags = 1 << iota
	FlagUnpassable
	FlagBlockMissile
	FlagBlockPath
	FlagPickupable
	FlagMoveable
	FlagStackable
	FlagFluidContainer
	FlagSplash
	FlagHangable
	FlagHookSouth
	FlagHookEast
	FlagRotatable
	FlagReadable
	FlagWritable
	FlagContainer
	FlagFloorChange
	FlagElevation
	FlagFullGround
	FlagCharges
	FlagCorpse
	// FlagMeta marks editor-only items that are never saved.
	FlagMeta
	FlagIgnoreLook
	FlagLight
)

func (f Flags) Has(flags Flags) bool { return f&flags == flags }

type Type struct {
	ID       uint16 `json:"id"`
	ClientID uint16 `json:"clientId"`
	Name     string `json:"name"`
	Article  string `json:"article,omitempty"`
	Plural   string `json:"plural,omitempty"`
	Group    Group  `json:"group"`
	Kind     Kind   `json:"kind"`
	// TopOrder is 1 for borders, 2 for bottom items (walls) and 3 for top
	// items (always drawn above creatures); 0 for common items.
	TopOrder     uint8  `json:"topOrder"`
	Flags        Flags  `json:"flags"`
	MinimapColor uint16 `json:"minimapColor"`
	Speed        uint16 `json:"speed"`
	RotateTo     uint16 `json:"rotateTo,omitempty"`
	MaxItems     uint16 `json:"maxItems,omitempty"`
	LightLevel   uint8  `json:"lightLevel,omitempty"`
	LightColor   uint8  `json:"lightColor,omitempty"`
	EditorSuffix string `json:"editorSuffix,omitempty"`
}

func (t *Type) IsGround() bool { return t.Flags.Has(FlagGround) }

func (t *Type) IsBorder() bool { return t.TopOrder == 1 }

func (t *Type) HasSubtype() bool {
	return t.Flags&(FlagStackable|FlagFluidContainer|FlagSplash|FlagCharges) != 0
}

func (t *Type) IsContainer() bool {
	return t.Flags.Has(FlagContainer) || t.Kind == KindContainer || t.Group == GroupContainer
}

func (t *Type) IsDoor() bool { return t.Kind == KindDoor || t.Group == GroupDoor }

func (t *Type) IsTeleport() bool { return t.Kind == KindTeleport || t.Group == GroupTeleport }

func (t *Type) IsDepot() bool { return t.Kind == KindDepot }

// KindName names what the item is for the inspector: its items.xml type,
// else a kind derived from group and flags ("container", "door",
// "teleport", "fluid", "stackable", "writeable", "charges"), else "".
func (t *Type) KindName() string {
	switch {
	case t.Kind != KindNone:
		return t.Kind.String()
	case t.IsContainer():
		return "container"
	case t.IsDoor():
		return "door"
	case t.IsTeleport():
		return "teleport"
	case t.Flags&(FlagSplash|FlagFluidContainer) != 0:
		return "fluid"
	case t.Flags.Has(FlagStackable):
		return "stackable"
	case t.Flags.Has(FlagWritable):
		return "writeable"
	case t.Flags.Has(FlagCharges):
		return "charges"
	}
	return ""
}

func FlagNames() map[string]Flags {
	return map[string]Flags{
		"ground": FlagGround, "unpassable": FlagUnpassable, "blockMissile": FlagBlockMissile,
		"blockPath": FlagBlockPath, "pickupable": FlagPickupable, "moveable": FlagMoveable,
		"stackable": FlagStackable, "fluidContainer": FlagFluidContainer, "splash": FlagSplash,
		"hangable": FlagHangable, "hookSouth": FlagHookSouth, "hookEast": FlagHookEast,
		"rotatable": FlagRotatable, "readable": FlagReadable, "writable": FlagWritable,
		"container": FlagContainer, "floorChange": FlagFloorChange, "elevation": FlagElevation,
		"fullGround": FlagFullGround, "charges": FlagCharges, "corpse": FlagCorpse,
		"meta": FlagMeta, "light": FlagLight,
	}
}

func (t *Type) DisplayName() string {
	if t.Name != "" {
		return t.Name
	}
	return "item " + strconv.Itoa(int(t.ID))
}
