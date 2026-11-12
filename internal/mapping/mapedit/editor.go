package mapedit

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

type Settings struct {
	Automagic bool `json:"automagic"`
	// EraserLeaveUnique keeps items with attributes (and borders) when
	// erasing.
	EraserLeaveUnique bool  `json:"eraserLeaveUnique"`
	HouseRemoveItems  bool  `json:"houseRemoveItems"`
	AutoDoorID        bool  `json:"autoDoorId"`
	DoodadEraseLike   bool  `json:"doodadEraseLike"`
	DefaultSpawnTime  int32 `json:"defaultSpawnTime"`
	AutoCreateSpawn   bool  `json:"autoCreateSpawn"`
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
	// ViewRangeX and ViewRangeY are the tiles a player sees to each side,
	// ViewRangeFloors the floors seen up and down: tiles out of this view
	// of every walkable tile are removed by RemoveUnreachable.
	ViewRangeX      int `json:"viewRangeX"`
	ViewRangeY      int `json:"viewRangeY"`
	ViewRangeFloors int `json:"viewRangeFloors"`
	// ShowFPS shows the frames drawn per second in the corner of the map.
	ShowFPS bool `json:"showFps"`
}

// UnmarshalJSON starts from DefaultSettings, so settings saved before a
// field existed get its default.
func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	decoded := plain(DefaultSettings())
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = Settings(decoded)
	return nil
}

func (s Settings) Format() otmap.Format {
	return otmap.Format{CountU16: s.CountU16}
}

func DefaultSettings() Settings {
	return Settings{
		Automagic: true, EraserLeaveUnique: true, HouseRemoveItems: true, AutoDoorID: true,
		DefaultSpawnTime: defaultSpawnTimeSeconds, AutoCreateSpawn: true,
		ViewRangeX: defaultViewRangeX, ViewRangeY: defaultViewRangeY, ViewRangeFloors: defaultViewRangeFloors,
		ShowFPS: true,
	}
}

// DefaultMarkedItems returns the items RME marks: the invisible stairs and
// holes, 459 and 460 in items.otb, 469 and 470 as client ids.
func DefaultMarkedItems(mode otitems.Mode) []uint16 {
	if mode == otitems.ModeClient {
		return []uint16{469, 470}
	}
	return []uint16{459, 460}
}

type change struct {
	pos    otmap.Position
	before *otmap.Tile
	after  *otmap.Tile
}

type metaState struct {
	header    otmap.Map // only the header fields are used
	towns     map[uint32]*otmap.Town
	houses    map[uint32]*otmap.House
	waypoints []*otmap.Waypoint
	zones     []otmap.Zone
}

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

func (s *metaState) restore(gameMap *otmap.Map) {
	header := s.header
	gameMap.Width, gameMap.Height, gameMap.ItemsMajor, gameMap.ItemsMinor = header.Width, header.Height, header.ItemsMajor, header.ItemsMinor
	gameMap.Description, gameMap.SpawnFile, gameMap.NpcFile, gameMap.HouseFile, gameMap.ZoneFile, gameMap.SpawnFormat = header.Description, header.SpawnFile, header.NpcFile, header.HouseFile, header.ZoneFile, header.SpawnFormat
	restored := snapshotMeta(&otmap.Map{Towns: s.towns, Houses: s.houses, Waypoints: s.waypoints, Zones: s.zones})
	gameMap.Towns, gameMap.Houses, gameMap.Waypoints, gameMap.Zones = restored.towns, restored.houses, restored.waypoints, restored.zones
}

type Action struct {
	Label   string
	changes []change
	// ID conversion changes the interpretation together with the tiles.
	itemsBefore, itemsAfter     *otitems.DB
	brushesBefore, brushesAfter *Brushes
	// metaBefore/metaAfter are set when the map data outside tiles changed.
	metaBefore *metaState
	metaAfter  *metaState
	stroke     int64
}

type ActionInfo struct {
	Label string `json:"label"`
	Tiles int    `json:"tiles"`
}

type Editor struct {
	Map      *otmap.Map
	Items    *otitems.DB
	Brushes  *Brushes
	Settings Settings
	// Progress, when set, is called by the map-wide tools as they work
	// through the map (see progressOver).
	Progress func(done, total int)

	undo []*Action
	redo []*Action
	// clean is the undo stack depth of the last save (-1: never matches).
	clean             int
	rng               *rand.Rand
	dirty             map[otmap.ChunkKey]bool
	metaDirty         bool
	doodad            *DoodadBuffer
	selectedPositions map[otmap.Position]struct{}
	replace           strokeReplace
	peekScratch       otmap.Tile
	peekItems         [32]otmap.Item
	// observer sees the changes of a shared map (see shared.go); foreign is
	// set when other editors changed the map since the last save.
	observer func(Commit)
	foreign  bool
}

// peek returns the map tile at x, y, z for a read-only look that ends
// before the next peek: packed tiles are unpacked into the editor's scratch
// instead of a new copy (see otmap.Map.Peek).
func (e *Editor) peek(x, y, z int) *otmap.Tile {
	return e.Map.PeekAt(x, y, z, &e.peekScratch, e.peekItems[:])
}

func (e *Editor) peekPos(pos otmap.Position) *otmap.Tile {
	return e.Map.Peek(pos, &e.peekScratch, e.peekItems[:])
}

func New(gameMap *otmap.Map, items *otitems.DB, brushes *Brushes) *Editor {
	return &Editor{
		Map: gameMap, Items: items, Brushes: brushes, Settings: DefaultSettings(),
		rng:               rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rngStream)),
		dirty:             map[otmap.ChunkKey]bool{},
		selectedPositions: map[otmap.Position]struct{}{},
	}
}

func (e *Editor) SetBrushes(brushes *Brushes) {
	e.Brushes = brushes
	e.doodad = nil
}

func (e *Editor) Seed(seed uint64) { e.rng = rand.New(rand.NewPCG(seed, seed)) }

func (e *Editor) Changed() bool { return len(e.undo) != e.clean || e.foreign }

func (e *Editor) MarkSaved() {
	e.clean = len(e.undo)
	e.foreign = false
}

func (e *Editor) CanUndo() bool { return len(e.undo) > 0 }

func (e *Editor) CanRedo() bool { return len(e.redo) > 0 }

func (e *Editor) History() (undo, redo []ActionInfo) {
	for _, action := range e.undo {
		undo = append(undo, ActionInfo{action.Label, len(action.changes)})
	}
	for i := len(e.redo) - 1; i >= 0; i-- {
		redo = append(redo, ActionInfo{e.redo[i].Label, len(e.redo[i].changes)})
	}
	return undo, redo
}

func (e *Editor) TakeDirty() (chunks []otmap.ChunkKey, meta bool) {
	for chunk := range e.dirty {
		chunks = append(chunks, chunk)
	}
	clear(e.dirty)
	meta, e.metaDirty = e.metaDirty, false
	return chunks, meta
}

func (e *Editor) markDirty(pos otmap.Position) { e.dirty[otmap.KeyOf(pos)] = true }

func (e *Editor) Undo() string {
	if len(e.undo) == 0 {
		return ""
	}
	action := e.undo[len(e.undo)-1]
	if action.itemsBefore != nil && e.Shared() {
		return ""
	}
	e.undo = e.undo[:len(e.undo)-1]
	var shared []TileState
	for i := len(action.changes) - 1; i >= 0; i-- {
		tileChange := action.changes[i]
		// On a shared map a tile changed by another editor since keeps
		// their change.
		if e.Shared() && !SameTile(e.peekPos(tileChange.pos), tileChange.after) {
			continue
		}
		e.Map.SetTile(tileChange.pos, tileChange.before.Clone())
		e.markDirty(tileChange.pos)
		e.syncSelection(tileChange.pos)
		if e.Shared() {
			shared = append(shared, sharedState(tileChange.pos, tileChange.before))
		}
	}
	var meta *MetaData
	if action.metaBefore != nil {
		action.metaBefore.restore(e.Map)
		e.metaDirty = true
		meta = e.sharedMetaIfObserved()
	}
	if action.itemsBefore != nil {
		e.Items = action.itemsBefore
		e.SetBrushes(action.brushesBefore)
	}
	e.redo = append(e.redo, action)
	e.notify("Undo "+action.Label, shared, meta)
	return action.Label
}

func (e *Editor) Redo() string {
	if len(e.redo) == 0 {
		return ""
	}
	action := e.redo[len(e.redo)-1]
	if action.itemsAfter != nil && e.Shared() {
		return ""
	}
	e.redo = e.redo[:len(e.redo)-1]
	var shared []TileState
	for _, tileChange := range action.changes {
		if e.Shared() && !SameTile(e.peekPos(tileChange.pos), tileChange.before) {
			continue
		}
		e.Map.SetTile(tileChange.pos, tileChange.after.Clone())
		e.markDirty(tileChange.pos)
		e.syncSelection(tileChange.pos)
		if e.Shared() {
			shared = append(shared, sharedState(tileChange.pos, tileChange.after))
		}
	}
	var meta *MetaData
	if action.metaAfter != nil {
		action.metaAfter.restore(e.Map)
		e.metaDirty = true
		meta = e.sharedMetaIfObserved()
	}
	if action.itemsAfter != nil {
		e.Items = action.itemsAfter
		e.SetBrushes(action.brushesAfter)
	}
	e.undo = append(e.undo, action)
	e.notify("Redo "+action.Label, shared, meta)
	return action.Label
}

func (e *Editor) sharedMetaIfObserved() *MetaData {
	if !e.Shared() {
		return nil
	}
	return metaDataOf(e.Map)
}

// Tx collects the changes of one action. Tiles are copied on first write,
// so the map never shares tiles with the history.
type Tx struct {
	editor *Editor
	label  string
	stroke int64
	before map[otmap.Position]*otmap.Tile
	order  []otmap.Position
	meta   *metaState
	noUndo bool
}

func (t *Tx) Label() string { return t.label }

// Begin starts an action. Actions with the same non-zero stroke merge into
// one undo step (a brush stroke sent in several requests).
func (e *Editor) Begin(label string, stroke int64) *Tx {
	return &Tx{editor: e, label: label, stroke: stroke, before: map[otmap.Position]*otmap.Tile{}}
}

// BeginSelection starts a change that is not recorded in the history
// (selection only).
func (e *Editor) BeginSelection() *Tx {
	tx := e.Begin("Select", 0)
	tx.noUndo = true
	return tx
}

func (t *Tx) Get(pos otmap.Position) *otmap.Tile { return t.editor.Map.Tile(pos) }

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
	_, seen := t.before[pos]
	if seen && current != nil {
		return current
	}
	if !seen {
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
	t.editor.markDirty(pos)
	return tile
}

func (t *Tx) Meta() {
	if t.meta == nil {
		t.meta = snapshotMeta(t.editor.Map)
	}
	t.editor.metaDirty = true
}

func (t *Tx) Commit() int {
	editor := t.editor
	changes := t.collectChanges()
	if t.noUndo || (len(changes) == 0 && t.meta == nil) {
		return len(changes)
	}
	if editor.Shared() {
		shared := make([]TileState, len(changes))
		for i, tileChange := range changes {
			shared[i] = sharedState(tileChange.pos, tileChange.after)
		}
		var meta *MetaData
		if t.meta != nil {
			meta = metaDataOf(editor.Map)
		}
		editor.notify(t.label, shared, meta)
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
	editor.undo = append(editor.undo, &Action{Label: t.label, changes: changes, metaBefore: t.meta, metaAfter: metaAfter, stroke: t.stroke})
	editor.trimHistory()
	return len(changes)
}

// collectChanges prunes the written tiles and returns the ones that changed,
// in the order they were first written.
func (t *Tx) collectChanges() []change {
	editor := t.editor
	changes := make([]change, 0, len(t.order))
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
