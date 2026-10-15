package mapedit

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/nekiro/ots-creator/internal/otitems"
	"github.com/nekiro/ots-creator/internal/otmap"
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
}

// Format is the OTBM format the settings ask for.
func (s Settings) Format() otmap.Format {
	return otmap.Format{CountU16: s.CountU16}
}

// DefaultSettings returns the defaults.
func DefaultSettings() Settings {
	return Settings{Automagic: true, EraserLeaveUnique: true, HouseRemoveItems: true, AutoDoorID: true, DefaultSpawnTime: 60, AutoCreateSpawn: true}
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

func snapshotMeta(m *otmap.Map) *metaState {
	s := &metaState{
		towns:     make(map[uint32]*otmap.Town, len(m.Towns)),
		houses:    make(map[uint32]*otmap.House, len(m.Houses)),
		waypoints: make([]*otmap.Waypoint, len(m.Waypoints)),
		zones:     slices.Clone(m.Zones),
	}
	s.header = otmap.Map{
		Width: m.Width, Height: m.Height, ItemsMajor: m.ItemsMajor, ItemsMinor: m.ItemsMinor,
		Description: m.Description, SpawnFile: m.SpawnFile, NpcFile: m.NpcFile, HouseFile: m.HouseFile, ZoneFile: m.ZoneFile,
		SpawnFormat: m.SpawnFormat,
	}
	for k, v := range m.Towns {
		c := *v
		s.towns[k] = &c
	}
	for k, v := range m.Houses {
		s.houses[k] = v.Clone()
	}
	for i, w := range m.Waypoints {
		c := *w
		s.waypoints[i] = &c
	}
	return s
}

func (s *metaState) restore(m *otmap.Map) {
	h := s.header
	m.Width, m.Height, m.ItemsMajor, m.ItemsMinor = h.Width, h.Height, h.ItemsMajor, h.ItemsMinor
	m.Description, m.SpawnFile, m.NpcFile, m.HouseFile, m.ZoneFile, m.SpawnFormat = h.Description, h.SpawnFile, h.NpcFile, h.HouseFile, h.ZoneFile, h.SpawnFormat
	again := snapshotMeta(&otmap.Map{Towns: s.towns, Houses: s.houses, Waypoints: s.waypoints, Zones: s.zones})
	m.Towns, m.Houses, m.Waypoints, m.Zones = again.towns, again.houses, again.waypoints, again.zones
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
	dirty    map[otmap.ChunkKey]bool
	metaDirt bool
	// doodad is the current doodad placement preview.
	doodad *DoodadBuffer
	// sel holds the positions of tiles with something selected.
	sel map[otmap.Position]struct{}
	// replace is the ground an Alt ground stroke replaces.
	replace strokeReplace
}

// maxHistory limits the undo stack.
const maxHistory = 400

// New returns an editor for a map.
func New(m *otmap.Map, items *otitems.DB, brushes *Brushes) *Editor {
	return &Editor{
		Map: m, Items: items, Brushes: brushes, Settings: DefaultSettings(),
		rng:   rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
		dirty: map[otmap.ChunkKey]bool{},
		sel:   map[otmap.Position]struct{}{},
	}
}

// SetBrushes replaces the brush index (after materials change).
func (e *Editor) SetBrushes(b *Brushes) {
	e.Brushes = b
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
	for _, a := range e.undo {
		undo = append(undo, ActionInfo{a.Label, len(a.changes)})
	}
	for i := len(e.redo) - 1; i >= 0; i-- {
		redo = append(redo, ActionInfo{e.redo[i].Label, len(e.redo[i].changes)})
	}
	return undo, redo
}

// TakeDirty returns the chunks changed since the last call and whether map
// data outside tiles changed.
func (e *Editor) TakeDirty() (chunks []otmap.ChunkKey, meta bool) {
	for k := range e.dirty {
		chunks = append(chunks, k)
	}
	clear(e.dirty)
	meta, e.metaDirt = e.metaDirt, false
	return chunks, meta
}

func (e *Editor) markDirty(p otmap.Position) { e.dirty[otmap.KeyOf(p)] = true }

// Undo reverts the last action and returns its label.
func (e *Editor) Undo() string {
	if len(e.undo) == 0 {
		return ""
	}
	a := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for i := len(a.changes) - 1; i >= 0; i-- {
		c := a.changes[i]
		e.Map.SetTile(c.pos, c.before.Clone())
		e.markDirty(c.pos)
		e.syncSel(c.pos)
	}
	if a.metaBefore != nil {
		a.metaBefore.restore(e.Map)
		e.metaDirt = true
	}
	e.redo = append(e.redo, a)
	return a.Label
}

// Redo applies the last undone action again.
func (e *Editor) Redo() string {
	if len(e.redo) == 0 {
		return ""
	}
	a := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, c := range a.changes {
		e.Map.SetTile(c.pos, c.after.Clone())
		e.markDirty(c.pos)
		e.syncSel(c.pos)
	}
	if a.metaAfter != nil {
		a.metaAfter.restore(e.Map)
		e.metaDirt = true
	}
	e.undo = append(e.undo, a)
	return a.Label
}

// Tx collects the changes of one action. Tiles are copied on first write,
// so the map never shares tiles with the history.
type Tx struct {
	e       *Editor
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
	return &Tx{e: e, label: label, stroke: stroke, before: map[otmap.Position]*otmap.Tile{}, written: map[otmap.Position]bool{}}
}

// BeginSelection starts a change that is not recorded in the history
// (selection only).
func (e *Editor) BeginSelection() *Tx {
	t := e.Begin("Select", 0)
	t.noUndo = true
	return t
}

// Get returns the current tile (read-only).
func (t *Tx) Get(p otmap.Position) *otmap.Tile { return t.e.Map.Tile(p) }

// Prior returns the tile at p as it was before this action (read-only).
func (t *Tx) Prior(p otmap.Position) *otmap.Tile {
	if b, seen := t.before[p]; seen {
		return b
	}
	return t.e.Map.Tile(p)
}

// Edit returns a writable copy of the tile at p, creating an empty tile.
// The copy is stored in the map right away so neighbors see it.
func (t *Tx) Edit(p otmap.Position) *otmap.Tile {
	cur := t.e.Map.Tile(p)
	if t.written[p] && cur != nil {
		return cur
	}
	if _, seen := t.before[p]; !seen {
		t.before[p] = cur
		t.order = append(t.order, p)
	}
	nt := cur.Clone()
	if nt == nil {
		nt = &otmap.Tile{}
	}
	if !t.noUndo {
		nt.State |= otmap.StateModified
	}
	t.e.Map.Put(p, nt)
	t.written[p] = true
	t.e.markDirty(p)
	return nt
}

// Meta records the map data outside tiles before it changes.
func (t *Tx) Meta() {
	if t.meta == nil {
		t.meta = snapshotMeta(t.e.Map)
	}
	t.e.metaDirt = true
}

// Commit finishes the action and returns the number of changed tiles.
func (t *Tx) Commit() int {
	e := t.e
	var changes []change
	for _, p := range t.order {
		e.Map.Prune(p)
		e.syncSel(p)
		after := e.Map.Tile(p)
		before := t.before[p]
		if before == nil && after == nil {
			continue
		}
		changes = append(changes, change{pos: p, before: before, after: after})
	}
	if t.noUndo || (len(changes) == 0 && t.meta == nil) {
		return len(changes)
	}
	var metaAfter *metaState
	if t.meta != nil {
		metaAfter = snapshotMeta(e.Map)
	}
	// Stored tiles are clones: the map keeps editing its own copies.
	for i := range changes {
		changes[i].after = changes[i].after.Clone()
	}
	e.redo = nil
	if top := e.top(); t.stroke != 0 && top != nil && top.stroke == t.stroke {
		// Merge: keep the oldest "before" of each tile.
		known := map[otmap.Position]int{}
		for i, c := range top.changes {
			known[c.pos] = i
		}
		for _, c := range changes {
			if i, ok := known[c.pos]; ok {
				top.changes[i].after = c.after
			} else {
				top.changes = append(top.changes, c)
			}
		}
		if t.meta != nil {
			if top.metaBefore == nil {
				top.metaBefore = t.meta
			}
			top.metaAfter = metaAfter
		}
		return len(changes)
	}
	e.undo = append(e.undo, &Action{Label: t.label, changes: changes, metaBefore: t.meta, metaAfter: metaAfter, stroke: t.stroke, at: time.Now()})
	if len(e.undo) > maxHistory {
		n := len(e.undo) - maxHistory
		e.undo = slices.Delete(e.undo, 0, n)
		if e.clean >= 0 {
			e.clean -= n
			if e.clean < 0 {
				e.clean = -1
			}
		}
	}
	if e.clean > len(e.undo) {
		e.clean = -1 // the saved state was undone and replaced
	}
	return len(changes)
}

func (e *Editor) top() *Action {
	if len(e.undo) == 0 {
		return nil
	}
	return e.undo[len(e.undo)-1]
}
