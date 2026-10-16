package app

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// TileItem is one item of the tile inspector.
type TileItem struct {
	ID        uint16         `json:"id"`
	ClientID  uint16         `json:"clientId"`
	Name      string         `json:"name"`
	Subtype   uint16         `json:"subtype"`
	Selected  bool           `json:"selected"`
	Ext       *otmap.ItemExt `json:"ext"`
	Kind      string         `json:"kind"`
	Flags     otitems.Flags  `json:"flags"`
	TopOrder  uint8          `json:"topOrder"`
	Brush     string         `json:"brush"`
	BrushKind string         `json:"brushKind"`
	RotateTo  uint16         `json:"rotateTo"`
	Contents  []TileItem     `json:"contents"`
}

// TileCreature is a creature with its look.
type TileCreature struct {
	otmap.Creature
	Outfit  materials.Outfit `json:"outfit"`
	Unknown bool             `json:"unknown"`
}

// TileInfo is everything on a tile, for the inspector.
type TileInfo struct {
	Pos       otmap.Position `json:"pos"`
	Exists    bool           `json:"exists"`
	Ground    *TileItem      `json:"ground"`
	Items     []TileItem     `json:"items"`
	Flags     uint32         `json:"flags"`
	HouseID   uint32         `json:"houseId"`
	HouseName string         `json:"houseName"`
	Creatures []TileCreature `json:"creatures"`
	Spawn     *otmap.Spawn   `json:"spawn"`
	SpawnNPC  *otmap.Spawn   `json:"spawnNpc"`
	Zones     []uint16       `json:"zones"`
	Modified  bool           `json:"modified"`
	// HouseExit names the house whose exit is here.
	HouseExit string `json:"houseExit"`
	// Waypoints lists waypoints on the tile; Temple the town whose temple
	// is here.
	Waypoints []string `json:"waypoints"`
	Temple    string   `json:"temple"`
}

// kindInvalid is the TileItem kind of items missing from the item data.
const kindInvalid = "invalid"

func (data *mapData) tileItem(item otmap.Item) TileItem {
	tileItem := TileItem{ID: item.ID, Subtype: item.Subtype, Selected: item.Selected, Ext: item.Ext}
	if itemType := data.items.Get(item.ID); itemType != nil {
		tileItem.ClientID, tileItem.Name, tileItem.Flags = itemType.ClientID, itemType.DisplayName(), itemType.Flags
		tileItem.TopOrder, tileItem.RotateTo, tileItem.Kind = itemType.TopOrder, itemType.RotateTo, itemType.KindName()
	} else {
		tileItem.Name = fmt.Sprintf("unknown item %d", item.ID)
		tileItem.Kind = kindInvalid
	}
	tileItem.Brush, tileItem.BrushKind = data.brushes.BrushOf(item.ID)
	if item.Ext != nil {
		for _, content := range item.Ext.Items {
			tileItem.Contents = append(tileItem.Contents, data.tileItem(content))
		}
	}
	return tileItem
}

// Tile returns the inspector data of a tile.
func (ms *MapService) Tile(id int, p otmap.Position) (info TileInfo, err error) {
	err = ms.withMap(id, func(m *openMap, data *mapData) error {
		info = data.tileInfo(m.editor.Map, p)
		return nil
	})
	return info, err
}

func (data *mapData) tileInfo(m *otmap.Map, p otmap.Position) TileInfo {
	info := TileInfo{Pos: p}
	for _, house := range m.Houses {
		if house.Exit == p {
			info.HouseExit = house.Name
		}
	}
	for _, waypoint := range m.Waypoints {
		if waypoint.Pos == p {
			info.Waypoints = append(info.Waypoints, waypoint.Name)
		}
	}
	for _, town := range m.Towns {
		if town.Temple == p {
			info.Temple = town.Name
		}
	}
	tile := m.Tile(p)
	if tile == nil {
		return info
	}
	info.Exists = true
	if tile.Ground.ID != 0 {
		ground := data.tileItem(tile.Ground)
		info.Ground = &ground
	}
	for _, item := range tile.Items {
		info.Items = append(info.Items, data.tileItem(item))
	}
	info.Flags, info.HouseID, info.Zones = tile.Flags, tile.HouseID, tile.Zones()
	if house := m.Houses[tile.HouseID]; house != nil {
		info.HouseName = house.Name
	}
	for _, creature := range tile.Creatures() {
		outfit, known := data.outfit(creature.Name)
		info.Creatures = append(info.Creatures, TileCreature{Creature: creature, Outfit: outfit, Unknown: !known})
	}
	info.Spawn, info.SpawnNPC = tile.Spawn(), tile.SpawnNPC()
	info.Modified = tile.State&otmap.StateModified != 0
	return info
}
