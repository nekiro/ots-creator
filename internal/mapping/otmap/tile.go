package otmap

import (
	"slices"
)

// Tile map flags (OTBM tile flags, TFS layout).
const (
	FlagPZ       uint32 = 1 << 0
	FlagNoPVP    uint32 = 1 << 2
	FlagNoLogout uint32 = 1 << 3
	FlagPVPZone  uint32 = 1 << 4
	FlagRefresh  uint32 = 1 << 5
)

// Creature is a monster or NPC placed by a spawn.
type Creature struct {
	Name      string `json:"name"`
	NPC       bool   `json:"npc"`
	SpawnTime int32  `json:"spawnTime"`
	Direction uint8  `json:"direction"`
	// Weight is the spawn weight of separate monster files (0 = default).
	Weight   uint8 `json:"weight,omitempty"`
	Selected bool  `json:"selected"`
}

// Spawn is a spawn area centered on its tile.
type Spawn struct {
	Radius   int32 `json:"radius"`
	Selected bool  `json:"selected"`
}

// Tile state bits (editor only, never saved).
const (
	StateModified uint8 = 1 << 0
	// StateOptionalBorder asks the ground brush to add the optional border
	// (mountain) of a neighbor.
	StateOptionalBorder uint8 = 1 << 1
)

// Tile is one map tile.
type Tile struct {
	Ground  Item     `json:"ground"`
	Items   []Item   `json:"items"`
	Flags   uint32   `json:"flags"`
	HouseID uint32   `json:"houseId"`
	State   uint8    `json:"state"`
	Ext     *TileExt `json:"ext,omitempty"`
}

// TileExt holds what few tiles have.
type TileExt struct {
	Creatures []Creature `json:"creatures,omitempty"`
	Spawn     *Spawn     `json:"spawn,omitempty"`
	// SpawnNPC is a separate NPC spawn area.
	SpawnNPC *Spawn   `json:"spawnNpc,omitempty"`
	Zones    []uint16 `json:"zones,omitempty"`
}

func (e *TileExt) empty() bool {
	return e == nil || (len(e.Creatures) == 0 && e.Spawn == nil && e.SpawnNPC == nil && len(e.Zones) == 0)
}

// EnsureExt returns the tile's ext, creating it.
func (t *Tile) EnsureExt() *TileExt {
	if t.Ext == nil {
		t.Ext = &TileExt{}
	}
	return t.Ext
}

// Compact drops an empty ext.
func (t *Tile) Compact() {
	if t.Ext.empty() {
		t.Ext = nil
	}
}

// Empty reports whether the tile holds nothing worth saving.
func (t *Tile) Empty() bool {
	return t == nil || (t.Ground.ID == 0 && len(t.Items) == 0 && t.Flags == 0 && t.HouseID == 0 && t.Ext.empty())
}

// Creatures returns the creatures on the tile.
func (t *Tile) Creatures() []Creature {
	if t.Ext == nil {
		return nil
	}
	return t.Ext.Creatures
}

// Spawn returns the monster spawn on the tile.
func (t *Tile) Spawn() *Spawn {
	if t.Ext == nil {
		return nil
	}
	return t.Ext.Spawn
}

// SpawnNPC returns the NPC spawn on the tile.
func (t *Tile) SpawnNPC() *Spawn {
	if t.Ext == nil {
		return nil
	}
	return t.Ext.SpawnNPC
}

// Zones returns the zone ids of the tile.
func (t *Tile) Zones() []uint16 {
	if t.Ext == nil {
		return nil
	}
	return t.Ext.Zones
}

// Clone returns a deep copy.
func (t *Tile) Clone() *Tile {
	if t == nil {
		return nil
	}
	clone := *t
	clone.Ground = t.Ground.Clone()
	clone.Items = cloneItems(t.Items)
	if t.Ext != nil {
		ext := *t.Ext
		ext.Creatures = slices.Clone(ext.Creatures)
		if ext.Spawn != nil {
			spawn := *ext.Spawn
			ext.Spawn = &spawn
		}
		if ext.SpawnNPC != nil {
			spawn := *ext.SpawnNPC
			ext.SpawnNPC = &spawn
		}
		ext.Zones = slices.Clone(ext.Zones)
		clone.Ext = &ext
	}
	return &clone
}

// Selected reports whether anything on the tile is selected.
func (t *Tile) Selected() bool {
	if t == nil {
		return false
	}
	if t.Ground.Selected {
		return true
	}
	for i := range t.Items {
		if t.Items[i].Selected {
			return true
		}
	}
	if t.Ext != nil {
		for _, creature := range t.Ext.Creatures {
			if creature.Selected {
				return true
			}
		}
		if t.Ext.Spawn != nil && t.Ext.Spawn.Selected {
			return true
		}
		if t.Ext.SpawnNPC != nil && t.Ext.SpawnNPC.Selected {
			return true
		}
	}
	return false
}

// SetSelected selects or deselects everything on the tile.
func (t *Tile) SetSelected(on bool) {
	if t.Ground.ID != 0 {
		t.Ground.Selected = on
	}
	for i := range t.Items {
		t.Items[i].Selected = on
	}
	if t.Ext != nil {
		for i := range t.Ext.Creatures {
			t.Ext.Creatures[i].Selected = on
		}
		if t.Ext.Spawn != nil {
			t.Ext.Spawn.Selected = on
		}
		if t.Ext.SpawnNPC != nil {
			t.Ext.SpawnNPC.Selected = on
		}
	}
}

// ItemCount returns the number of items including the ground.
func (t *Tile) ItemCount() int {
	count := len(t.Items)
	if t.Ground.ID != 0 {
		count++
	}
	return count
}
