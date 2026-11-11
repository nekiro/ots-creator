package mapedit

import (
	"errors"
	"reflect"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// A shared map is edited by several editors at once (see internal/collab):
// every change made here goes to the others as the whole new state of the
// changed tiles, and their changes come back through ApplyRemote. Selection
// and the editor-only tile state are never shared.

// ErrShared refuses changes that cannot be shared with other editors.
var ErrShared = errors.New("not possible while the map is shared with other editors")

// TileState is the whole state of one tile; a nil Tile removes the tile.
type TileState struct {
	Pos  otmap.Position
	Tile *otmap.Tile
}

// MetaData is the shared part of the map data outside tiles: size,
// description, towns, houses, waypoints and zones. The file names stay with
// each editor.
type MetaData struct {
	Width       uint16
	Height      uint16
	Description string
	SpawnFormat otmap.SpawnFormat
	Towns       []otmap.Town
	Houses      []otmap.House
	Waypoints   []otmap.Waypoint
	Zones       []otmap.Zone
}

// Commit is one change of the map made in this editor: an edit, an undo or
// a redo.
type Commit struct {
	Label string
	Tiles []TileState
	Meta  *MetaData
}

// Observe calls fn with every change made in this editor (nil stops). It
// runs while the map is locked: fn must not block.
func (e *Editor) Observe(fn func(Commit)) { e.observer = fn }

// Shared reports whether the map is shared with other editors.
func (e *Editor) Shared() bool { return e.observer != nil }

// ApplyRemote stores the tiles and the map data changed by another editor.
// The change is not in the undo history; undoing an own change later keeps
// the tiles changed by others since.
func (e *Editor) ApplyRemote(tiles []TileState, meta *MetaData) {
	for _, state := range tiles {
		tile := state.Tile
		if tile != nil {
			tile.State = otmap.StateModified
		}
		e.Map.SetTile(state.Pos, tile)
		e.markDirty(state.Pos)
		e.syncSelection(state.Pos)
	}
	if meta != nil {
		meta.apply(e.Map)
		e.metaDirty = true
	}
	if len(tiles) > 0 || meta != nil {
		e.foreign = true
	}
}

// SharedMeta returns the shared map data of the map.
func (e *Editor) SharedMeta() *MetaData { return metaDataOf(e.Map) }

func (e *Editor) notify(label string, tiles []TileState, meta *MetaData) {
	if e.observer == nil || (len(tiles) == 0 && meta == nil) {
		return
	}
	e.observer(Commit{Label: label, Tiles: tiles, Meta: meta})
}

// sharedState returns the state of the tile to send to other editors: a
// copy without selection and editor state.
func sharedState(pos otmap.Position, tile *otmap.Tile) TileState {
	if tile == nil || tile.Empty() {
		return TileState{Pos: pos}
	}
	shared := tile.Clone()
	shared.State = 0
	shared.Ground.Selected = false
	for i := range shared.Items {
		shared.Items[i].Selected = false
	}
	if shared.Ext != nil {
		for i := range shared.Ext.Creatures {
			shared.Ext.Creatures[i].Selected = false
		}
		if shared.Ext.Spawn != nil {
			shared.Ext.Spawn.Selected = false
		}
		if shared.Ext.SpawnNPC != nil {
			shared.Ext.SpawnNPC.Selected = false
		}
	}
	return TileState{Pos: pos, Tile: shared}
}

// SameTile reports whether two tiles hold the same things, ignoring the
// selection and the editor state.
func SameTile(left, right *otmap.Tile) bool {
	leftEmpty, rightEmpty := left.Empty(), right.Empty()
	if leftEmpty || rightEmpty {
		return leftEmpty == rightEmpty
	}
	if left.Flags != right.Flags || left.HouseID != right.HouseID || !sameItem(left.Ground, right.Ground) || len(left.Items) != len(right.Items) {
		return false
	}
	for i := range left.Items {
		if !sameItem(left.Items[i], right.Items[i]) {
			return false
		}
	}
	return sameExt(left.Ext, right.Ext)
}

func sameItem(left, right otmap.Item) bool {
	if left.ID != right.ID || left.Subtype != right.Subtype {
		return false
	}
	if left.Ext == nil || right.Ext == nil {
		return left.Complex() == right.Complex()
	}
	leftExt, rightExt := *left.Ext, *right.Ext
	leftItems, rightItems := leftExt.Items, rightExt.Items
	leftExt.Items, rightExt.Items = nil, nil
	if !reflect.DeepEqual(leftExt, rightExt) || len(leftItems) != len(rightItems) {
		return false
	}
	for i := range leftItems {
		if !sameItem(leftItems[i], rightItems[i]) {
			return false
		}
	}
	return true
}

func sameExt(left, right *otmap.TileExt) bool {
	var leftCreatures, rightCreatures []otmap.Creature
	var leftSpawn, rightSpawn, leftNPC, rightNPC *otmap.Spawn
	var leftZones, rightZones []uint16
	if left != nil {
		leftCreatures, leftSpawn, leftNPC, leftZones = left.Creatures, left.Spawn, left.SpawnNPC, left.Zones
	}
	if right != nil {
		rightCreatures, rightSpawn, rightNPC, rightZones = right.Creatures, right.Spawn, right.SpawnNPC, right.Zones
	}
	sameCreature := func(a, b otmap.Creature) bool {
		a.Selected, b.Selected = false, false
		return a == b
	}
	return slices.EqualFunc(leftCreatures, rightCreatures, sameCreature) && sameSpawn(leftSpawn, rightSpawn) &&
		sameSpawn(leftNPC, rightNPC) && slices.Equal(leftZones, rightZones)
}

func sameSpawn(left, right *otmap.Spawn) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Radius == right.Radius
}

func metaDataOf(gameMap *otmap.Map) *MetaData {
	meta := &MetaData{
		Width: gameMap.Width, Height: gameMap.Height, Description: gameMap.Description, SpawnFormat: gameMap.SpawnFormat,
		Zones: slices.Clone(gameMap.Zones),
	}
	for _, town := range gameMap.SortedTowns() {
		meta.Towns = append(meta.Towns, *town)
	}
	for _, house := range gameMap.SortedHouses() {
		meta.Houses = append(meta.Houses, *house)
	}
	for _, waypoint := range gameMap.Waypoints {
		meta.Waypoints = append(meta.Waypoints, *waypoint)
	}
	return meta
}

func (m *MetaData) apply(gameMap *otmap.Map) {
	gameMap.Width, gameMap.Height, gameMap.Description = m.Width, m.Height, m.Description
	if m.SpawnFormat != "" {
		gameMap.SpawnFormat = m.SpawnFormat
	}
	gameMap.Towns = make(map[uint32]*otmap.Town, len(m.Towns))
	for _, town := range m.Towns {
		gameMap.Towns[town.ID] = &town
	}
	gameMap.Houses = make(map[uint32]*otmap.House, len(m.Houses))
	for _, house := range m.Houses {
		gameMap.Houses[house.ID] = &house
	}
	gameMap.Waypoints = make([]*otmap.Waypoint, len(m.Waypoints))
	for i := range m.Waypoints {
		waypoint := m.Waypoints[i]
		gameMap.Waypoints[i] = &waypoint
	}
	gameMap.Zones = slices.Clone(m.Zones)
}
