package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/mapedit"
	"github.com/nekiro/ots-creator/internal/materials"
	"github.com/nekiro/ots-creator/internal/otitems"
	"github.com/nekiro/ots-creator/internal/otmap"
	"github.com/nekiro/ots-creator/internal/project"
	"github.com/nekiro/ots-creator/internal/thing"
)

// ErrNoMap is returned for an unknown map id.
var ErrNoMap = errors.New("the map is not open")

// mapData is the item database and brushes of the open client.
type mapData struct {
	project   *project.Project
	client    string
	cfg       MapDataConfig
	items     *otitems.DB
	mats      *materials.Materials
	matsPath  string
	brushes   *mapedit.Brushes
	creatures map[string]*materials.CreatureType
	scanned   []*materials.CreatureType
	warnings  []string
}

type openMap struct {
	id   int
	path string
	ed   *mapedit.Editor
}

func (m *openMap) name() string {
	if m.path == "" {
		return fmt.Sprintf("untitled-%d.otbm", m.id)
	}
	return filepath.Base(m.path)
}

// MapService edits OTBM maps with the sprites of the open client.
type MapService struct {
	s     *Session
	store *MapStore
	// mu guards everything; requests that only read the map share it.
	mu    sync.RWMutex
	maps  map[int]*openMap
	next  int
	data  *mapData
	clip  *mapedit.Clip
}

// NewMapService returns the service.
func NewMapService(s *Session, store *MapStore) *MapService {
	return &MapService{s: s, store: store, maps: map[int]*openMap{}}
}

// projectSource reads client items for the item database.
type projectSource struct{ m map[uint32]*thing.Thing }

func (ps projectSource) Thing(_ thing.Category, id uint32) (*thing.Thing, error) {
	if t, ok := ps.m[id]; ok {
		return t, nil
	}
	return nil, errors.New("missing")
}

func clientKey(p *project.Project) string {
	info := p.Info()
	if info.DatPath == "" {
		return "unsaved client " + info.Version.Name
	}
	return info.DatPath
}

// loadData builds the item database and brushes for the open client.
func (ms *MapService) loadData(cfg *MapDataConfig, hint string) (*mapData, error) {
	p, err := ms.s.Project()
	if err != nil {
		return nil, errNoClient
	}
	key := clientKey(p)
	var c MapDataConfig
	switch {
	case cfg != nil:
		c = *cfg
	default:
		if stored, ok := ms.store.Profile(key); ok {
			c = stored
		}
		if c.XMLPath == "" && c.OTBPath == "" && hint != "" {
			d := detectServerData(hint)
			c.OTBPath, c.XMLPath = d.OTBPath, d.XMLPath
			if c.ServerDir == "" {
				c.ServerDir = d.ServerDir
			}
		}
	}
	if c.ItemsMode == "" {
		c.ItemsMode = "auto"
	}
	d := &mapData{project: p, client: key, cfg: c, creatures: map[string]*materials.CreatureType{}}
	info := p.Info()
	src := projectSource{m: map[uint32]*thing.Thing{}}
	for _, t := range p.ThingRange(thing.CategoryItem, 100, info.Counts.Items) {
		src.m[t.ID] = t
	}
	useOTB := c.ItemsMode == "otb" || (c.ItemsMode == "auto" && c.OTBPath != "" && fileExists(c.OTBPath))
	if useOTB {
		data, err := os.ReadFile(c.OTBPath)
		if err != nil {
			return nil, fmt.Errorf("items.otb: %w", err)
		}
		db, err := otitems.ParseOTB(data)
		if err != nil {
			return nil, err
		}
		db.ApplyClient(src)
		d.items = db
	} else {
		d.items = otitems.FromClient(src, info.Counts.Items, p.Names(thing.CategoryItem))
	}
	if xml, err := readOptional(c.XMLPath); err != nil {
		d.warnings = append(d.warnings, "items.xml: "+err.Error())
	} else if xml != nil {
		if err := d.items.ApplyXML(xml); err != nil {
			d.warnings = append(d.warnings, err.Error())
		}
	}
	d.matsPath = c.MaterialsPath
	if d.matsPath == "" && ms.store.Dir() != "" {
		d.matsPath = filepath.Join(ms.store.Dir(), "materials", fmt.Sprintf("%d-%s.json", info.Version.Value, d.items.Mode))
	}
	d.mats = materials.New()
	if d.matsPath != "" {
		if m, err := materials.Load(d.matsPath); err == nil {
			d.mats = m
		} else if !os.IsNotExist(err) {
			d.warnings = append(d.warnings, err.Error())
		}
	}
	if c.ServerDir != "" {
		if cs, err := materials.ScanCreatures(c.ServerDir); err == nil {
			d.scanned = cs
		}
	}
	d.index()
	return d, nil
}

func (d *mapData) index() {
	d.brushes = mapedit.NewBrushes(d.items, d.mats)
	d.creatures = map[string]*materials.CreatureType{}
	for _, c := range d.scanned {
		d.creatures[strings.ToLower(c.Name)] = c
	}
	for _, c := range d.mats.Creatures {
		d.creatures[strings.ToLower(c.Name)] = c
	}
}

// ensure returns the data of the open client, building it when the client
// changed.
func (ms *MapService) ensure(hint string) (*mapData, error) {
	p, err := ms.s.Project()
	if err != nil {
		return nil, errNoClient
	}
	if ms.data != nil && ms.data.project == p {
		return ms.data, nil
	}
	d, err := ms.loadData(nil, hint)
	if err != nil {
		return nil, err
	}
	ms.setData(d)
	return d, nil
}

func (ms *MapService) setData(d *mapData) {
	ms.data = d
	for _, m := range ms.maps {
		m.ed.Items = d.items
		m.ed.SetBrushes(d.brushes)
	}
}

func (ms *MapService) get(id int) (*openMap, *mapData, error) {
	d, err := ms.ensure("")
	if err != nil {
		return nil, nil, err
	}
	m := ms.maps[id]
	if m == nil {
		return nil, nil, ErrNoMap
	}
	return m, d, nil
}

// read read-locks the service and returns an open map and the item data
// with the function that unlocks. Loading the item data, the first time,
// takes the write lock.
func (ms *MapService) read(id int) (*openMap, *mapData, func(), error) {
	for {
		ms.mu.RLock()
		p, err := ms.s.Project()
		if err == nil && ms.data != nil && ms.data.project == p {
			m := ms.maps[id]
			if m == nil {
				ms.mu.RUnlock()
				return nil, nil, nil, ErrNoMap
			}
			return m, ms.data, ms.mu.RUnlock, nil
		}
		ms.mu.RUnlock()
		ms.mu.Lock()
		_, _, err = ms.get(id)
		ms.mu.Unlock()
		if err != nil {
			return nil, nil, nil, err
		}
	}
}

// DataInfo describes the item and brush data of the map editor.
type DataInfo struct {
	Ready      bool          `json:"ready"`
	Client     string        `json:"client"`
	Config     MapDataConfig `json:"config"`
	Mode       otitems.Mode  `json:"mode"`
	Items      int           `json:"items"`
	OTBVersion string        `json:"otbVersion"`
	Materials  string        `json:"materials"`
	Brushes    int           `json:"brushes"`
	Tilesets   int           `json:"tilesets"`
	Creatures  int           `json:"creatures"`
	Warnings   []string      `json:"warnings"`
}

// MapInfo describes an open map.
type MapInfo struct {
	ID        int    `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Changed   bool   `json:"changed"`
	CanUndo   bool   `json:"canUndo"`
	CanRedo   bool   `json:"canRedo"`
	UndoLabel string `json:"undoLabel"`
	RedoLabel string `json:"redoLabel"`
	Tiles     int    `json:"tiles"`
	// Floors has a bit set for every floor with tiles.
	Floors    uint16              `json:"floors"`
	Props     mapedit.Properties  `json:"props"`
	Selection int                 `json:"selection"`
	SelMin    otmap.Position      `json:"selMin"`
	SelMax    otmap.Position      `json:"selMax"`
	Rev       uint64              `json:"rev"`
	Bounds    [4]int              `json:"bounds"`
	HasBounds bool                `json:"hasBounds"`
	Towns     []*otmap.Town       `json:"towns"`
	Houses    []mapedit.HouseInfo `json:"houses"`
	Waypoints []*otmap.Waypoint   `json:"waypoints"`
	Zones     []otmap.Zone        `json:"zones"`
}

func (ms *MapService) info(m *openMap, meta bool) MapInfo {
	e := m.ed
	undo, redo := e.History()
	in := MapInfo{
		ID: m.id, Path: m.path, Name: m.name(), Changed: e.Changed(), CanUndo: e.CanUndo(), CanRedo: e.CanRedo(),
		Tiles: e.Map.TileCount(), Props: e.Properties(), Selection: e.SelectionCount(), Rev: e.Map.Rev,
	}
	if len(undo) > 0 {
		in.UndoLabel = undo[len(undo)-1].Label
	}
	if len(redo) > 0 {
		in.RedoLabel = redo[0].Label
	}
	for z, n := range e.Map.FloorTiles() {
		if n > 0 {
			in.Floors |= 1 << z
		}
	}
	in.SelMin, in.SelMax, _ = e.SelectionBounds()
	if meta {
		x0, y0, x1, y1, ok := e.Map.Bounds()
		in.Bounds, in.HasBounds = [4]int{x0, y0, x1, y1}, ok
		in.Towns = e.Map.SortedTowns()
		in.Houses = e.Houses()
		in.Waypoints = slices.Clone(e.Map.Waypoints)
		in.Zones = slices.Clone(e.Map.Zones)
	}
	return in
}

// MapChange is the result of an edit: the chunks to redraw and the map
// state. Meta is set when towns, houses, waypoints or zones changed (Info
// then lists them).
type MapChange struct {
	Chunks []uint32 `json:"chunks"`
	All    bool     `json:"all"`
	Meta   bool     `json:"meta"`
	Info   MapInfo  `json:"info"`
	// Count is the number of affected tiles or items (tools).
	Count    int      `json:"count"`
	Warnings []string `json:"warnings"`
}

func (ms *MapService) change(m *openMap) MapChange {
	chunks, meta := m.ed.TakeDirty()
	keys := make([]uint32, len(chunks))
	for i, k := range chunks {
		keys[i] = uint32(k)
	}
	return MapChange{Chunks: keys, Meta: meta, Info: ms.info(m, meta)}
}

// Status returns the data info, the open maps and the recent maps.
func (ms *MapService) Status() MapStatus {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	st := MapStatus{Recent: ms.store.Recent(), Settings: ms.store.EditorSettings(), Keys: ms.store.Keys()}
	if d, err := ms.ensure(""); err == nil {
		st.Data = ms.dataInfo(d)
	}
	ids := make([]int, 0, len(ms.maps))
	for id := range ms.maps {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		st.Maps = append(st.Maps, ms.info(ms.maps[id], true))
	}
	return st
}

// MapStatus is the state of the map editor.
type MapStatus struct {
	Data     DataInfo         `json:"data"`
	Maps     []MapInfo        `json:"maps"`
	Recent   []RecentMap      `json:"recent"`
	Settings mapedit.Settings `json:"settings"`
	// Keys are the keyboard shortcuts changed by the user.
	Keys map[string][]string `json:"keys"`
}

func (ms *MapService) dataInfo(d *mapData) DataInfo {
	in := DataInfo{
		Ready: true, Client: d.client, Config: d.cfg, Mode: d.items.Mode, Items: d.items.Count(),
		Materials: d.matsPath, Brushes: len(d.mats.Brushes), Tilesets: len(d.mats.Tilesets),
		Creatures: len(d.creatures), Warnings: append(append([]string(nil), d.warnings...), d.brushes.Warnings...),
	}
	if d.items.Mode == otitems.ModeOTB {
		in.OTBVersion = fmt.Sprintf("%d.%d (build %d)", d.items.MajorVersion, d.items.MinorVersion, d.items.BuildNumber)
	}
	return in
}

// DetectData looks for items.otb and items.xml from a folder or map path.
func (ms *MapService) DetectData(path string) MapDataConfig { return detectServerData(path) }

// SetData changes where the item data and brushes come from and reloads
// them.
func (ms *MapService) SetData(cfg MapDataConfig) (DataInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.loadData(&cfg, "")
	if err != nil {
		return DataInfo{}, err
	}
	if err := ms.store.SetProfile(d.client, cfg); err != nil {
		d.warnings = append(d.warnings, "settings: "+err.Error())
	}
	ms.setData(d)
	return ms.dataInfo(d), nil
}

// Reload rebuilds the item data from the client (after object edits).
func (ms *MapService) Reload() (DataInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	cfg := MapDataConfig{}
	if ms.data != nil {
		cfg = ms.data.cfg
	}
	d, err := ms.loadData(&cfg, "")
	if err != nil {
		return DataInfo{}, err
	}
	ms.setData(d)
	return ms.dataInfo(d), nil
}

// SetEditorSettings changes brush behavior settings.
func (ms *MapService) SetEditorSettings(s mapedit.Settings) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, m := range ms.maps {
		m.ed.Settings = s
	}
	return ms.store.SetEditorSettings(s)
}

// SetKeys stores the keyboard shortcuts changed by the user (action id ->
// key combinations); nil restores the defaults.
func (ms *MapService) SetKeys(keys map[string][]string) error {
	return ms.store.SetKeys(keys)
}

func (ms *MapService) add(m *otmap.Map, path string, d *mapData) *openMap {
	ms.next++
	ed := mapedit.New(m, d.items, d.brushes)
	ed.Settings = ms.store.EditorSettings()
	ed.RebuildSelection()
	om := &openMap{id: ms.next, path: path, ed: ed}
	ms.maps[om.id] = om
	return om
}

// NewMapOptions configure a new map.
type NewMapOptions struct {
	Width       uint16            `json:"width"`
	Height      uint16            `json:"height"`
	SpawnFormat otmap.SpawnFormat `json:"spawnFormat"`
	Description string            `json:"description"`
	// Fill paints a ground brush over the whole area (empty: no ground).
	Fill string `json:"fill"`
}

// NewMap creates an empty map.
func (ms *MapService) NewMap(o NewMapOptions) (MapInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	d, err := ms.ensure("")
	if err != nil {
		return MapInfo{}, err
	}
	m := otmap.New(max(o.Width, 32), max(o.Height, 32))
	if o.SpawnFormat == otmap.SpawnsSeparate {
		m.SpawnFormat = otmap.SpawnsSeparate
	}
	m.Description = o.Description
	m.ItemsMajor, m.ItemsMinor = d.items.MajorVersion, d.items.MinorVersion
	om := ms.add(m, "", d)
	if o.Fill != "" && int(m.Width)*int(m.Height) <= 4_000_000 {
		om.ed.Draw(mapedit.DrawRequest{Kind: mapedit.KindGround, Name: o.Fill, Rect: &mapedit.Rect{
			From: otmap.Position{Z: otmap.GroundFloor}, To: otmap.Position{X: m.Width - 1, Y: m.Height - 1, Z: otmap.GroundFloor},
		}})
	}
	om.ed.MarkUnsaved()
	om.ed.TakeDirty()
	return ms.info(om, true), nil
}

// OpenResult is an opened map with load warnings.
type OpenResult struct {
	Info     MapInfo  `json:"info"`
	Warnings []string `json:"warnings"`
}

// EventMapProgress carries a Progress while a map opens: Label is the step,
// Done/Total the whole load in thousandths.
const EventMapProgress = "map:progress"

// loadSteps are the steps of opening a map: label and the share of the
// whole load each takes (start, end in thousandths).
var loadSteps = map[otmap.Stage]struct {
	label      string
	start, end int
}{
	otmap.StageRead:  {"Reading the file", 50, 120},
	otmap.StageTiles: {"Reading tiles", 120, 900},
	otmap.StageFiles: {"Reading houses and spawns", 900, 960},
}

// saveSteps are the steps of saving a map, like loadSteps.
var saveSteps = map[otmap.Stage]struct {
	label      string
	start, end int
}{
	otmap.StageEncode: {"Writing tiles", 0, 800},
	otmap.StageWrite:  {"Writing the files", 800, 1000},
}

// mapProgress returns a progress callback that sends EventMapProgress at
// most once per progressEvery, and always when the step changes.
func (ms *MapService) mapProgress() func(label string, permille int) {
	var last time.Time
	lastLabel := ""
	return func(label string, permille int) {
		if label == lastLabel && time.Since(last) < progressEvery {
			return
		}
		last, lastLabel = time.Now(), label
		ms.s.notify(EventMapProgress, Progress{Label: label, Done: permille, Total: 1000, Percent: true})
	}
}

// OpenMap loads an OTBM map (and its houses, spawns and zones).
func (ms *MapService) OpenMap(path string) (OpenResult, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, m := range ms.maps {
		if strings.EqualFold(filepath.Clean(m.path), filepath.Clean(path)) {
			return OpenResult{Info: ms.info(m, true)}, nil
		}
	}
	report := ms.mapProgress()
	report("Loading items", 0)
	d, err := ms.ensure(path)
	if err != nil {
		return OpenResult{}, err
	}
	m, warns, err := otmap.LoadProgress(path, d.items, ms.store.EditorSettings().Format(), func(stage otmap.Stage, done, total int) {
		st := loadSteps[stage]
		p := st.start
		if total > 0 {
			p += (st.end - st.start) * min(done, total) / total
		}
		report(st.label, p)
	})
	if err != nil {
		return OpenResult{}, err
	}
	if d.items.Mode == otitems.ModeOTB && m.ItemsMinor > d.items.MinorVersion && d.items.MinorVersion != 0 {
		warns = append([]string{fmt.Sprintf("the map was saved with a newer items.otb (client version %d, loaded %d)", m.ItemsMinor, d.items.MinorVersion)}, warns...)
	}
	report("Preparing the editor", 960)
	om := ms.add(m, path, d)
	// Loading leaves as much garbage as the map itself: give it back to the
	// system instead of keeping it for later allocations.
	go debug.FreeOSMemory()
	om.ed.MarkSaved()
	_ = ms.store.AddRecent(RecentMap{Path: path, Client: d.client})
	return OpenResult{Info: ms.info(om, true), Warnings: warns}, nil
}

// SaveMap writes a map to its file (path "" keeps the current file).
func (ms *MapService) SaveMap(id int, path string) (MapChange, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, d, err := ms.get(id)
	if err != nil {
		return MapChange{}, err
	}
	if path == "" {
		path = m.path
	}
	if path == "" {
		return MapChange{}, errors.New("choose where to save the map")
	}
	if !strings.EqualFold(filepath.Ext(path), ".otbm") {
		path += ".otbm"
	}
	if path != m.path && m.path != "" {
		// Save as: the auxiliary files follow the new map name.
		e := m.ed.Map
		e.HouseFile, e.SpawnFile, e.NpcFile = "", "", ""
		if e.ZoneFile != "" {
			e.ZoneFile = ""
		}
	}
	if d.items.Mode == otitems.ModeOTB {
		m.ed.Map.ItemsMajor, m.ed.Map.ItemsMinor = d.items.MajorVersion, d.items.MinorVersion
	}
	report := ms.mapProgress()
	warns, err := otmap.SaveProgress(path, m.ed.Map, d.items, m.ed.Settings.Format(), func(stage otmap.Stage, done, total int) {
		st := saveSteps[stage]
		p := st.start
		if total > 0 {
			p += (st.end - st.start) * min(done, total) / total
		}
		report(st.label, p)
	})
	if err != nil {
		return MapChange{}, err
	}
	m.path = path
	m.ed.MarkSaved()
	_ = ms.store.AddRecent(RecentMap{Path: path, Client: d.client})
	ch := ms.change(m)
	// Saving clears the modified state of every tile.
	ch.All = true
	ch.Warnings = warns
	return ch, nil
}

// CloseMap closes a map without saving.
func (ms *MapService) CloseMap(id int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.maps, id)
	go debug.FreeOSMemory()
}

// AnyChanged reports whether an open map has unsaved changes.
func (ms *MapService) AnyChanged() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, m := range ms.maps {
		if m.ed.Changed() {
			return true
		}
	}
	return false
}

// RemoveRecent forgets a recent map ("" clears the list).
func (ms *MapService) RemoveRecent(path string) error { return ms.store.RemoveRecent(path) }

// Info returns the state of a map (with towns, houses and waypoints).
func (ms *MapService) Info(id int) (MapInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return MapInfo{}, err
	}
	return ms.info(m, true), nil
}

// edit runs fn on a map and returns the change.
func (ms *MapService) edit(id int, fn func(e *mapedit.Editor, d *mapData) (int, error)) (MapChange, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, d, err := ms.get(id)
	if err != nil {
		return MapChange{}, err
	}
	n, err := fn(m.ed, d)
	ch := ms.change(m)
	ch.Count = n
	return ch, err
}

// Undo reverts the last action.
func (ms *MapService) Undo(id int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.Undo(); return 0, nil })
}

// Redo applies the last undone action.
func (ms *MapService) Redo(id int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.Redo(); return 0, nil })
}

// History lists the undo and redo steps.
func (ms *MapService) History(id int) (map[string][]mapedit.ActionInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return nil, err
	}
	u, r := m.ed.History()
	return map[string][]mapedit.ActionInfo{"undo": u, "redo": r}, nil
}

// Draw applies a brush.
func (ms *MapService) Draw(id int, r mapedit.DrawRequest) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, d *mapData) (int, error) {
		if r.Kind == mapedit.KindCreature && !r.Erase {
			if c := d.creatures[strings.ToLower(r.Creature)]; c != nil {
				r.NPC = c.NPC
			}
		}
		return e.Draw(r)
	})
}

// DoodadPreview generates the next placement of a doodad brush.
func (ms *MapService) DoodadPreview(id int, name string, variation, size int, shape string, thickness float64) (*mapedit.DoodadBuffer, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return nil, err
	}
	return m.ed.GenerateDoodad(name, variation, size, shape, thickness), nil
}

// SelectRect selects a rectangle.
func (ms *MapService) SelectRect(id int, r mapedit.Rect, floors string, add, remove, compensate bool) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		e.SelectRect(r, floors, add, remove, compensate)
		return e.SelectionCount(), nil
	})
}

// SelectAt selects the top thing of a tile.
func (ms *MapService) SelectAt(id int, p otmap.Position, toggle, creatures, spawns bool) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		e.SelectAt(p, toggle, creatures, spawns)
		return e.SelectionCount(), nil
	})
}

// SelectItem selects one item of a tile.
func (ms *MapService) SelectItem(id int, p otmap.Position, index int, add bool) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		e.SelectItem(p, index, add)
		return e.SelectionCount(), nil
	})
}

// SelectTiles selects whole tiles.
func (ms *MapService) SelectTiles(id int, ps []otmap.Position, add bool) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		e.SelectTiles(ps, add)
		return e.SelectionCount(), nil
	})
}

// ClearSelection deselects everything.
func (ms *MapService) ClearSelection(id int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { e.ClearSelection(); return 0, nil })
}

// SelectAll selects every tile of a floor.
func (ms *MapService) SelectAll(id int, z int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		e.SelectRect(mapedit.Rect{From: otmap.Position{Z: uint8(z)}, To: otmap.Position{X: 0xFFFF, Y: 0xFFFF, Z: uint8(z)}}, mapedit.FloorsCurrent, false, false, false)
		return e.SelectionCount(), nil
	})
}

// Copy copies the selection to the clipboard (shared by all maps).
func (ms *MapService) Copy(id int) (int, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return 0, err
	}
	c := m.ed.Copy()
	if c == nil {
		return 0, errors.New("nothing is selected")
	}
	ms.clip = c
	return len(c.Tiles), nil
}

// Cut copies the selection and deletes it.
func (ms *MapService) Cut(id int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		c := e.Cut()
		if c == nil {
			return 0, errors.New("nothing is selected")
		}
		ms.clip = c
		return len(c.Tiles), nil
	})
}

// Clipboard returns the copied content for the paste preview.
func (ms *MapService) Clipboard() *mapedit.Clip {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.clip
}

// Paste puts the clipboard with its top-left corner at p.
func (ms *MapService) Paste(id int, p otmap.Position) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		return 0, e.Paste(ms.clip, p)
	})
}

// DeleteSelection removes everything selected.
func (ms *MapService) DeleteSelection(id int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return e.DeleteSelection(""), nil })
}

// MoveSelection moves the selection.
func (ms *MapService) MoveSelection(id int, dx, dy, dz int) (MapChange, error) {
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return 0, e.MoveSelection(dx, dy, dz) })
}

// Tool runs a map tool: "borderizeSelection", "borderizeMap",
// "randomizeSelection", "randomizeMap", "removeCorpses", "removeInvalid",
// "removeUnreachable", "removeEmptySpawns", "clearInvalidHouses",
// "clearModified". inSelection limits item tools to the selection.
func (ms *MapService) Tool(id int, name string, inSelection bool) (MapChange, error) {
	ch, err := ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) {
		switch name {
		case "borderizeSelection":
			return e.BorderizeSelection(), nil
		case "borderizeMap":
			return e.BorderizeMap(), nil
		case "randomizeSelection":
			return e.RandomizeSelection(), nil
		case "randomizeMap":
			return e.RandomizeMap(), nil
		case "removeCorpses":
			return e.RemoveCorpses(inSelection), nil
		case "removeInvalid":
			return e.RemoveInvalid(inSelection), nil
		case "removeUnreachable":
			return e.RemoveUnreachable(), nil
		case "removeEmptySpawns":
			return e.RemoveEmptySpawns(), nil
		case "clearInvalidHouses":
			return e.ClearInvalidHouses(), nil
		case "clearModified":
			e.ClearModified()
			return 0, nil
		}
		return 0, fmt.Errorf("unknown tool %q", name)
	})
	if name == "clearModified" {
		ch.All = true
	}
	return ch, err
}

// FindResult lists found items; More is set when the limit was reached.
type FindResult struct {
	Results []mapedit.FindResult `json:"results"`
	More    bool                 `json:"more"`
}

// Find searches items on the map.
func (ms *MapService) Find(id int, q mapedit.FindQuery) (FindResult, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return FindResult{}, err
	}
	r, more := m.ed.Find(q)
	return FindResult{r, more}, nil
}

// Replace replaces items (to 0 removes them).
func (ms *MapService) Replace(id int, from, to []uint16, inSelection bool) (MapChange, error) {
	if len(from) != len(to) {
		return MapChange{}, errors.New("replace needs pairs of ids")
	}
	pairs := map[uint16]uint16{}
	for i := range from {
		pairs[from[i]] = to[i]
	}
	return ms.edit(id, func(e *mapedit.Editor, _ *mapData) (int, error) { return e.ReplaceItems(pairs, inSelection), nil })
}

// Stats returns map statistics.
func (ms *MapService) Stats(id int) (mapedit.Statistics, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, _, err := ms.get(id)
	if err != nil {
		return mapedit.Statistics{}, err
	}
	return m.ed.Stats(), nil
}
