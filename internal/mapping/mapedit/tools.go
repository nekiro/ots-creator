package mapedit

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// FindQuery searches items on the map.
type FindQuery struct {
	// IDs are server ids to find (empty: use Name or Kind).
	IDs []uint16 `json:"ids"`
	// Name matches item names (case-insensitive substring).
	Name string `json:"name"`
	// Kind is "unique", "action", "container", "writeable", "teleport",
	// "door", "duplicate" (same item twice on a tile) or "".
	Kind string `json:"kind"`
	// ActionID / UniqueID match exact ids (0 = any).
	ActionID uint16 `json:"actionId"`
	UniqueID uint16 `json:"uniqueId"`
	// InSelection limits the search to the selection.
	InSelection bool `json:"inSelection"`
	// Containers also searches container contents.
	Containers bool `json:"containers"`
	Limit      int  `json:"limit"`
}

// FindResult is one found item.
type FindResult struct {
	Pos  otmap.Position `json:"pos"`
	ID   uint16         `json:"id"`
	Name string         `json:"name"`
	// Detail is the matched attribute ("aid 1000").
	Detail string `json:"detail"`
}

// Find searches the map.
func (e *Editor) Find(query FindQuery) ([]FindResult, bool) {
	limit := query.Limit
	if limit <= 0 {
		limit = defaultFindLimit
	}
	lowerName := strings.ToLower(strings.TrimSpace(query.Name))
	var results []FindResult
	more := false
	visit := func(pos otmap.Position, tile *otmap.Tile) {
		if more {
			return
		}
		if query.Kind == "duplicate" {
			if duplicate, found := firstDuplicate(tile.Items); found {
				results = append(results, FindResult{Pos: pos, ID: duplicate.ID, Name: e.name(duplicate.ID), Detail: "duplicate"})
			}
			more = len(results) >= limit
			return
		}
		var walk func(items []otmap.Item)
		walk = func(items []otmap.Item) {
			for i := range items {
				// The limit may have been reached inside a container.
				if more {
					return
				}
				if ok, detail := e.matchFindItem(query, lowerName, &items[i]); ok {
					results = append(results, FindResult{Pos: pos, ID: items[i].ID, Name: e.name(items[i].ID), Detail: detail})
					if len(results) >= limit {
						more = true
						return
					}
				}
				if query.Containers && items[i].Ext != nil && len(items[i].Ext.Items) > 0 {
					walk(items[i].Ext.Items)
				}
			}
		}
		if tile.Ground.ID != 0 {
			walk([]otmap.Item{tile.Ground})
		}
		walk(tile.Items)
	}
	e.eachTile(query.InSelection, visit)
	return results, more
}

// firstDuplicate returns the first item whose id already appeared
// earlier in the list.
func firstDuplicate(items []otmap.Item) (otmap.Item, bool) {
	seen := map[uint16]bool{}
	for _, item := range items {
		if seen[item.ID] {
			return item, true
		}
		seen[item.ID] = true
	}
	return otmap.Item{}, false
}

// matchFindItem reports whether an item matches the query and returns
// the matched attribute. lowerName is the trimmed lower-case query name.
func (e *Editor) matchFindItem(query FindQuery, lowerName string, item *otmap.Item) (bool, string) {
	itemType := e.itemType(item.ID)
	switch {
	case len(query.IDs) > 0:
		if !slices.Contains(query.IDs, item.ID) {
			return false, ""
		}
	case lowerName != "":
		if itemType == nil || !strings.Contains(strings.ToLower(itemType.Name), lowerName) {
			return false, ""
		}
	}
	if query.ActionID != 0 && item.ActionID() != query.ActionID {
		return false, ""
	}
	if query.UniqueID != 0 && item.UniqueID() != query.UniqueID {
		return false, ""
	}
	ok, detail := findKindDetail(query.Kind, itemType, item)
	if !ok {
		return false, ""
	}
	if detail == "" && query.ActionID != 0 {
		detail = "aid " + strconv.Itoa(int(query.ActionID))
	}
	return true, detail
}

// findKindDetail checks an item against a FindQuery kind and returns the
// detail text for that kind (empty when there is nothing to show).
func findKindDetail(kind string, itemType *otitems.Type, item *otmap.Item) (bool, string) {
	switch kind {
	case "unique":
		if item.UniqueID() == 0 {
			return false, ""
		}
		return true, "uid " + strconv.Itoa(int(item.UniqueID()))
	case "action":
		if item.ActionID() == 0 {
			return false, ""
		}
		return true, "aid " + strconv.Itoa(int(item.ActionID()))
	case "container":
		if itemType == nil || !itemType.IsContainer() {
			return false, ""
		}
		contentCount := 0
		if item.Ext != nil {
			contentCount = len(item.Ext.Items)
		}
		return true, strconv.Itoa(contentCount) + " items"
	case "writeable":
		if itemType == nil || !(itemType.Flags.Has(otitems.FlagWritable) || itemType.Flags.Has(otitems.FlagReadable)) {
			return false, ""
		}
		if item.Ext != nil && item.Ext.Text != "" {
			return true, strconv.Quote(truncate(item.Ext.Text, findTextPreviewLen))
		}
	case "teleport":
		if item.Ext == nil || item.Ext.Dest == nil {
			return false, ""
		}
		dest := item.Ext.Dest
		return true, fmt.Sprintf("to %d, %d, %d", dest.X, dest.Y, dest.Z)
	case "door":
		if itemType == nil || !itemType.IsDoor() {
			return false, ""
		}
		if item.Ext != nil && item.Ext.DoorID != 0 {
			return true, "door id " + strconv.Itoa(int(item.Ext.DoorID))
		}
	}
	return true, ""
}

func truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "…"
}

func (e *Editor) name(id uint16) string {
	if itemType := e.itemType(id); itemType != nil {
		return itemType.DisplayName()
	}
	return "unknown item " + strconv.Itoa(int(id))
}

// eachTile visits the selection or the whole map.
func (e *Editor) eachTile(inSelection bool, fn func(pos otmap.Position, tile *otmap.Tile)) {
	if inSelection {
		for _, pos := range e.Selected() {
			if tile := e.Map.Tile(pos); tile != nil {
				fn(pos, tile)
			}
		}
		return
	}
	e.Map.Each(fn)
}

// rewriteItems rewrites items of the selection or the whole map,
// recursing into containers. fn returns the new item list for one list;
// items it sets to id 0 are removed.
func (e *Editor) rewriteItems(label string, inSelection bool, fn func(items []otmap.Item) ([]otmap.Item, bool)) int {
	tx := e.Begin(label, 0)
	count := 0
	var rewrite func(items []otmap.Item) ([]otmap.Item, bool)
	rewrite = func(items []otmap.Item) ([]otmap.Item, bool) {
		rewritten, changed := fn(items)
		for i := range rewritten {
			if rewritten[i].Ext != nil && len(rewritten[i].Ext.Items) > 0 {
				if contents, contentsChanged := rewrite(rewritten[i].Ext.Items); contentsChanged {
					if !changed {
						rewritten = slices.Clone(rewritten)
						changed = true
					}
					rewritten[i] = rewritten[i].Clone()
					rewritten[i].Ext.Items = slices.DeleteFunc(slices.Clone(contents), func(item otmap.Item) bool { return item.ID == 0 })
					rewritten[i].Compact()
				}
			}
		}
		return rewritten, changed
	}
	e.eachTile(inSelection, func(pos otmap.Position, tile *otmap.Tile) {
		items := append([]otmap.Item{tile.Ground}, tile.Items...)
		rewritten, changed := rewrite(items)
		if !changed {
			return
		}
		count++
		editTile := tx.Edit(pos)
		editTile.Ground = rewritten[0]
		if editTile.Ground.ID == 0 {
			editTile.Ground = otmap.Item{}
		}
		// rewritten is a new list (items is built above), so it is reused.
		editTile.Items = slices.DeleteFunc(rewritten[1:], func(item otmap.Item) bool { return item.ID == 0 })
	})
	tx.Commit()
	return count
}

// ReplaceItems replaces items with one id by another (to 0 removes them).
func (e *Editor) ReplaceItems(pairs map[uint16]uint16, inSelection bool) int {
	replaced := 0
	label := "Replace items"
	if len(pairs) == 1 {
		for _, toID := range pairs {
			if toID == 0 {
				label = "Remove items"
			}
		}
	}
	e.rewriteItems(label, inSelection, func(items []otmap.Item) ([]otmap.Item, bool) {
		var rewritten []otmap.Item
		changed := false
		for i, item := range items {
			toID, ok := pairs[item.ID]
			if !ok || item.ID == 0 {
				continue
			}
			if !changed {
				rewritten = slices.Clone(items)
				changed = true
			}
			replaced++
			rewritten[i].ID = toID
		}
		if !changed {
			return items, false
		}
		return rewritten, true
	})
	return replaced
}

// removeItemsWhere clears (sets id 0) every item for which shouldRemove
// is true and returns how many were removed.
func (e *Editor) removeItemsWhere(label string, inSelection bool, shouldRemove func(item otmap.Item) bool) int {
	removed := 0
	e.rewriteItems(label, inSelection, func(items []otmap.Item) ([]otmap.Item, bool) {
		changed := false
		rewritten := slices.Clone(items)
		for i, item := range items {
			if shouldRemove(item) {
				rewritten[i].ID = 0
				removed++
				changed = true
			}
		}
		return rewritten, changed
	})
	return removed
}

// RemoveCorpses removes corpse items.
func (e *Editor) RemoveCorpses(inSelection bool) int {
	return e.removeItemsWhere("Remove corpses", inSelection, func(item otmap.Item) bool {
		itemType := e.itemType(item.ID)
		return itemType != nil && itemType.Flags.Has(otitems.FlagCorpse) && !item.Complex()
	})
}

// RemoveInvalid removes items the item database does not know.
func (e *Editor) RemoveInvalid(inSelection bool) int {
	return e.removeItemsWhere("Remove invalid items", inSelection, func(item otmap.Item) bool {
		return item.ID != 0 && e.itemType(item.ID) == nil
	})
}

// RemoveUnreachable removes tiles no player can see: tiles farther than
// the client view (9x7 tiles) from any walkable tile within two floors.
func (e *Editor) RemoveUnreachable() int {
	walkable := func(x, y, z int) bool {
		tile := e.Map.At(x, y, z)
		return tile != nil && !e.blocking(tile)
	}
	var unreachable []otmap.Position
	e.Map.Each(func(pos otmap.Position, _ *otmap.Tile) {
		x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
		for floor := max(z-clientViewFloorsRange, 0); floor <= min(z+clientViewFloorsRange, otmap.MaxFloor); floor++ {
			for dy := -clientViewRangeY; dy <= clientViewRangeY; dy++ {
				for dx := -clientViewRangeX; dx <= clientViewRangeX; dx++ {
					if walkable(x+dx, y+dy, floor) {
						return
					}
				}
			}
		}
		unreachable = append(unreachable, pos)
	})
	if len(unreachable) == 0 {
		return 0
	}
	tx := e.Begin("Remove unreachable tiles", 0)
	for _, pos := range unreachable {
		tile := tx.Edit(pos)
		*tile = otmap.Tile{}
	}
	return tx.Commit()
}

// RemoveEmptySpawns removes spawns without creatures in their area.
func (e *Editor) RemoveEmptySpawns() int {
	type spawnRef struct {
		pos otmap.Position
		npc bool
	}
	var empty []spawnRef
	e.Map.EachStored(func(pos otmap.Position, tile *otmap.Tile) {
		for _, npc := range []bool{false, true} {
			spawn := tile.Spawn()
			if npc {
				spawn = tile.SpawnNPC()
			}
			if spawn == nil {
				continue
			}
			found := false
			radius := int(spawn.Radius)
			e.Map.EachIn(int(pos.X)-radius, int(pos.Y)-radius, int(pos.X)+radius, int(pos.Y)+radius, int(pos.Z), int(pos.Z), func(_ otmap.Position, areaTile *otmap.Tile) {
				if len(areaTile.Creatures()) > 0 {
					found = true
				}
			})
			if !found {
				empty = append(empty, spawnRef{pos, npc})
			}
		}
	})
	if len(empty) == 0 {
		return 0
	}
	tx := e.Begin("Remove empty spawns", 0)
	for _, spawn := range empty {
		tile := tx.Edit(spawn.pos)
		if spawn.npc {
			tile.Ext.SpawnNPC = nil
		} else {
			tile.Ext.Spawn = nil
		}
		tile.Compact()
	}
	tx.Commit()
	return len(empty)
}

// ClearInvalidHouses clears house ids of tiles whose house does not exist.
func (e *Editor) ClearInvalidHouses() int {
	tx := e.Begin("Clear invalid houses", 0)
	cleared := 0
	e.Map.EachStored(func(pos otmap.Position, tile *otmap.Tile) {
		if tile.HouseID != 0 && e.Map.Houses[tile.HouseID] == nil {
			editTile := tx.Edit(pos)
			editTile.HouseID = 0
			cleared++
		}
	})
	tx.Commit()
	return cleared
}

// ClearModified clears the modified state of all tiles (not undoable).
func (e *Editor) ClearModified() {
	e.Map.ClearModified()
	for _, chunk := range e.Map.Chunks() {
		e.dirty[otmap.MakeKey(chunk.X, chunk.Y, chunk.Z)] = true
	}
}

// allTilePositions lists the positions of all tiles of the map.
func (e *Editor) allTilePositions() []otmap.Position {
	var positions []otmap.Position
	e.Map.Each(func(pos otmap.Position, _ *otmap.Tile) { positions = append(positions, pos) })
	return positions
}

// BorderizeMap redoes all borders and walls.
func (e *Editor) BorderizeMap() int {
	return e.borderizePositions("Borderize map", e.allTilePositions())
}

// RandomizeMap rerolls all grounds.
func (e *Editor) RandomizeMap() int {
	return e.randomizePositions("Randomize map", e.allTilePositions())
}

// borderizePositions redoes borders and walls of the tiles at positions as
// one undo step.
func (e *Editor) borderizePositions(label string, positions []otmap.Position) int {
	tx := e.Begin(label, 0)
	for _, pos := range positions {
		e.reborder(tx, pos, true)
		e.fitWalls(tx, pos)
	}
	return tx.Commit()
}

// randomizePositions rerolls the grounds of the tiles at positions as one
// undo step.
func (e *Editor) randomizePositions(label string, positions []otmap.Position) int {
	tx := e.Begin(label, 0)
	for _, pos := range positions {
		e.randomize(tx, pos)
	}
	return tx.Commit()
}

// Statistics summarizes the map.
type Statistics struct {
	Tiles          int   `json:"tiles"`
	Items          int   `json:"items"`
	Walkable       int   `json:"walkable"`
	Blocking       int   `json:"blocking"`
	PZ             int   `json:"pz"`
	HouseTiles     int   `json:"houseTiles"`
	Houses         int   `json:"houses"`
	Towns          int   `json:"towns"`
	Spawns         int   `json:"spawns"`
	Monsters       int   `json:"monsters"`
	NPCs           int   `json:"npcs"`
	Containers     int   `json:"containers"`
	ContainerItems int   `json:"containerItems"`
	ActionIDs      int   `json:"actionIds"`
	UniqueIDs      int   `json:"uniqueIds"`
	Teleports      int   `json:"teleports"`
	Waypoints      int   `json:"waypoints"`
	Floors         []int `json:"floors"`
	MinX           int   `json:"minX"`
	MinY           int   `json:"minY"`
	MaxX           int   `json:"maxX"`
	MaxY           int   `json:"maxY"`
}

// Stats computes map statistics.
func (e *Editor) Stats() Statistics {
	stats := Statistics{Houses: len(e.Map.Houses), Towns: len(e.Map.Towns), Waypoints: len(e.Map.Waypoints), Floors: make([]int, otmap.MaxFloor+1)}
	stats.MinX, stats.MinY, stats.MaxX, stats.MaxY, _ = e.Map.Bounds()
	e.Map.Each(func(pos otmap.Position, tile *otmap.Tile) {
		e.countTileStats(&stats, pos, tile)
	})
	return stats
}

// countTileStats adds one tile and everything on it to stats.
func (e *Editor) countTileStats(stats *Statistics, pos otmap.Position, tile *otmap.Tile) {
	stats.Tiles++
	stats.Floors[pos.Z]++
	e.countItemStats(stats, []otmap.Item{tile.Ground}, false)
	e.countItemStats(stats, tile.Items, false)
	if e.blocking(tile) {
		stats.Blocking++
	} else {
		stats.Walkable++
	}
	if tile.Flags&otmap.FlagPZ != 0 {
		stats.PZ++
	}
	if tile.HouseID != 0 {
		stats.HouseTiles++
	}
	if tile.Spawn() != nil {
		stats.Spawns++
	}
	if tile.SpawnNPC() != nil {
		stats.Spawns++
	}
	for _, creature := range tile.Creatures() {
		if creature.NPC {
			stats.NPCs++
		} else {
			stats.Monsters++
		}
	}
}

// countItemStats adds items and their container contents to stats.
// nested marks items inside a container.
func (e *Editor) countItemStats(stats *Statistics, items []otmap.Item, nested bool) {
	for _, item := range items {
		if item.ID == 0 {
			continue
		}
		stats.Items++
		if nested {
			stats.ContainerItems++
		}
		if item.ActionID() != 0 {
			stats.ActionIDs++
		}
		if item.UniqueID() != 0 {
			stats.UniqueIDs++
		}
		if item.Ext != nil && item.Ext.Dest != nil {
			stats.Teleports++
		}
		if itemType := e.itemType(item.ID); itemType != nil && itemType.IsContainer() {
			stats.Containers++
		}
		if item.Ext != nil {
			e.countItemStats(stats, item.Ext.Items, true)
		}
	}
}

// ItemPatch changes attributes of an item. Nil fields stay unchanged.
type ItemPatch struct {
	ID          *uint16         `json:"id,omitempty"`
	Subtype     *uint16         `json:"subtype,omitempty"`
	ActionID    *uint16         `json:"actionId,omitempty"`
	UniqueID    *uint16         `json:"uniqueId,omitempty"`
	Text        *string         `json:"text,omitempty"`
	Description *string         `json:"description,omitempty"`
	Dest        *otmap.Position `json:"dest,omitempty"`
	ClearDest   bool            `json:"clearDest,omitempty"`
	DepotID     *uint16         `json:"depotId,omitempty"`
	DoorID      *uint8          `json:"doorId,omitempty"`
	Attrs       *[]otmap.Attr   `json:"attrs,omitempty"`
}

// ItemRef addresses an item: Index -1 is the ground, Path walks into
// container contents.
type ItemRef struct {
	Pos   otmap.Position `json:"pos"`
	Index int            `json:"index"`
	Path  []int          `json:"path"`
}

// itemAt resolves a reference on a writable tile.
func itemAt(tile *otmap.Tile, ref ItemRef) (*otmap.Item, []otmap.Item, int, error) {
	var list []otmap.Item
	var item *otmap.Item
	if ref.Index < 0 {
		if tile.Ground.ID == 0 {
			return nil, nil, 0, ErrNoItem
		}
		item = &tile.Ground
	} else {
		if ref.Index >= len(tile.Items) {
			return nil, nil, 0, ErrNoItem
		}
		list = tile.Items
		item = &tile.Items[ref.Index]
	}
	index := ref.Index
	for _, childIndex := range ref.Path {
		if item.Ext == nil || childIndex < 0 || childIndex >= len(item.Ext.Items) {
			return nil, nil, 0, ErrNoItem
		}
		list = item.Ext.Items
		index = childIndex
		item = &item.Ext.Items[childIndex]
	}
	return item, list, index, nil
}

// UpdateItem applies a patch to an item.
func (e *Editor) UpdateItem(ref ItemRef, patch ItemPatch) error {
	if e.Map.Tile(ref.Pos) == nil {
		return ErrNoItem
	}
	tx := e.Begin("Edit item", 0)
	defer tx.Commit()
	tile := tx.Edit(ref.Pos)
	item, _, _, err := itemAt(tile, ref)
	if err != nil {
		return err
	}
	if patch.ID != nil {
		if e.itemType(*patch.ID) == nil {
			return fmt.Errorf("unknown item %d", *patch.ID)
		}
		item.ID = *patch.ID
	}
	if patch.Subtype != nil {
		item.Subtype = *patch.Subtype
	}
	ext := item.Ensure()
	if patch.ActionID != nil {
		ext.ActionID = *patch.ActionID
	}
	if patch.UniqueID != nil {
		ext.UniqueID = *patch.UniqueID
	}
	if patch.Text != nil {
		ext.Text = *patch.Text
	}
	if patch.Description != nil {
		ext.Description = *patch.Description
	}
	if patch.Dest != nil {
		dest := *patch.Dest
		ext.Dest = &dest
	}
	if patch.ClearDest {
		ext.Dest = nil
	}
	if patch.DepotID != nil {
		ext.DepotID = *patch.DepotID
	}
	if patch.DoorID != nil {
		ext.DoorID = *patch.DoorID
	}
	if patch.Attrs != nil {
		ext.Attrs = slices.Clone(*patch.Attrs)
	}
	item.Compact()
	return nil
}

// RemoveItem deletes an item (or container content).
func (e *Editor) RemoveItem(ref ItemRef) error {
	if e.Map.Tile(ref.Pos) == nil {
		return ErrNoItem
	}
	tx := e.Begin("Remove item", 0)
	defer tx.Commit()
	tile := tx.Edit(ref.Pos)
	if len(ref.Path) == 0 {
		if ref.Index < 0 {
			tile.Ground = otmap.Item{}
			return nil
		}
		if ref.Index >= len(tile.Items) {
			return ErrNoItem
		}
		tile.Items = slices.Delete(tile.Items, ref.Index, ref.Index+1)
		return nil
	}
	parentRef := ItemRef{Pos: ref.Pos, Index: ref.Index, Path: ref.Path[:len(ref.Path)-1]}
	parent, _, _, err := itemAt(tile, parentRef)
	if err != nil {
		return err
	}
	childIndex := ref.Path[len(ref.Path)-1]
	if parent.Ext == nil || childIndex >= len(parent.Ext.Items) {
		return ErrNoItem
	}
	parent.Ext.Items = slices.Delete(parent.Ext.Items, childIndex, childIndex+1)
	parent.Compact()
	return nil
}

// AddToContainer puts a new item into a container.
func (e *Editor) AddToContainer(ref ItemRef, id uint16) error {
	if e.itemType(id) == nil {
		return fmt.Errorf("unknown item %d", id)
	}
	if e.Map.Tile(ref.Pos) == nil {
		return ErrNoItem
	}
	tx := e.Begin("Add to container", 0)
	defer tx.Commit()
	tile := tx.Edit(ref.Pos)
	container, _, _, err := itemAt(tile, ref)
	if err != nil {
		return err
	}
	if itemType := e.itemType(container.ID); itemType == nil || !itemType.IsContainer() {
		return errors.New("the item is not a container")
	}
	ext := container.Ensure()
	ext.Items = append(ext.Items, e.newItem(id))
	return nil
}

// MoveItem changes the stack position of an item on a tile.
func (e *Editor) MoveItem(pos otmap.Position, from, to int) error {
	current := e.Map.Tile(pos)
	if current == nil || from < 0 || from >= len(current.Items) || to < 0 || to >= len(current.Items) || from == to {
		return nil
	}
	tx := e.Begin("Reorder items", 0)
	defer tx.Commit()
	tile := tx.Edit(pos)
	moved := tile.Items[from]
	tile.Items = slices.Delete(tile.Items, from, from+1)
	tile.Items = slices.Insert(tile.Items, to, moved)
	return nil
}

// AddItem puts a new item on a tile like a raw brush.
func (e *Editor) AddItem(pos otmap.Position, id uint16) error {
	if e.itemType(id) == nil {
		return fmt.Errorf("unknown item %d", id)
	}
	tx := e.Begin("Add item", 0)
	defer tx.Commit()
	e.addItem(tx.Edit(pos), e.newItem(id))
	return nil
}

// RotateItem turns an item into its "rotateTo" item.
func (e *Editor) RotateItem(ref ItemRef) error {
	if e.Map.Tile(ref.Pos) == nil {
		return ErrNoItem
	}
	tx := e.Begin("Rotate item", 0)
	defer tx.Commit()
	tile := tx.Edit(ref.Pos)
	item, _, _, err := itemAt(tile, ref)
	if err != nil {
		return err
	}
	itemType := e.itemType(item.ID)
	if itemType == nil || itemType.RotateTo == 0 {
		return errors.New("the item cannot be rotated")
	}
	item.ID = itemType.RotateTo
	return nil
}

// SwitchDoor opens or closes a door.
func (e *Editor) SwitchDoor(pos otmap.Position, index int) error {
	current := e.Map.Tile(pos)
	if current == nil || index < 0 || index >= len(current.Items) {
		return ErrNoItem
	}
	tx := e.Begin("Switch door", 0)
	defer tx.Commit()
	if !e.switchDoor(tx.Edit(pos), index) {
		return errors.New("no door brush knows this door")
	}
	return nil
}

// TilePatch changes tile properties.
type TilePatch struct {
	Flags   *uint32   `json:"flags,omitempty"`
	HouseID *uint32   `json:"houseId,omitempty"`
	Zones   *[]uint16 `json:"zones,omitempty"`
}

// UpdateTile applies a tile patch.
func (e *Editor) UpdateTile(pos otmap.Position, patch TilePatch) {
	tx := e.Begin("Edit tile", 0)
	tile := tx.Edit(pos)
	if patch.Flags != nil {
		tile.Flags = *patch.Flags
	}
	if patch.HouseID != nil {
		tile.HouseID = *patch.HouseID
	}
	if patch.Zones != nil {
		tile.EnsureExt().Zones = slices.Clone(*patch.Zones)
		tile.Compact()
	}
	tx.Commit()
}

// CreaturePatch changes a creature (Remove deletes it).
type CreaturePatch struct {
	SpawnTime *int32  `json:"spawnTime,omitempty"`
	Direction *uint8  `json:"direction,omitempty"`
	Weight    *uint8  `json:"weight,omitempty"`
	Name      *string `json:"name,omitempty"`
	Remove    bool    `json:"remove,omitempty"`
}

// UpdateCreature edits a creature of a tile.
func (e *Editor) UpdateCreature(pos otmap.Position, index int, patch CreaturePatch) error {
	current := e.Map.Tile(pos)
	if current == nil || index < 0 || index >= len(current.Creatures()) {
		return errors.New("creature not found")
	}
	tx := e.Begin("Edit creature", 0)
	defer tx.Commit()
	tile := tx.Edit(pos)
	creature := &tile.Ext.Creatures[index]
	if patch.Remove {
		tile.Ext.Creatures = slices.Delete(tile.Ext.Creatures, index, index+1)
		tile.Compact()
		return nil
	}
	if patch.SpawnTime != nil {
		creature.SpawnTime = *patch.SpawnTime
	}
	if patch.Direction != nil {
		creature.Direction = *patch.Direction % creatureDirectionCount
	}
	if patch.Weight != nil {
		creature.Weight = *patch.Weight
	}
	if patch.Name != nil && *patch.Name != "" {
		creature.Name = *patch.Name
	}
	return nil
}

// UpdateSpawn sets the radius of a spawn (0 removes it).
func (e *Editor) UpdateSpawn(pos otmap.Position, npc bool, radius int32) error {
	current := e.Map.Tile(pos)
	if current == nil || (npc && current.SpawnNPC() == nil) || (!npc && current.Spawn() == nil) {
		return errors.New("spawn not found")
	}
	tx := e.Begin("Edit spawn", 0)
	defer tx.Commit()
	tile := tx.Edit(pos)
	spawn := &tile.Ext.Spawn
	if npc {
		spawn = &tile.Ext.SpawnNPC
	}
	if radius <= 0 {
		*spawn = nil
		tile.Compact()
		return nil
	}
	(*spawn).Radius = min(radius, maxSpawnRadius)
	return nil
}
