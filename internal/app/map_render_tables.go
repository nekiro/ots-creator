package app

import (
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// ItemTable lists every item type in parallel arrays (compact for the
// palette, search and drawing).
type ItemTable struct {
	IDs      []uint16          `json:"ids"`
	Clients  []uint16          `json:"clients"`
	Flags    []uint32          `json:"flags"`
	Tops     []uint16          `json:"tops"`
	Minimap  []uint16          `json:"minimap"`
	Names    []string          `json:"names"`
	Kinds    []uint16          `json:"kinds"`
	Mode     string            `json:"mode"`
	MaxID    uint16            `json:"maxId"`
	Flagbits map[string]uint32 `json:"flagbits"`
}

// Items returns the item table.
func (ms *MapService) Items() (table ItemTable, err error) {
	err = ms.withData(func(data *mapData) error {
		table = itemTable(data.items)
		return nil
	})
	return table, err
}

func itemTable(items *otitems.DB) ItemTable {
	table := ItemTable{Mode: string(items.Mode), MaxID: items.MaxID(), Flagbits: map[string]uint32{}}
	for name, flag := range otitems.FlagNames() {
		table.Flagbits[name] = uint32(flag)
	}
	items.Each(func(item *otitems.Type) {
		table.IDs = append(table.IDs, item.ID)
		table.Clients = append(table.Clients, item.ClientID)
		table.Flags = append(table.Flags, uint32(item.Flags))
		table.Tops = append(table.Tops, uint16(item.TopOrder))
		table.Minimap = append(table.Minimap, item.MinimapColor)
		table.Names = append(table.Names, item.Name)
		table.Kinds = append(table.Kinds, uint16(item.Kind))
	})
	return table
}

// Appearance flags, the bits of Appearance.Flags.
const (
	appearanceAnimateAlways uint32 = 1 << iota
	appearanceHangable
	appearanceHookSouth
	appearanceHookEast
	appearanceLyingObject
	appearanceTranslucent
	appearanceTop
	appearanceBottom
	appearanceBorder
	appearanceGround
	appearanceDontHide
	appearanceFullGround
)

// Appearance is what the map renderer needs to draw a client thing.
type Appearance struct {
	ID        uint32   `json:"id"`
	Width     uint8    `json:"w"`
	Height    uint8    `json:"h"`
	Layers    uint8    `json:"l"`
	PatternX  uint8    `json:"px"`
	PatternY  uint8    `json:"py"`
	PatternZ  uint8    `json:"pz"`
	Frames    uint8    `json:"f"`
	Sprites   []uint32 `json:"s"`
	Durations []uint32 `json:"d,omitempty"`
	OffsetX   int16    `json:"ox,omitempty"`
	OffsetY   int16    `json:"oy,omitempty"`
	Elevation uint16   `json:"el,omitempty"`
	// Flags: 1 animate always, 2 hangable, 4 hook south, 8 hook east,
	// 16 lying object, 32 translucent, 64 top, 128 bottom, 256 border,
	// 512 ground, 1024 dont hide, 2048 full ground.
	Flags      uint32 `json:"fl"`
	LightLevel uint16 `json:"ll,omitempty"`
	LightColor uint16 `json:"lc,omitempty"`
}

// Appearances returns render data of client things (missing ids are left
// out).
func (ms *MapService) Appearances(category thing.Category, ids []uint32) ([]Appearance, error) {
	p, err := ms.session.Project()
	if err != nil {
		return nil, errNoClient
	}
	out := make([]Appearance, 0, len(ids))
	for _, id := range ids {
		found, err := p.Thing(category, id)
		if err != nil || len(found.FrameGroups) == 0 {
			continue
		}
		out = append(out, appearanceOf(id, found))
	}
	return out, nil
}

func appearanceOf(id uint32, found *thing.Thing) Appearance {
	group := found.FrameGroups[0]
	appearance := Appearance{
		ID: id, Width: group.Width, Height: group.Height, Layers: group.Layers,
		PatternX: group.PatternX, PatternY: group.PatternY, PatternZ: group.PatternZ,
		Frames: group.Frames, Sprites: group.Sprites,
	}
	if group.Frames > 1 {
		for _, duration := range group.Durations {
			appearance.Durations = append(appearance.Durations, (duration.Min+duration.Max)/2)
		}
	}
	props := &found.Props
	if props.HasOffset {
		appearance.OffsetX, appearance.OffsetY = props.OffsetX, props.OffsetY
	}
	if props.HasElevation {
		appearance.Elevation = props.Elevation
	}
	if props.HasLight {
		appearance.LightLevel, appearance.LightColor = props.LightLevel, props.LightColor
	}
	for _, flag := range []struct {
		on  bool
		bit uint32
	}{
		{props.AnimateAlways, appearanceAnimateAlways}, {props.Hangable, appearanceHangable},
		{props.HookSouth, appearanceHookSouth}, {props.HookEast, appearanceHookEast},
		{props.LyingObject, appearanceLyingObject}, {props.Translucent, appearanceTranslucent},
		{props.OnTop, appearanceTop}, {props.OnBottom, appearanceBottom},
		{props.GroundBorder, appearanceBorder}, {props.Ground, appearanceGround},
		{props.DontHide, appearanceDontHide}, {props.FullGround, appearanceFullGround},
	} {
		if flag.on {
			appearance.Flags |= flag.bit
		}
	}
	return appearance
}
