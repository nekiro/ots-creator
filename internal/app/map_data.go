package app

// Item data of the map editor.
//
// A map stores its items by id, in one of two ways (its item id mode,
// otitems.Mode):
//
//   - server ids (otitems.ModeOTB): the ids of the server, translated to the
//     sprites of the client with the items.otb attached to the client
//     (Session.ItemsOTB, the same file compiling keeps in sync);
//   - client ids (otitems.ModeClient): the ids of the client objects, used as
//     they are, without an items.otb.
//
// The mode belongs to each open map (opened.editor.Items.Mode). It is chosen
// when the map is opened or created (map_item_ids.go) and remembered for the
// map file. The service loads the item data of the open client once per mode
// (ms.data) and every map uses the data of its mode: dataFor.
//
// The server data of the client (items.xml, the brush file and the server
// folder, MapDataConfig) describe the server the client belongs to, so they
// apply to the server's mode: server ids when the client has an items.otb,
// else client ids. Data of the other mode (a map converted to client ids, or
// opened on client ids although the client has an items.otb) takes the names
// of the client and the default brush file of its mode.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/core/project"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

var errNoItemsOTB = errors.New("maps on server item ids need the items.otb of the server: attach it to the client")

type mapData struct {
	project *project.Project
	client  string
	config  MapDataConfig
	// otbPath is the items.otb the server ids were read with ("" for client
	// ids).
	otbPath       string
	items         *otitems.DB
	materials     *materials.Materials
	materialsPath string
	brushes       *mapedit.Brushes
	// creatures holds every known creature by lower case name; the
	// materials file wins over the creatures scanned from the server.
	creatures map[string]*materials.CreatureType
	scanned   []*materials.CreatureType
	warnings  []string
}

type projectSource struct{ things map[uint32]*thing.Thing }

func (source projectSource) Thing(_ thing.Category, id uint32) (*thing.Thing, error) {
	if found, ok := source.things[id]; ok {
		return found, nil
	}
	return nil, errors.New("missing")
}

func clientItems(proj *project.Project) projectSource {
	source := projectSource{things: map[uint32]*thing.Thing{}}
	for _, item := range proj.ThingRange(thing.CategoryItem, thing.CategoryItem.MinID(), proj.Info().Counts.Items) {
		source.things[item.ID] = item
	}
	return source
}

func clientKey(proj *project.Project) string {
	info := proj.Info()
	if info.DatPath == "" {
		return "unsaved client " + info.Version.Name
	}
	return info.DatPath
}

// serverMode is the item id mode of the server the client belongs to:
// server ids when the client has an items.otb attached. New maps and the
// palette without an open map use it.
func (ms *MapService) serverMode() otitems.Mode {
	if ms.session.ItemsOTB() != "" {
		return otitems.ModeOTB
	}
	return otitems.ModeClient
}

// current returns the loaded item data of mode while it fits the open
// client and its items.otb, else nil. Server ids keep their data when the
// items.otb is detached: the maps on them stay readable.
func (ms *MapService) current(mode otitems.Mode) *mapData {
	data := ms.data[mode]
	proj, err := ms.session.Project()
	if data == nil || err != nil || data.project != proj {
		return nil
	}
	if attached := ms.session.ItemsOTB(); mode == otitems.ModeOTB && attached != "" && !fsutil.SamePath(attached, data.otbPath) {
		return nil
	}
	return data
}

// dataOf returns the item data of mode, loading it the first time and
// after the client or its items.otb changed. The caller holds the write
// lock.
func (ms *MapService) dataOf(mode otitems.Mode) (*mapData, error) {
	if data := ms.current(mode); data != nil {
		return data, nil
	}
	proj, err := ms.session.Project()
	if err != nil {
		return nil, errNoClient
	}
	data, err := ms.loadData(proj, mode)
	if err != nil {
		return nil, err
	}
	ms.setData(data)
	return data, nil
}

// dataFor returns the item data of the mode of an open map.
func (ms *MapService) dataFor(opened *openMap) (*mapData, error) {
	return ms.dataOf(opened.editor.Items.Mode)
}

func (ms *MapService) loadData(proj *project.Project, mode otitems.Mode) (*mapData, error) {
	client := clientKey(proj)
	config, _ := ms.store.Profile(client)
	data := &mapData{project: proj, client: client}
	// The server data of the client apply to the server's mode only.
	if mode == ms.serverMode() {
		data.config = config
	} else {
		data.config = MapDataConfig{ServerDir: config.ServerDir}
	}
	switch mode {
	case otitems.ModeOTB:
		data.otbPath = ms.session.ItemsOTB()
		if data.otbPath == "" {
			return nil, errNoItemsOTB
		}
	case otitems.ModeClient:
	default:
		return nil, fmt.Errorf("unknown item ids %q: use %q (server ids) or %q", mode, otitems.ModeOTB, otitems.ModeClient)
	}
	if err := data.loadItems(mode); err != nil {
		return nil, err
	}
	data.loadMaterials(ms.store.Dir())
	if dir := data.config.ServerDir; dir != "" {
		if creatures, err := materials.ScanCreatures(dir); err == nil {
			data.scanned = creatures
		}
	}
	data.index()
	return data, nil
}

// loadItems reads the item database: server ids from the items.otb, client
// ids from the client; then names from items.xml.
func (data *mapData) loadItems(mode otitems.Mode) error {
	source := clientItems(data.project)
	if mode == otitems.ModeOTB {
		otb, err := os.ReadFile(data.otbPath)
		if err != nil {
			return fmt.Errorf("items.otb: %w", err)
		}
		if data.items, err = otitems.ParseOTB(otb); err != nil {
			return fmt.Errorf("items.otb %s: %w", data.otbPath, err)
		}
		data.items.ApplyClient(source)
	} else {
		data.items = otitems.FromClient(source, data.project.Info().Counts.Items, data.project.Names(thing.CategoryItem))
	}
	if xml, err := fsutil.ReadOptional(data.config.XMLPath); err != nil {
		data.warnings = append(data.warnings, "items.xml: "+err.Error())
	} else if xml != nil {
		if err := data.items.ApplyXML(xml); err != nil {
			data.warnings = append(data.warnings, err.Error())
		}
	}
	return nil
}

// loadMaterials reads the brush file: the configured one, else one per
// client version and item id mode in storeDir.
func (data *mapData) loadMaterials(storeDir string) {
	data.materialsPath = data.config.MaterialsPath
	if data.materialsPath == "" && storeDir != "" {
		name := fmt.Sprintf("%d-%s.json", data.project.Info().Version.Value, data.items.Mode)
		data.materialsPath = filepath.Join(storeDir, "materials", name)
	}
	data.materials = materials.New()
	if data.materialsPath == "" {
		return
	}
	if loaded, err := materials.Load(data.materialsPath); err == nil {
		data.materials = loaded
	} else if !os.IsNotExist(err) {
		data.warnings = append(data.warnings, err.Error())
	}
}

func (data *mapData) index() {
	data.brushes = mapedit.NewBrushes(data.items, data.materials)
	data.creatures = map[string]*materials.CreatureType{}
	for _, creature := range data.allCreatures() {
		data.creatures[strings.ToLower(creature.Name)] = creature
	}
}

func (data *mapData) allCreatures() []*materials.CreatureType {
	var all []*materials.CreatureType
	defined := map[string]bool{}
	for _, creature := range data.materials.Creatures {
		defined[strings.ToLower(creature.Name)] = true
		all = append(all, creature)
	}
	for _, creature := range data.scanned {
		if !defined[strings.ToLower(creature.Name)] {
			all = append(all, creature)
		}
	}
	slices.SortFunc(all, func(first, second *materials.CreatureType) int {
		return strings.Compare(strings.ToLower(first.Name), strings.ToLower(second.Name))
	})
	return all
}

func (data *mapData) outfit(creature string) (materials.Outfit, bool) {
	if found := data.creatures[strings.ToLower(creature)]; found != nil {
		return found.Outfit, true
	}
	return materials.Outfit{}, false
}

// setData keeps data for its mode and gives it to the open maps of that
// mode.
func (ms *MapService) setData(data *mapData) {
	mode := data.items.Mode
	ms.data[mode] = data
	for _, opened := range ms.maps {
		if opened.editor.Items.Mode == mode {
			opened.editor.Items = data.items
			opened.editor.SetBrushes(data.brushes)
		}
	}
}

// activeMode is the item id mode of the map in the active tab, else the
// server's.
func (ms *MapService) activeMode() otitems.Mode {
	if opened := ms.maps[ms.activeID]; opened != nil {
		return opened.editor.Items.Mode
	}
	return ms.serverMode()
}

// reloadData reads the item data of every mode in use again (after the
// client objects or the server data changed) and returns the active one.
func (ms *MapService) reloadData() (DataInfo, error) {
	proj, err := ms.session.Project()
	if err != nil {
		return DataInfo{}, errNoClient
	}
	active := ms.activeMode()
	modes := []otitems.Mode{active}
	for mode, data := range ms.data {
		if mode != active && data.project == proj {
			modes = append(modes, mode)
		}
	}
	var result DataInfo
	for _, mode := range modes {
		data, err := ms.loadData(proj, mode)
		if err != nil {
			if mode == active {
				return DataInfo{}, err
			}
			// A map on server ids keeps its data when the items.otb is gone.
			continue
		}
		ms.setData(data)
		if mode == active {
			result = data.info()
		}
	}
	return result, nil
}

type DataInfo struct {
	Ready  bool          `json:"ready"`
	Client string        `json:"client"`
	Config MapDataConfig `json:"config"`
	// Mode is the item id mode the data is for.
	Mode otitems.Mode `json:"mode"`
	// OTBPath is the items.otb the server ids are read with ("" for client
	// ids), OTBVersion its version.
	OTBPath    string   `json:"otbPath"`
	OTBVersion string   `json:"otbVersion"`
	Items      int      `json:"items"`
	Materials  string   `json:"materials"`
	Brushes    int      `json:"brushes"`
	Tilesets   int      `json:"tilesets"`
	Creatures  int      `json:"creatures"`
	Warnings   []string `json:"warnings"`
}

func (data *mapData) info() DataInfo {
	info := DataInfo{
		Ready: true, Client: data.client, Config: data.config, Mode: data.items.Mode, Items: data.items.Count(),
		OTBPath: data.otbPath, Materials: data.materialsPath, Brushes: len(data.materials.Brushes),
		Tilesets: len(data.materials.Tilesets), Creatures: len(data.creatures),
		Warnings: slices.Concat(data.warnings, data.brushes.Warnings),
	}
	if data.items.Mode == otitems.ModeOTB {
		info.OTBVersion = fmt.Sprintf("%d.%d (build %d)", data.items.MajorVersion, data.items.MinorVersion, data.items.BuildNumber)
	}
	return info
}

type MapStatus struct {
	// Data is the item data of the active map, else of the server's mode.
	Data     DataInfo            `json:"data"`
	Maps     []MapInfo           `json:"maps"`
	Recent   []RecentMap         `json:"recent"`
	Settings mapedit.Settings    `json:"settings"`
	Keys     map[string][]string `json:"keys"`
}

func (ms *MapService) Status() MapStatus {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	status := MapStatus{Recent: ms.store.Recent(), Settings: ms.store.EditorSettings(), Keys: ms.store.Keys()}
	if data, err := ms.dataOf(ms.activeMode()); err == nil {
		status.Data = data.info()
	}
	ids := make([]int, 0, len(ms.maps))
	for id := range ms.maps {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		status.Maps = append(status.Maps, ms.info(ms.maps[id], true))
	}
	return status
}

// DetectData looks for the server data (items.otb, items.xml and the data
// folder) in a server folder, a map folder or their parents.
func (ms *MapService) DetectData(path string) ServerData { return detectServerData(path) }

// SetData stores the server data of the open client and reads the item
// data again.
func (ms *MapService) SetData(config MapDataConfig) (DataInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	proj, err := ms.session.Project()
	if err != nil {
		return DataInfo{}, errNoClient
	}
	client := clientKey(proj)
	stored, _ := ms.store.Profile(client)
	config.LegacyOTBPath = stored.LegacyOTBPath
	saveErr := ms.store.SetProfile(client, config)
	info, err := ms.reloadData()
	if err != nil {
		return DataInfo{}, err
	}
	if saveErr != nil {
		info.Warnings = append(info.Warnings, "settings: "+saveErr.Error())
	}
	return info, nil
}

// Reload reads the item data again, after the client objects or the
// items.otb changed.
func (ms *MapService) Reload() (DataInfo, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.reloadData()
}

type EditorPrefs struct {
	Settings mapedit.Settings    `json:"settings"`
	Keys     map[string][]string `json:"keys"`
}

// EditorPrefs gives the map editor settings and keys without loading the
// server data, for the settings dialog of the object editor.
func (ms *MapService) EditorPrefs() EditorPrefs {
	return EditorPrefs{Settings: ms.store.EditorSettings(), Keys: ms.store.Keys()}
}

func (ms *MapService) SetEditorSettings(settings mapedit.Settings) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, opened := range ms.maps {
		opened.editor.Settings = settings
	}
	return ms.store.SetEditorSettings(settings)
}

// SetKeys stores the keyboard shortcuts changed by the user (action id ->
// key combinations); nil restores the defaults.
func (ms *MapService) SetKeys(keys map[string][]string) error {
	return ms.store.SetKeys(keys)
}
