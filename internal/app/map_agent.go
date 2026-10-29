package app

import (
	"errors"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/maprender"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Queries of the map editor's MCP server (internal/mcpserver): what an
// agent needs to look at a map without the editor's renderer.

// maxAreaTiles limits the tiles of one AreaTiles request.
const maxAreaTiles = 128 * 128

// maxContentTiles limits the tiles of one AreaContents request.
const maxContentTiles = 512 * 512

// ActiveMap returns the map shown in the editor (0 when none is open).
func (ms *MapService) ActiveMap() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if ms.maps[ms.activeID] != nil {
		return ms.activeID
	}
	lowest := 0
	for id := range ms.maps {
		if lowest == 0 || id < lowest {
			lowest = id
		}
	}
	return lowest
}

// ItemSummary describes an item type for item searches.
type ItemSummary struct {
	ID       uint16 `json:"id"`
	ClientID uint16 `json:"clientId"`
	Name     string `json:"name"`
	// Kind is what the item is for ("container", "door"...; see
	// otitems.Type.KindName), Flags its flags by name.
	Kind  string   `json:"kind,omitempty"`
	Flags []string `json:"flags,omitempty"`
	// Brush and BrushKind name the brush that places the item.
	Brush     string `json:"brush,omitempty"`
	BrushKind string `json:"brushKind,omitempty"`
}

// flagNames lists the names of the flags of an item, sorted.
func flagNames(flags otitems.Flags) []string {
	var names []string
	for name, flag := range otitems.FlagNames() {
		if flags.Has(flag) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func (data *mapData) itemSummary(itemType *otitems.Type) ItemSummary {
	summary := ItemSummary{ID: itemType.ID, ClientID: itemType.ClientID, Name: itemType.DisplayName(), Kind: itemType.KindName(), Flags: flagNames(itemType.Flags)}
	summary.Brush, summary.BrushKind = data.brushes.BrushOf(itemType.ID)
	return summary
}

// SearchItems finds item types by name or id (query "" lists all) that
// have every flag of flags, at most limit.
func (ms *MapService) SearchItems(query string, flags []string, limit int) (found []ItemSummary, err error) {
	var required otitems.Flags
	names := otitems.FlagNames()
	for _, name := range flags {
		flag, ok := names[name]
		if !ok {
			return nil, errors.New("unknown flag " + name)
		}
		required |= flag
	}
	query = strings.ToLower(strings.TrimSpace(query))
	err = ms.withData(func(data *mapData) error {
		data.items.Each(func(itemType *otitems.Type) {
			if len(found) >= limit || !itemType.Flags.Has(required) {
				return
			}
			if query != "" && !strings.Contains(strings.ToLower(itemType.Name), query) && query != itemIDText(itemType.ID) {
				return
			}
			found = append(found, data.itemSummary(itemType))
		})
		return nil
	})
	return found, err
}

// Item describes one item type.
func (ms *MapService) Item(id uint16) (summary ItemSummary, err error) {
	err = ms.withData(func(data *mapData) error {
		itemType := data.items.Get(id)
		if itemType == nil {
			return errors.New("unknown item " + itemIDText(id))
		}
		summary = data.itemSummary(itemType)
		return nil
	})
	return summary, err
}

func itemIDText(id uint16) string { return strconv.Itoa(int(id)) }

// Creatures lists the monsters and NPCs that can be placed.
func (ms *MapService) Creatures() (creatures []*materials.CreatureType, err error) {
	err = ms.withData(func(data *mapData) error {
		creatures = data.allCreatures()
		return nil
	})
	return creatures, err
}

// AreaTiles returns the tiles that exist in a rectangle of one floor
// (corners included), row by row; more is set when limit stopped it.
func (ms *MapService) AreaTiles(id int, from, to otmap.Position, limit int) (tiles []TileInfo, more bool, err error) {
	x0, x1 := int(min(from.X, to.X)), int(max(from.X, to.X))
	y0, y1 := int(min(from.Y, to.Y)), int(max(from.Y, to.Y))
	if (x1-x0+1)*(y1-y0+1) > maxAreaTiles {
		return nil, false, errors.New("the area is too big")
	}
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		return nil, false, err
	}
	defer unlock()
	gameMap := opened.editor.Map
	for y := y0; y <= y1 && !more; y++ {
		for x := x0; x <= x1; x++ {
			pos := otmap.Position{X: uint16(x), Y: uint16(y), Z: from.Z}
			if gameMap.Tile(pos) == nil {
				continue
			}
			if len(tiles) >= limit {
				more = true
				break
			}
			tiles = append(tiles, data.tileInfo(gameMap, pos))
		}
	}
	return tiles, more, nil
}

// RenderArea draws the tiles of one floor in area (tile coordinates, Max
// excluded) with the client's sprites; see maprender.RenderImage.
func (ms *MapService) RenderArea(id int, area image.Rectangle, floor int, creatures bool) (*image.NRGBA, error) {
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	options := maprender.GeometryOptions{}
	if creatures {
		options.ViewFlags = maprender.ViewCreatures
	}
	return maprender.RenderImage(ms.scene(opened, data), area, floor, options)
}

// MinimapArea draws one pixel per tile of one floor in area with the
// minimap colors.
func (ms *MapService) MinimapArea(id int, area image.Rectangle, floor int) (*image.NRGBA, error) {
	if area.Empty() || area.Dx()*area.Dy() > maxMinimapPixels || floor < 0 || floor > otmap.MaxFloor {
		return nil, errors.New("bad area")
	}
	opened, _, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return minimapImage(opened.editor, area, floor), nil
}

// DrawTiles returns the tiles a draw request paints; onlyEmpty leaves out
// tiles that have a ground.
func (ms *MapService) DrawTiles(id int, request mapedit.DrawRequest, onlyEmpty bool) ([]otmap.Position, error) {
	opened, _, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	tiles := opened.editor.Area(request)
	if !onlyEmpty {
		return tiles, nil
	}
	empty := tiles[:0:0]
	for _, pos := range tiles {
		if tile := opened.editor.Map.Tile(pos); tile == nil || tile.Ground.ID == 0 {
			empty = append(empty, pos)
		}
	}
	return empty, nil
}

// TerrainCell is one tile of AreaTerrain.
type TerrainCell struct {
	Exists bool
	// Ground is the ground's brush, else its item name ("" without ground).
	Ground string
	// Blocking is set when an item other than the ground and its borders
	// blocks walking (walls, trees, rocks...).
	Blocking bool
	Creature bool
	NPC      bool
	Spawn    bool
}

// AreaTerrain describes every tile of a rectangle of one floor, row by row.
func (ms *MapService) AreaTerrain(id int, area image.Rectangle, floor int) ([]TerrainCell, error) {
	if area.Empty() || area.Dx()*area.Dy() > maxAreaTiles {
		return nil, errors.New("the area is empty or too big")
	}
	opened, data, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	cells := make([]TerrainCell, 0, area.Dx()*area.Dy())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			tile := opened.editor.Map.Tile(otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(floor)})
			cells = append(cells, data.terrainCell(tile))
		}
	}
	return cells, nil
}

func (data *mapData) terrainCell(tile *otmap.Tile) TerrainCell {
	if tile == nil {
		return TerrainCell{}
	}
	cell := TerrainCell{Exists: true, Spawn: tile.Spawn() != nil || tile.SpawnNPC() != nil}
	if tile.Ground.ID != 0 {
		name, kind := data.brushes.BrushOf(tile.Ground.ID)
		if kind != materials.TypeGround {
			name = "item " + itemIDText(tile.Ground.ID)
			if itemType := data.items.Get(tile.Ground.ID); itemType != nil && itemType.Name != "" {
				name += " " + itemType.Name
			}
		}
		cell.Ground = name
	}
	for _, item := range tile.Items {
		if itemType := data.items.Get(item.ID); itemType != nil && !itemType.IsBorder() && itemType.Flags.Has(otitems.FlagUnpassable) {
			cell.Blocking = true
		}
	}
	for _, creature := range tile.Creatures() {
		cell.Creature = true
		cell.NPC = cell.NPC || creature.NPC
	}
	return cell
}

// GroundHasBorders reports whether a ground brush places borders.
func (ms *MapService) GroundHasBorders(name string) (bool, error) {
	brush, err := ms.Brush(name)
	if err != nil {
		return false, err
	}
	return len(brush.Borders) > 0 || brush.Optional != nil, nil
}

// SetTiles replaces the ground and items of tiles (see
// mapedit.Editor.SetTiles).
func (ms *MapService) SetTiles(id int, specs []mapedit.TileSpec, stroke int64) (MapChange, error) {
	return ms.edit(id, func(editor *mapedit.Editor, _ *mapData) (int, error) {
		return editor.SetTiles(specs, stroke), nil
	})
}

// AreaContents returns the ground and items of the tiles that exist in a
// rectangle of one floor (Max excluded), row by row.
func (ms *MapService) AreaContents(id int, area image.Rectangle, floor int) ([]mapedit.TileSpec, error) {
	if area.Empty() || area.Dx()*area.Dy() > maxContentTiles {
		return nil, errors.New("the area is empty or too big")
	}
	opened, _, unlock, err := ms.read(id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	var specs []mapedit.TileSpec
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			pos := otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(floor)}
			tile := opened.editor.Map.Tile(pos)
			if tile == nil {
				continue
			}
			spec := mapedit.TileSpec{Pos: pos, Ground: tile.Ground.ID, Items: make([]uint16, len(tile.Items))}
			for i, item := range tile.Items {
				spec.Items[i] = item.ID
			}
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

// SettingsDir returns the folder of the map editor settings ("" when they
// are kept in memory).
func (ms *MapService) SettingsDir() string { return ms.store.Dir() }
