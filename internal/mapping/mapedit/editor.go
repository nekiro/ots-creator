package mapedit

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Settings change how brushes behave.
type Settings struct {
	// Automagic places borders, walls, carpets and tables automatically.
	Automagic bool `json:"automagic"`
	// EraserLeaveUnique keeps items with attributes (and borders) when
	// erasing.
	EraserLeaveUnique bool `json:"eraserLeaveUnique"`
	// HouseRemoveItems removes moveable items from tiles painted as house.
	HouseRemoveItems bool `json:"houseRemoveItems"`
	// AutoDoorID assigns free door ids to doors of house tiles.
	AutoDoorID bool `json:"autoDoorId"`
	// DoodadEraseLike only erases items of the selected doodad brush.
	DoodadEraseLike bool `json:"doodadEraseLike"`
	// DefaultSpawnTime is the spawn time of new creatures (seconds).
	DefaultSpawnTime int32 `json:"defaultSpawnTime"`
	// AutoCreateSpawn creates a spawn under creatures placed outside one.
	AutoCreateSpawn bool `json:"autoCreateSpawn"`
	// RawReplaceBottom replaces bottom items (walls) of the same kind when
	// placing a raw bottom item.
	RawReplaceBottom bool `json:"rawReplaceBottom"`
	// CountU16 reads and writes item counts as uint16 (servers with stacks
	// above 255).
	CountU16 bool `json:"countU16"`
	// MarkedItems are server ids of items the editor covers with a square
	// in their minimap color, such as invisible stairs. Nil: the
	// DefaultMarkedItems of the items mode.
	MarkedItems []uint16 `json:"markedItems"`
}

// Format is the OTBM format the settings ask for.
func (s Settings) Format() otmap.Format {
	return otmap.Format{CountU16: s.CountU16}
}

// DefaultSettings returns the defaults.
func DefaultSettings() Settings {
	return Settings{Automagic: true, EraserLeaveUnique: true, HouseRemoveItems: true, AutoDoorID: true, DefaultSpawnTime: defaultSpawnTimeSeconds, AutoCreateSpawn: true}
}

// DefaultMarkedItems returns the items RME marks: the invisible stairs and
// holes, 459 and 460 in items.otb, 469 and 470 as client ids.
func DefaultMarkedItems(mode otitems.Mode) []uint16 {
	if mode == otitems.ModeClient {
		return []uint16{469, 470}
	}
	return []uint16{459, 460}
}

// change is one tile change of an action.
type change struct {
	pos    otmap.Position
	before *otmap.Tile
	after  *otmap.Tile
}

// metaState is the map data outside tiles.
type metaState struct {
	header    otmap.Map // only the header fields are used
	towns     map[uint32]*otmap.Town
	houses    map[uint32]*otmap.House
	waypoints []*otmap.Waypoint
	zones     []otmap.Zone
}

// snapshotMeta copies the map data outside tiles.
func snapshotMeta(gameMap *otmap.Map) *metaState {
	snapshot := &metaState{
		towns:     make(map[uint32]*otmap.Town, len(gameMap.Towns)),
		houses:    make(map[uint32]*otmap.House, len(gameMap.Houses)),
		waypoints: make([]*otmap.Waypoint, len(gameMap.Waypoints)),
		zones:     slices.Clone(gameMap.Zones),
	}
	snapshot.header = otmap.Map{
		Width: gameMap.Width, Height: gameMap.Height, ItemsMajor: gameMap.ItemsMajor, ItemsMinor: gameMap.ItemsMinor,
		Description: gameMap.Description, SpawnFile: gameMap.SpawnFile, NpcFile: gameMap.NpcFile, HouseFile: gameMap.HouseFile, ZoneFile: gameMap.ZoneFile,
		SpawnFormat: gameMap.SpawnFormat,
	}
	for id, town := range gameMap.Towns {
		townCopy := *town
		snapshot.towns[id] = &townCopy
	}
	for id, house := range gameMap.Houses {
		snapshot.houses[id] = house.Clone()
	}
	for i, waypoint := range gameMap.Waypoints {
		waypointCopy := *waypoint
		snapshot.waypoints[i] = &waypointCopy
	}
	return snapshot
}

// restore puts a copy of the saved data back into the map.
func (s *metaState) restore(gameMap *otmap.Map) {
	header := s.header
	gameMap.Width, gameMap.Height, gameMap.ItemsMajor, gameMap.ItemsMinor = header.Width, header.Height, header.ItemsMajor, header.ItemsMinor
	gameMap.Description, gameMap.SpawnFile, gameMap.NpcFile, gameMap.HouseFile, gameMap.ZoneFile, gameMap.SpawnFormat = header.Description, header.SpawnFile, header.NpcFile, header.HouseFile, header.ZoneFile, header.SpawnFormat
	restored := snapshotMeta(&otmap.Map{Towns: s.towns, Houses: s.houses, Waypoints: s.waypoints, Zones: s.zones})
	gameMap.Towns, gameMap.Houses, gameMap.Waypoints, gameMap.Zones = restored.towns, restored.houses, restored.waypoints, restored.zones
}

// Action is one undo step.
type Action struct {
	Label   string
	changes []change
	// metaBefore/metaAfter are set when the map data outside tiles changed.
	metaBefore *metaState
	metaAfter  *metaState
	stroke     int64
	at         time.Time
}

// ActionInfo describes an undo step for the history window.
type ActionInfo struct {
	Label string `json:"label"`
	Tiles int    `json:"tiles"`
}

// Editor edits one map.
type Editor struct {
	Map      *otmap.Map
	Items    *otitems.DB
	Brushes  *Brushes
	Settings Settings

	undo []*Action
	redo []*Action
	// clean is the undo stack depth of the last save (-1: never matches).
	clean int
	rng   *rand.Rand
	// dirty collects the chunks changed since the last TakeDirty.
	dirty     map[otmap.ChunkKey]bool
	metaDirty bool
	// doodad is the current doodad placement preview.
	doodad *DoodadBuffer
	// selectedPositions holds the positions of tiles with something selected.
	selectedPositions map[otmap.Position]struct{}
	// replace is the ground an Alt ground stroke replaces.
	replace strokeReplace
	// peekScratch and peekItems hold the tile returned by peek.
	peekScratch otmap.Tile
	peekItems   [32]otmap.Item
}

// peek returns the map tile at x, y, z for a read-only look that ends
// before the next peek: packed tiles are unpacked into the editor's scratch
// instead of a new copy (see otmap.Map.Peek).
func (e *Editor) peek(x, y, z int) *otmap.Tile {
	return e.Map.PeekAt(x, y, z, &e.peekScratch, e.peekItems[:])
}

// peekPos is peek for a position.
func (e *Editor) peekPos(pos otmap.Position) *otmap.Tile {
	return e.Map.Peek(pos, &e.peekScratch, e.peekItems[:])
}

// New returns an editor for a map.
func New(gameMap *otmap.Map, items *otitems.DB, brushes *Brushes) *Editor {
	return &Editor{
		Map: gameMap, Items: items, Brushes: brushes, Settings: DefaultSettings(),
		rng:               rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rngStream)),
		dirty:             map[otmap.ChunkKey]bool{},
		selectedPositions: map[otmap.Position]struct{}{},
	}
}

// SetBrushes replaces the brush index (after materials change).
func (e *Editor) SetBrushes(brushes *Brushes) {
	e.Brushes = brushes
	e.doodad = nil
}

// Seed makes random choices reproducible (tests).
func (e *Editor) Seed(seed uint64) { e.rng = rand.New(rand.NewPCG(seed, seed)) }

// Changed reports whether the map has changes since it was saved.
func (e *Editor) Changed() bool { return len(e.undo) != e.clean }

// MarkSaved marks the current state as saved.
func (e *Editor) MarkSaved() { e.clean = len(e.undo) }

// MarkUnsaved makes the map count as changed (new or imported maps).
func (e *Editor) MarkUnsaved() { e.clean = -1 }

// CanUndo reports whether there is something to undo.
func (e *Editor) CanUndo() bool { return len(e.undo) > 0 }

// CanRedo reports whether there is something to redo.
func (e *Editor) CanRedo() bool { return len(e.redo) > 0 }

// History returns the undo steps, oldest first, and the redo steps.
func (e *Editor) History() (undo, redo []ActionInfo) {
	for _, action := range e.undo {
		undo = append(undo, ActionInfo{action.Label, len(action.changes)})
	}
	for i := len(e.redo) - 1; i >= 0; i-- {
		redo = append(redo, ActionInfo{e.redo[i].Label, len(e.redo[i].changes)})
	}
	return undo, redo
}

// TakeDirty returns the chunks changed since the last call and whether map
// data outside tiles changed.
func (e *Editor) TakeDirty() (chunks []otmap.ChunkKey, meta bool) {
	for chunk := range e.dirty {
		chunks = append(chunks, chunk)
	}
	clear(e.dirty)
	meta, e.metaDirty = e.metaDirty, false
	return chunks, meta
}

func (e *Editor) markDirty(pos otmap.Position) { e.dirty[otmap.KeyOf(pos)] = true }

// Undo reverts the last action and returns its label.
func (e *Editor) Undo() string {
	if len(e.undo) == 0 {
		return ""
	}
	action := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for i := len(action.changes) - 1; i >= 0; i-- {
		tileChange := action.changes[i]
		e.Map.SetTile(tileChange.pos, tileChange.before.Clone())
		e.markDirty(tileChange.pos)
		e.syncSelection(tileChange.pos)
	}
	if action.metaBefore != nil {
		action.metaBefore.restore(e.Map)
		e.metaDirty = true
	}
	e.redo = append(e.redo, action)
	return action.Label
}

// Redo applies the last undone action again.
func (e *Editor) Redo() string {
	if len(e.redo) == 0 {
		return ""
	}
	action := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, tileChange := range action.changes {
		e.Map.SetTile(tileChange.pos, tileChange.after.Clone())
		e.markDirty(tileChange.pos)
		e.syncSelection(tileChange.pos)
	}
	if action.metaAfter != nil {
		action.metaAfter.restore(e.Map)
		e.metaDirty = true
	}
	e.undo = append(e.undo, action)
	return action.Label
}

// Tx collects the changes of one action. Tiles are copied on first write,
// so the map never shares tiles with the history.
type Tx struct {
	editor  *Editor
	label   string
	stroke  int64
	before  map[otmap.Position]*otmap.Tile
	order   []otmap.Position
	meta    *metaState
	noUndo  bool
	written map[otmap.Position]bool
}

// Label returns the action label.
func (t *Tx) Label() string { return t.label }

// Begin starts an action. Actions with the same non-zero stroke merge into
// one undo step (a brush stroke sent in several requests).
func (e *Editor) Begin(label string, stroke int64) *Tx {
	return &Tx{editor: e, label: label, stroke: stroke, before: map[otmap.Position]*otmap.Tile{}, written: map[otmap.Position]bool{}}
}

// BeginSelection starts a change that is not recorded in the history
// (selection only).
func (e *Editor) BeginSelection() *Tx {
	tx := e.Begin("Select", 0)
	tx.noUndo = true
	return tx
}

// Get returns the current tile (read-only).
func (t *Tx) Get(pos otmap.Position) *otmap.Tile { return t.editor.Map.Tile(pos) }

// Prior returns the tile at pos as it was before this action (read-only).
func (t *Tx) Prior(pos otmap.Position) *otmap.Tile {
	if beforeTile, seen := t.before[pos]; seen {
		return beforeTile
	}
	return t.editor.Map.Tile(pos)
}

// Edit returns a writable copy of the tile at pos, creating an empty tile.
// The copy is stored in the map right away so neighbors see it.
func (t *Tx) Edit(pos otmap.Position) *otmap.Tile {
	current := t.editor.Map.Tile(pos)
	if t.written[pos] && current != nil {
		return current
	}
	if _, seen := t.before[pos]; !seen {
		t.before[pos] = current
		t.order = append(t.order, pos)
	}
	tile := current.Clone()
	if tile == nil {
		tile = &otmap.Tile{}
	}
	if !t.noUndo {
		tile.State |= otmap.StateModified
	}
	t.editor.Map.Put(pos, tile)
	t.written[pos] = true
	t.editor.markDirty(pos)
	return tile
}

// Meta records the map data outside tiles before it changes.
func (t *Tx) Meta() {
	if t.meta == nil {
		t.meta = snapshotMeta(t.editor.Map)
	}
	t.editor.metaDirty = true
}

// Commit finishes the action and returns the number of changed tiles.
func (t *Tx) Commit() int {
	editor := t.editor
	changes := t.collectChanges()
	if t.noUndo || (len(changes) == 0 && t.meta == nil) {
		return len(changes)
	}
	var metaAfter *metaState
	if t.meta != nil {
		metaAfter = snapshotMeta(editor.Map)
	}
	// Stored tiles are clones: the map keeps editing its own copies.
	for i := range changes {
		changes[i].after = changes[i].after.Clone()
	}
	// A saved state on the redo stack is lost with it.
	if editor.clean > len(editor.undo) {
		editor.clean = -1
	}
	editor.redo = nil
	if top := editor.top(); t.stroke != 0 && top != nil && top.stroke == t.stroke {
		// The saved state is the top step: merging changes it.
		if editor.clean == len(editor.undo) {
			editor.clean = -1
		}
		top.mergeStroke(changes, t.meta, metaAfter)
		return len(changes)
	}
	editor.undo = append(editor.undo, &Action{Label: t.label, changes: changes, metaBefore: t.meta, metaAfter: metaAfter, stroke: t.stroke, at: time.Now()})
	editor.trimHistory()
	return len(changes)
}

// collectChanges prunes the written tiles and returns the ones that changed,
// in the order they were first written.
func (t *Tx) collectChanges() []change {
	editor := t.editor
	var changes []change
	for _, pos := range t.order {
		editor.Map.Prune(pos)
		editor.syncSelection(pos)
		after := editor.Map.Tile(pos)
		before := t.before[pos]
		if before == nil && after == nil {
			continue
		}
		changes = append(changes, change{pos: pos, before: before, after: after})
	}
	return changes
}

// mergeStroke adds the changes of a later request of the same stroke to the
// action. It keeps the oldest "before" of each tile.
func (a *Action) mergeStroke(changes []change, metaBefore, metaAfter *metaState) {
	known := map[otmap.Position]int{}
	for i, tileChange := range a.changes {
		known[tileChange.pos] = i
	}
	for _, tileChange := range changes {
		if i, ok := known[tileChange.pos]; ok {
			a.changes[i].after = tileChange.after
		} else {
			a.changes = append(a.changes, tileChange)
		}
	}
	if metaBefore != nil {
		if a.metaBefore == nil {
			a.metaBefore = metaBefore
		}
		a.metaAfter = metaAfter
	}
}

// trimHistory drops the oldest undo steps above maxHistory and moves the
// saved mark with them.
func (e *Editor) trimHistory() {
	if len(e.undo) <= maxHistory {
		return
	}
	dropped := len(e.undo) - maxHistory
	e.undo = slices.Delete(e.undo, 0, dropped)
	if e.clean >= 0 {
		e.clean -= dropped
		if e.clean < 0 {
			e.clean = -1
		}
	}
}

func (e *Editor) top() *Action {
	if len(e.undo) == 0 {
		return nil
	}
	return e.undo[len(e.undo)-1]
}
