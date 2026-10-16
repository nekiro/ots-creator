package mapedit

import (
	"errors"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// floorRange returns the floors a selection mode covers from floor z.
func floorRange(z int, mode string) (int, int) {
	switch mode {
	case FloorsLower:
		if z <= otmap.GroundFloor {
			return z, otmap.GroundFloor
		}
		return z, min(otmap.MaxFloor, z+lowerFloorSpan)
	case FloorsVisible:
		if z <= otmap.GroundFloor {
			return 0, otmap.GroundFloor
		}
		return z, min(otmap.MaxFloor, z+lowerFloorSpan)
	}
	return z, z
}

// selection tracking: positions of tiles with something selected.
func (e *Editor) syncSelection(pos otmap.Position) {
	if e.selectedPositions == nil {
		e.selectedPositions = map[otmap.Position]struct{}{}
	}
	if e.peekPos(pos).Selected() {
		e.selectedPositions[pos] = struct{}{}
	} else {
		delete(e.selectedPositions, pos)
	}
}

// RebuildSelection scans the map for selected tiles (after loading).
func (e *Editor) RebuildSelection() {
	e.selectedPositions = map[otmap.Position]struct{}{}
	e.Map.EachStored(func(pos otmap.Position, tile *otmap.Tile) {
		if tile.Selected() {
			e.selectedPositions[pos] = struct{}{}
		}
	})
}

// Selected returns the positions with something selected, sorted.
func (e *Editor) Selected() []otmap.Position {
	positions := make([]otmap.Position, 0, len(e.selectedPositions))
	for pos := range e.selectedPositions {
		positions = append(positions, pos)
	}
	slices.SortFunc(positions, otmap.ComparePositions)
	return positions
}

// SelectionCount returns the number of tiles with a selection.
func (e *Editor) SelectionCount() int { return len(e.selectedPositions) }

// SelectionBounds returns the selected area.
func (e *Editor) SelectionBounds() (lowest, highest otmap.Position, ok bool) {
	first := true
	for pos := range e.selectedPositions {
		if first {
			lowest, highest, first = pos, pos, false
			continue
		}
		lowest.X, lowest.Y, lowest.Z = min(lowest.X, pos.X), min(lowest.Y, pos.Y), min(lowest.Z, pos.Z)
		highest.X, highest.Y, highest.Z = max(highest.X, pos.X), max(highest.Y, pos.Y), max(highest.Z, pos.Z)
	}
	return lowest, highest, !first
}

// ClearSelection deselects everything.
func (e *Editor) ClearSelection() {
	if len(e.selectedPositions) == 0 {
		return
	}
	tx := e.BeginSelection()
	e.deselectAll(tx)
	tx.Commit()
}

// deselectAll deselects every selected tile within tx.
func (e *Editor) deselectAll(tx *Tx) {
	for _, pos := range e.Selected() {
		tx.Edit(pos).SetSelected(false)
	}
}

// SelectRect selects the tiles of a rectangle. add keeps the selection,
// remove deselects instead. compensate shifts lower floors like they are
// drawn (one tile right and down per floor).
func (e *Editor) SelectRect(area Rect, floors string, add, remove, compensate bool) {
	tx := e.BeginSelection()
	if !add && !remove {
		e.deselectAll(tx)
	}
	x0, y0, x1, y1 := area.bounds()
	z := int(area.From.Z)
	firstFloor, lastFloor := floorRange(z, floors)
	for floor := firstFloor; floor <= lastFloor; floor++ {
		shift := 0
		if compensate {
			shift = floor - z
		}
		e.Map.EachIn(x0+shift, y0+shift, x1+shift, y1+shift, floor, floor, func(pos otmap.Position, tile *otmap.Tile) {
			if tile.Selected() == !remove && (remove || allSelected(tile)) {
				return
			}
			tx.Edit(pos).SetSelected(!remove)
		})
	}
	tx.Commit()
}

// allSelected reports whether everything on a tile is already selected.
func allSelected(tile *otmap.Tile) bool {
	fullySelected := tile.Clone()
	fullySelected.SetSelected(true)
	return equalSelection(fullySelected, tile)
}

// equalSelection reports whether two tiles have the same ground and items
// selected.
func equalSelection(left, right *otmap.Tile) bool {
	if left.Ground.Selected != right.Ground.Selected || len(left.Items) != len(right.Items) {
		return false
	}
	for i := range left.Items {
		if left.Items[i].Selected != right.Items[i].Selected {
			return false
		}
	}
	return true
}

// SelectAt selects the top thing of a tile: a creature, the spawn, or the
// top item. toggle adds or removes it from the selection instead of
// replacing it. It returns what was selected ("creature", "spawn", "item"
// or "").
func (e *Editor) SelectAt(target otmap.Position, toggle bool, showCreatures, showSpawns bool) string {
	tile := e.Map.Tile(target)
	tx := e.BeginSelection()
	defer tx.Commit()
	if !toggle {
		for _, other := range e.Selected() {
			if other != target {
				tx.Edit(other).SetSelected(false)
			}
		}
	}
	if tile == nil {
		return ""
	}
	edited := tx.Edit(target)
	if !toggle {
		edited.SetSelected(false)
	}
	if showCreatures && len(edited.Creatures()) > 0 {
		creature := &edited.Ext.Creatures[len(edited.Ext.Creatures)-1]
		creature.Selected = !toggle || !creature.Selected
		return "creature"
	}
	if showSpawns && edited.Spawn() != nil {
		edited.Ext.Spawn.Selected = !toggle || !edited.Ext.Spawn.Selected
		return "spawn"
	}
	if showSpawns && edited.SpawnNPC() != nil {
		edited.Ext.SpawnNPC.Selected = !toggle || !edited.Ext.SpawnNPC.Selected
		return "spawn"
	}
	if itemCount := len(edited.Items); itemCount > 0 {
		topItem := &edited.Items[itemCount-1]
		topItem.Selected = !toggle || !topItem.Selected
		return "item"
	}
	if edited.Ground.ID != 0 {
		edited.Ground.Selected = !toggle || !edited.Ground.Selected
		return "item"
	}
	return ""
}

// SelectItem selects one item of a tile (index -1 is the ground).
func (e *Editor) SelectItem(pos otmap.Position, index int, add bool) {
	tx := e.BeginSelection()
	defer tx.Commit()
	if !add {
		e.deselectAll(tx)
	}
	if e.Map.Tile(pos) == nil {
		return
	}
	edited := tx.Edit(pos)
	if index < 0 {
		edited.Ground.Selected = edited.Ground.ID != 0
	} else if index < len(edited.Items) {
		edited.Items[index].Selected = true
	}
}

// SelectTiles selects whole tiles (find results).
func (e *Editor) SelectTiles(positions []otmap.Position, add bool) {
	tx := e.BeginSelection()
	if !add {
		e.deselectAll(tx)
	}
	for _, pos := range positions {
		if e.Map.Tile(pos) != nil {
			tx.Edit(pos).SetSelected(true)
		}
	}
	tx.Commit()
}

// ClipTile is one tile of the clipboard: what was selected on it.
type ClipTile struct {
	DX        int              `json:"dx"`
	DY        int              `json:"dy"`
	DZ        int              `json:"dz"`
	Ground    *otmap.Item      `json:"ground,omitempty"`
	Items     []otmap.Item     `json:"items,omitempty"`
	Creatures []otmap.Creature `json:"creatures,omitempty"`
	Spawn     *otmap.Spawn     `json:"spawn,omitempty"`
	SpawnNPC  *otmap.Spawn     `json:"spawnNpc,omitempty"`
	HouseID   uint32           `json:"houseId,omitempty"`
	Flags     uint32           `json:"flags,omitempty"`
}

// Clip is copied map content.
type Clip struct {
	Tiles  []ClipTile `json:"tiles"`
	Width  int        `json:"width"`
	Height int        `json:"height"`
}

// extract returns the selected parts of a tile; take also removes them.
func extract(tile *otmap.Tile, take bool) ClipTile {
	var clipTile ClipTile
	if tile.Ground.ID != 0 && tile.Ground.Selected {
		ground := tile.Ground.Clone()
		ground.Selected = false
		clipTile.Ground = &ground
		if take {
			tile.Ground = otmap.Item{}
		}
	}
	kept := tile.Items[:0:0]
	for _, item := range tile.Items {
		if item.Selected {
			copied := item.Clone()
			copied.Selected = false
			clipTile.Items = append(clipTile.Items, copied)
			if take {
				continue
			}
		}
		kept = append(kept, item)
	}
	if take {
		tile.Items = kept
	}
	if tile.Ext != nil {
		extractExtras(tile, &clipTile, take)
	}
	// A whole tile selection carries the house and flags too.
	if clipTile.Ground != nil {
		clipTile.HouseID, clipTile.Flags = tile.HouseID, tile.Flags
	}
	return clipTile
}

// extractExtras copies the selected creatures and spawns of a tile with
// extras into clipTile; take also removes them.
func extractExtras(tile *otmap.Tile, clipTile *ClipTile, take bool) {
	var keptCreatures []otmap.Creature
	for _, creature := range tile.Ext.Creatures {
		if creature.Selected {
			creature.Selected = false
			clipTile.Creatures = append(clipTile.Creatures, creature)
			if take {
				continue
			}
		}
		keptCreatures = append(keptCreatures, creature)
	}
	if take {
		tile.Ext.Creatures = keptCreatures
	}
	if spawn := selectedSpawnCopy(tile.Ext.Spawn); spawn != nil {
		clipTile.Spawn = spawn
		if take {
			tile.Ext.Spawn = nil
		}
	}
	if spawn := selectedSpawnCopy(tile.Ext.SpawnNPC); spawn != nil {
		clipTile.SpawnNPC = spawn
		if take {
			tile.Ext.SpawnNPC = nil
		}
	}
	if take {
		tile.Compact()
	}
}

// selectedSpawnCopy returns an unselected copy of a selected spawn, or nil
// when the spawn is missing or not selected.
func selectedSpawnCopy(spawn *otmap.Spawn) *otmap.Spawn {
	if spawn == nil || !spawn.Selected {
		return nil
	}
	copied := *spawn
	copied.Selected = false
	return &copied
}

// put merges clipboard parts into a tile, selecting them when selected is
// set.
func (e *Editor) put(tile *otmap.Tile, clipTile ClipTile, selected bool) {
	if clipTile.Ground != nil {
		ground := clipTile.Ground.Clone()
		ground.Selected = selected
		tile.Ground = ground
		if clipTile.HouseID != 0 || clipTile.Flags != 0 {
			tile.HouseID, tile.Flags = clipTile.HouseID, clipTile.Flags
		}
	}
	for _, item := range clipTile.Items {
		copied := item.Clone()
		copied.Selected = selected
		e.addItem(tile, copied)
	}
	for _, creature := range clipTile.Creatures {
		creature.Selected = selected
		tile.EnsureExt().Creatures = append(tile.EnsureExt().Creatures, creature)
	}
	if clipTile.Spawn != nil {
		spawn := *clipTile.Spawn
		spawn.Selected = selected
		tile.EnsureExt().Spawn = &spawn
	}
	if clipTile.SpawnNPC != nil {
		spawn := *clipTile.SpawnNPC
		spawn.Selected = selected
		tile.EnsureExt().SpawnNPC = &spawn
	}
}

func (c *ClipTile) empty() bool {
	return c.Ground == nil && len(c.Items) == 0 && len(c.Creatures) == 0 && c.Spawn == nil && c.SpawnNPC == nil
}

// Copy returns the selection relative to its top-left corner.
func (e *Editor) Copy() *Clip {
	lowest, highest, ok := e.SelectionBounds()
	if !ok {
		return nil
	}
	clip := &Clip{Width: int(highest.X-lowest.X) + 1, Height: int(highest.Y-lowest.Y) + 1}
	for _, pos := range e.Selected() {
		// extract without take only reads the tile.
		clipTile := extract(e.Map.Tile(pos), false)
		if clipTile.empty() {
			continue
		}
		clipTile.DX, clipTile.DY, clipTile.DZ = int(pos.X-lowest.X), int(pos.Y-lowest.Y), int(pos.Z)-int(lowest.Z)
		clip.Tiles = append(clip.Tiles, clipTile)
	}
	return clip
}

// Cut copies the selection and deletes it.
func (e *Editor) Cut() *Clip {
	clip := e.Copy()
	if clip != nil {
		e.DeleteSelection("Cut")
	}
	return clip
}

// Paste puts a clip with its top-left corner at at and selects it.
func (e *Editor) Paste(clip *Clip, at otmap.Position) error {
	if clip == nil || len(clip.Tiles) == 0 {
		return errors.New("the clipboard is empty")
	}
	tx := e.Begin("Paste", 0)
	e.deselectAll(tx)
	changed := newPosSet()
	for _, clipTile := range clip.Tiles {
		target, ok := at.Add(clipTile.DX, clipTile.DY, clipTile.DZ)
		if !ok {
			continue
		}
		e.put(tx.Edit(target), clipTile, true)
		if e.reshapes(clipTile) {
			changed.add(target)
		}
	}
	e.autoAround(tx, changed)
	tx.Commit()
	return nil
}

// reshapes reports whether placing or removing clipTile changes the
// borders, walls, tables or carpets around it. Other items leave them
// alone, so moving a barrel does not redo hand-placed borders.
func (e *Editor) reshapes(clipTile ClipTile) bool {
	if clipTile.Ground != nil {
		return true
	}
	for _, item := range clipTile.Items {
		if info := e.Brushes.info[item.ID]; info != nil && (info.wall != nil || info.table != nil || info.carpet != nil) {
			return true
		}
	}
	return false
}

// autoAround redoes borders and walls around changed tiles.
func (e *Editor) autoAround(tx *Tx, changed *posSet) {
	if !e.Settings.Automagic || len(changed.list) == 0 || len(changed.list) > maxAutoTiles {
		return
	}
	for _, pos := range changed.ring().list {
		e.fitToNeighbors(tx, pos)
	}
}

// DeleteSelection removes everything selected.
func (e *Editor) DeleteSelection(label string) int {
	if len(e.selectedPositions) == 0 {
		return 0
	}
	if label == "" {
		label = "Delete"
	}
	tx := e.Begin(label, 0)
	changed := newPosSet()
	for _, pos := range e.Selected() {
		if e.reshapes(extract(tx.Edit(pos), true)) {
			changed.add(pos)
		}
	}
	e.autoAround(tx, changed)
	return tx.Commit()
}

// MoveSelection moves the selection by an offset.
func (e *Editor) MoveSelection(dx, dy, dz int) error {
	if len(e.selectedPositions) == 0 || (dx == 0 && dy == 0 && dz == 0) {
		return nil
	}
	selected := e.Selected()
	for _, pos := range selected {
		if _, ok := pos.Add(dx, dy, dz); !ok {
			return errors.New("the selection would leave the map")
		}
	}
	tx := e.Begin("Move", 0)
	type movedPart struct {
		to       otmap.Position
		clipTile ClipTile
	}
	var parts []movedPart
	changed := newPosSet()
	for _, from := range selected {
		clipTile := extract(tx.Edit(from), true)
		if clipTile.Ground != nil {
			// The house and flags move with the ground.
			tile := tx.Edit(from)
			tile.HouseID, tile.Flags = 0, 0
		}
		to, _ := from.Add(dx, dy, dz)
		parts = append(parts, movedPart{to, clipTile})
		if e.reshapes(clipTile) {
			changed.add(from)
		}
	}
	for _, part := range parts {
		e.put(tx.Edit(part.to), part.clipTile, true)
		if e.reshapes(part.clipTile) {
			changed.add(part.to)
		}
	}
	e.autoAround(tx, changed)
	tx.Commit()
	return nil
}

// BorderizeSelection redoes borders of the selected tiles.
func (e *Editor) BorderizeSelection() int {
	return e.borderizePositions("Borderize selection", e.Selected())
}

// RandomizeSelection rerolls the grounds of the selected tiles.
func (e *Editor) RandomizeSelection() int {
	return e.randomizePositions("Randomize selection", e.Selected())
}

// randomize rerolls the ground of one tile from its ground brush.
func (e *Editor) randomize(tx *Tx, pos otmap.Position) {
	current := tx.Get(pos)
	brush := e.groundBrushOf(current)
	if brush == nil || !brush.randomize {
		return
	}
	id := brush.items.pick(e.rng)
	if id == 0 || id == current.Ground.ID {
		return
	}
	tile := tx.Edit(pos)
	tile.Ground.ID = id
}
