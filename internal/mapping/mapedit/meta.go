package mapedit

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Properties are the editable map header fields.
type Properties struct {
	Description string            `json:"description"`
	Width       uint16            `json:"width"`
	Height      uint16            `json:"height"`
	HouseFile   string            `json:"houseFile"`
	SpawnFile   string            `json:"spawnFile"`
	NpcFile     string            `json:"npcFile"`
	ZoneFile    string            `json:"zoneFile"`
	SpawnFormat otmap.SpawnFormat `json:"spawnFormat"`
}

// Properties returns the map header.
func (e *Editor) Properties() Properties {
	gameMap := e.Map
	return Properties{gameMap.Description, gameMap.Width, gameMap.Height, gameMap.HouseFile, gameMap.SpawnFile, gameMap.NpcFile, gameMap.ZoneFile, gameMap.SpawnFormat}
}

// SetProperties changes the map header.
func (e *Editor) SetProperties(props Properties) error {
	if props.SpawnFormat != otmap.SpawnsSingle && props.SpawnFormat != otmap.SpawnsSeparate {
		props.SpawnFormat = otmap.SpawnsSingle
	}
	tx := e.Begin("Map properties", 0)
	tx.Meta()
	gameMap := e.Map
	gameMap.Description, gameMap.Width, gameMap.Height = props.Description, max(props.Width, 1), max(props.Height, 1)
	gameMap.HouseFile, gameMap.SpawnFile, gameMap.NpcFile, gameMap.ZoneFile, gameMap.SpawnFormat = props.HouseFile, props.SpawnFile, props.NpcFile, props.ZoneFile, props.SpawnFormat
	tx.Commit()
	return nil
}

// HouseInfo is a house with its size.
type HouseInfo struct {
	otmap.House
	Size int `json:"size"`
}

// Houses lists the houses with their tile counts.
func (e *Editor) Houses() []HouseInfo {
	sizes := otmap.HouseSizes(e.Map)
	var out []HouseInfo
	for _, house := range e.Map.SortedHouses() {
		out = append(out, HouseInfo{*house, sizes[house.ID]})
	}
	return out
}

// firstFreeHouseID returns the lowest house id not in use.
func (e *Editor) firstFreeHouseID() uint32 {
	id := uint32(1)
	for e.Map.Houses[id] != nil {
		id++
	}
	return id
}

// SaveHouse creates or updates a house (ID 0 creates one with a free id).
func (e *Editor) SaveHouse(house otmap.House) (uint32, error) {
	if strings.TrimSpace(house.Name) == "" {
		return 0, errors.New("the house needs a name")
	}
	tx := e.Begin("Edit house", 0)
	tx.Meta()
	if house.ID == 0 {
		house.ID = e.firstFreeHouseID()
		tx.label = "Add house"
	}
	stored := house
	e.Map.Houses[house.ID] = &stored
	tx.Commit()
	return house.ID, nil
}

// RemoveHouse deletes a house and clears its tiles.
func (e *Editor) RemoveHouse(id uint32) {
	tx := e.Begin("Remove house", 0)
	tx.Meta()
	delete(e.Map.Houses, id)
	e.Map.EachStored(func(pos otmap.Position, tile *otmap.Tile) {
		if tile.HouseID == id {
			edited := tx.Edit(pos)
			edited.HouseID = 0
			edited.Flags &^= otmap.FlagPZ
		}
	})
	tx.Commit()
}

// HouseTiles returns the tiles of a house.
func (e *Editor) HouseTiles(id uint32) []otmap.Position {
	var out []otmap.Position
	e.Map.EachStored(func(pos otmap.Position, tile *otmap.Tile) {
		if tile.HouseID == id {
			out = append(out, pos)
		}
	})
	return out
}

// SaveTown creates or updates a town (ID 0 creates one).
func (e *Editor) SaveTown(town otmap.Town) (uint32, error) {
	if strings.TrimSpace(town.Name) == "" {
		return 0, errors.New("the town needs a name")
	}
	tx := e.Begin("Edit town", 0)
	tx.Meta()
	if town.ID == 0 {
		for id := uint32(1); ; id++ {
			if e.Map.Towns[id] == nil {
				town.ID = id
				break
			}
		}
	}
	stored := town
	e.Map.Towns[town.ID] = &stored
	tx.Commit()
	return town.ID, nil
}

// RemoveTown deletes a town. Houses of the town keep their town id.
func (e *Editor) RemoveTown(id uint32) {
	tx := e.Begin("Remove town", 0)
	tx.Meta()
	delete(e.Map.Towns, id)
	tx.Commit()
}

// SaveWaypoint creates, renames or moves a waypoint. oldName is the current
// name ("" for a new waypoint).
func (e *Editor) SaveWaypoint(oldName string, waypoint otmap.Waypoint) error {
	waypoint.Name = strings.TrimSpace(waypoint.Name)
	if waypoint.Name == "" {
		return errors.New("the waypoint needs a name")
	}
	if !strings.EqualFold(oldName, waypoint.Name) && e.Map.Waypoint(waypoint.Name) != nil {
		return fmt.Errorf("a waypoint named %q exists", waypoint.Name)
	}
	tx := e.Begin("Edit waypoint", 0)
	tx.Meta()
	if existing := e.Map.Waypoint(oldName); oldName != "" && existing != nil {
		*existing = waypoint
	} else {
		stored := waypoint
		e.Map.Waypoints = append(e.Map.Waypoints, &stored)
	}
	tx.Commit()
	return nil
}

// RemoveWaypoint deletes a waypoint.
func (e *Editor) RemoveWaypoint(name string) {
	tx := e.Begin("Remove waypoint", 0)
	tx.Meta()
	e.Map.Waypoints = slices.DeleteFunc(e.Map.Waypoints, func(waypoint *otmap.Waypoint) bool { return waypoint.Name == name })
	tx.Commit()
}

// SaveZone creates or renames a zone (ID 0 creates one).
func (e *Editor) SaveZone(zone otmap.Zone) (uint16, error) {
	if strings.TrimSpace(zone.Name) == "" {
		return 0, errors.New("the zone needs a name")
	}
	tx := e.Begin("Edit zone", 0)
	tx.Meta()
	if zone.ID == 0 {
		used := map[uint16]bool{}
		for _, existing := range e.Map.Zones {
			used[existing.ID] = true
		}
		for id := uint16(1); ; id++ {
			if !used[id] {
				zone.ID = id
				break
			}
		}
	}
	index := slices.IndexFunc(e.Map.Zones, func(existing otmap.Zone) bool { return existing.ID == zone.ID })
	if index >= 0 {
		e.Map.Zones[index] = zone
	} else {
		e.Map.Zones = append(e.Map.Zones, zone)
	}
	tx.Commit()
	return zone.ID, nil
}

// RemoveZone deletes a zone and removes it from tiles.
func (e *Editor) RemoveZone(id uint16) {
	tx := e.Begin("Remove zone", 0)
	tx.Meta()
	e.Map.Zones = slices.DeleteFunc(e.Map.Zones, func(zone otmap.Zone) bool { return zone.ID == id })
	e.Map.EachStored(func(pos otmap.Position, tile *otmap.Tile) {
		if slices.Contains(tile.Zones(), id) {
			edited := tx.Edit(pos)
			edited.Ext.Zones = slices.DeleteFunc(edited.Ext.Zones, func(zoneID uint16) bool { return zoneID == id })
			edited.Compact()
		}
	})
	tx.Commit()
}

// ImportOptions control merging another map.
type ImportOptions struct {
	DX int `json:"dx"`
	DY int `json:"dy"`
	DZ int `json:"dz"`
	// Houses: "merge" keeps ids (existing ids win), "renumber" gives
	// imported houses new ids, "skip" drops house tiles' house ids.
	Houses string `json:"houses"`
	// Replace replaces tiles that exist instead of merging items.
	Replace bool `json:"replace"`
	// Towns and waypoints are added when missing.
	Towns bool `json:"towns"`
}

// Import merges another map into this one.
func (e *Editor) Import(source *otmap.Map, options ImportOptions) (int, error) {
	tx := e.Begin("Import map", 0)
	tx.Meta()
	houseIDs := e.importHouses(source, options)
	if options.Towns {
		e.importTownsAndWaypoints(source, options)
	}
	var failed error
	source.Each(func(sourcePos otmap.Position, sourceTile *otmap.Tile) {
		targetPos, ok := sourcePos.Add(options.DX, options.DY, options.DZ)
		if !ok {
			failed = errors.New("some tiles were outside the map and were skipped")
			return
		}
		e.importTile(tx.Edit(targetPos), sourceTile, houseIDs, options)
	})
	changed := tx.Commit()
	return changed, failed
}

// importHouses adds the houses of source that are missing and returns the new
// id of each imported house id.
func (e *Editor) importHouses(source *otmap.Map, options ImportOptions) map[uint32]uint32 {
	houseIDs := map[uint32]uint32{}
	for _, house := range source.SortedHouses() {
		switch options.Houses {
		case importHousesSkip:
			continue
		case importHousesRenumber:
			houseIDs[house.ID] = e.firstFreeHouseID()
		default:
			houseIDs[house.ID] = house.ID
			if e.Map.Houses[house.ID] != nil {
				continue
			}
		}
		imported := house.Clone()
		imported.ID = houseIDs[house.ID]
		if exit, ok := imported.Exit.Add(options.DX, options.DY, options.DZ); ok && imported.Exit != (otmap.Position{}) {
			imported.Exit = exit
		}
		if e.Map.Houses[imported.ID] == nil {
			e.Map.Houses[imported.ID] = imported
		}
	}
	return houseIDs
}

// importTownsAndWaypoints adds the towns and waypoints of source that are
// missing.
func (e *Editor) importTownsAndWaypoints(source *otmap.Map, options ImportOptions) {
	for _, town := range source.SortedTowns() {
		if e.Map.Towns[town.ID] == nil {
			imported := *town
			if temple, ok := imported.Temple.Add(options.DX, options.DY, options.DZ); ok {
				imported.Temple = temple
			}
			e.Map.Towns[town.ID] = &imported
		}
	}
	for _, waypoint := range source.Waypoints {
		if e.Map.Waypoint(waypoint.Name) == nil {
			imported := *waypoint
			if pos, ok := imported.Pos.Add(options.DX, options.DY, options.DZ); ok {
				imported.Pos = pos
			}
			e.Map.Waypoints = append(e.Map.Waypoints, &imported)
		}
	}
}

// importTile replaces or merges target with a tile of the imported map.
func (e *Editor) importTile(target, sourceTile *otmap.Tile, houseIDs map[uint32]uint32, options ImportOptions) {
	imported := sourceTile.Clone()
	imported.SetSelected(false)
	if options.Houses == importHousesSkip {
		imported.HouseID = 0
	} else if imported.HouseID != 0 {
		imported.HouseID = houseIDs[imported.HouseID]
	}
	if options.Replace || target.Empty() {
		state := target.State
		*target = *imported
		target.State = state | otmap.StateModified
		return
	}
	if imported.Ground.ID != 0 {
		target.Ground = imported.Ground
	}
	for _, item := range imported.Items {
		e.addItem(target, item)
	}
	if imported.Ext != nil {
		ext := target.EnsureExt()
		ext.Creatures = append(ext.Creatures, imported.Ext.Creatures...)
		if imported.Ext.Spawn != nil {
			ext.Spawn = imported.Ext.Spawn
		}
		if imported.Ext.SpawnNPC != nil {
			ext.SpawnNPC = imported.Ext.SpawnNPC
		}
		ext.Zones = append(ext.Zones, imported.Ext.Zones...)
	}
	target.Flags |= imported.Flags
	if imported.HouseID != 0 {
		target.HouseID = imported.HouseID
	}
}

// MinimapColor returns the minimap color index of a tile (0 = none): the
// top item with a color, else the ground.
func (e *Editor) MinimapColor(tile *otmap.Tile) uint8 {
	for i := len(tile.Items) - 1; i >= 0; i-- {
		if itemType := e.itemType(tile.Items[i].ID); itemType != nil && itemType.MinimapColor != 0 {
			return uint8(itemType.MinimapColor)
		}
	}
	if itemType := e.itemType(tile.Ground.ID); itemType != nil {
		return uint8(itemType.MinimapColor)
	}
	return 0
}

// MinimapRGB converts a Tibia minimap color index to RGB.
func MinimapRGB(colorIndex uint8) (red, green, blue uint8) {
	if colorIndex >= minimapColorCount {
		return 0, 0, 0
	}
	return uint8(colorIndex/(minimapLevels*minimapLevels)%minimapLevels) * minimapLevelStep, uint8(colorIndex/minimapLevels%minimapLevels) * minimapLevelStep, uint8(colorIndex%minimapLevels) * minimapLevelStep
}
