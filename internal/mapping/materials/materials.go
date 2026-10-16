// Package materials holds the brush definitions of the map editor: borders,
// ground, wall, carpet, table and doodad brushes, tilesets and creature
// types. Materials are stored as JSON (the editor's own format) and can be
// imported from a Remere's Map Editor data folder.
package materials

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nekiro/ots-creator/internal/fsutil"
)

// FormatVersion is the version of the JSON format.
const FormatVersion = 1

// Brush types.
const (
	TypeGround     = "ground"
	TypeWall       = "wall"
	TypeDecoration = "decoration"
	TypeDoodad     = "doodad"
	TypeCarpet     = "carpet"
	TypeTable      = "table"
)

// Edge names of ground borders and carpets.
var EdgeNames = []string{"n", "e", "s", "w", "cnw", "cne", "csw", "cse", "dnw", "dne", "dse", "dsw"}

// Wall alignment names (RME names; "corner" is the northwest diagonal).
var WallAlignments = []string{
	"pole", "south end", "east end", "corner", "west end", "northeast diagonal", "horizontal", "south T",
	"north end", "vertical", "southwest diagonal", "east T", "southeast diagonal", "west T", "north T",
	"intersection", "untouchable",
}

// Table alignment names.
var TableAlignments = []string{"north", "south", "east", "west", "horizontal", "vertical", "alone"}

// Door types.
var DoorTypes = []string{"archway", "normal", "locked", "quest", "magic", "window", "hatch window"}

// Materials is a complete brush set.
type Materials struct {
	Version   int             `json:"version"`
	Borders   []*Border       `json:"borders"`
	Brushes   []*Brush        `json:"brushes"`
	Tilesets  []*Tileset      `json:"tilesets"`
	Creatures []*CreatureType `json:"creatures"`
	// MetaItems are editor-only items that are never saved.
	MetaItems []uint16 `json:"metaItems,omitempty"`
}

// Border is a set of border items by edge.
type Border struct {
	ID    uint32            `json:"id"`
	Group uint16            `json:"group,omitempty"`
	Edges map[string]uint16 `json:"edges"`
	// GroundEquivalent marks an inline border whose items replace the
	// ground with this item.
	GroundEquivalent uint16 `json:"groundEquivalent,omitempty"`
	// Optional marks a border only placed with the optional border tool.
	Optional bool `json:"optional,omitempty"`
}

// Chance is an item with a relative chance.
type Chance struct {
	ID     uint16 `json:"id"`
	Chance int    `json:"chance"`
}

// ItemSpec is an item placed by a doodad, with optional attributes.
type ItemSpec struct {
	ID       uint16 `json:"id"`
	Subtype  uint16 `json:"subtype,omitempty"`
	ActionID uint16 `json:"actionId,omitempty"`
	UniqueID uint16 `json:"uniqueId,omitempty"`
	Text     string `json:"text,omitempty"`
}

// BorderRule tells a ground brush which border to place against what.
type BorderRule struct {
	// Align is "outer" (the border lies on the neighbor) or "inner".
	Align string `json:"align"`
	// To is "all" (default), "none" (against empty tiles) or a brush name.
	To     string `json:"to,omitempty"`
	Border uint32 `json:"border,omitempty"`
	// Inline is a border defined in the rule (with a ground equivalent).
	Inline   *Border        `json:"inline,omitempty"`
	Super    bool           `json:"super,omitempty"`
	Specific []SpecificCase `json:"specific,omitempty"`
}

// SpecificCase replaces border items when all conditions match.
type SpecificCase struct {
	Conditions []Condition `json:"conditions"`
	Actions    []Action    `json:"actions"`
}

// Condition kinds: "border" (border id + edge), "group" (group + edge),
// "item" (item id).
type Condition struct {
	Kind   string `json:"kind"`
	Border uint32 `json:"border,omitempty"`
	Edge   string `json:"edge,omitempty"`
	Group  uint16 `json:"group,omitempty"`
	Item   uint16 `json:"item,omitempty"`
}

// Action kinds: "replaceBorder" (border id + edge -> with), "replaceItem"
// (item -> with), "deleteBorders".
type Action struct {
	Kind   string `json:"kind"`
	Border uint32 `json:"border,omitempty"`
	Edge   string `json:"edge,omitempty"`
	Item   uint16 `json:"item,omitempty"`
	With   uint16 `json:"with,omitempty"`
}

// WallPart is the items of one wall alignment.
type WallPart struct {
	Items []Chance `json:"items"`
	Doors []Door   `json:"doors,omitempty"`
}

// Door is a door or window of a wall alignment.
type Door struct {
	ID   uint16 `json:"id"`
	Type string `json:"type"`
	Open bool   `json:"open"`
	// Hate keeps neighbor walls from connecting to it.
	Hate bool `json:"hate,omitempty"`
}

// DoodadOptions are the placement options of a doodad brush.
type DoodadOptions struct {
	OnBlocking  bool `json:"onBlocking,omitempty"`
	OnDuplicate bool `json:"onDuplicate,omitempty"`
	RedoBorders bool `json:"redoBorders,omitempty"`
	OneSize     bool `json:"oneSize,omitempty"`
	Draggable   bool `json:"draggable,omitempty"`
	// Thickness/ThicknessCeiling is the density of random placement.
	Thickness            int  `json:"thickness,omitempty"`
	ThicknessCeiling     int  `json:"thicknessCeiling,omitempty"`
	RemoveOptionalBorder bool `json:"removeOptionalBorder,omitempty"`
}

// Alternative is one variation of a doodad brush.
type Alternative struct {
	Singles    []Single    `json:"singles,omitempty"`
	Composites []Composite `json:"composites,omitempty"`
}

// Single is one item placed alone.
type Single struct {
	Chance int      `json:"chance"`
	Item   ItemSpec `json:"item"`
}

// Composite is a group of tiles placed together.
type Composite struct {
	Chance int             `json:"chance"`
	Tiles  []CompositeTile `json:"tiles"`
}

// CompositeTile is one tile of a composite, relative to the click.
type CompositeTile struct {
	X     int        `json:"x"`
	Y     int        `json:"y"`
	Z     int        `json:"z,omitempty"`
	Items []ItemSpec `json:"items"`
}

// Brush is one brush definition. Fields are used depending on Type.
type Brush struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Look is the item shown in the palette.
	Look uint16 `json:"look,omitempty"`

	// Ground
	Items        []Chance     `json:"items,omitempty"`
	ZOrder       int          `json:"zOrder,omitempty"`
	NoRandomize  bool         `json:"noRandomize,omitempty"`
	SoloOptional bool         `json:"soloOptional,omitempty"`
	Optional     *BorderRule  `json:"optional,omitempty"`
	Borders      []BorderRule `json:"borders,omitempty"`

	// Ground and wall
	Friends     []string `json:"friends,omitempty"`
	HateFriends bool     `json:"hateFriends,omitempty"`

	// Wall and decoration
	Walls    map[string]*WallPart `json:"walls,omitempty"`
	Redirect string               `json:"redirect,omitempty"`

	// Doodad (walls and grounds can carry doodad variations too)
	Doodad       *DoodadOptions `json:"doodad,omitempty"`
	Alternatives []Alternative  `json:"alternatives,omitempty"`

	// Carpet and table pieces by alignment
	Parts map[string][]Chance `json:"parts,omitempty"`
}

// Tileset groups brushes and items in the palette.
type Tileset struct {
	Name string `json:"name"`
	// Terrain lists ground, wall, carpet and table brushes.
	Terrain []string `json:"terrain,omitempty"`
	Doodads []string `json:"doodads,omitempty"`
	// Items are placed as single items (item palette).
	Items []uint16 `json:"items,omitempty"`
	// Raw are shown in the raw palette.
	Raw []uint16 `json:"raw,omitempty"`
	// Auto marks tilesets generated from item flags.
	Auto bool `json:"auto,omitempty"`
}

// Outfit is the look of a creature.
type Outfit struct {
	LookType uint16 `json:"lookType"`
	Head     uint8  `json:"head,omitempty"`
	Body     uint8  `json:"body,omitempty"`
	Legs     uint8  `json:"legs,omitempty"`
	Feet     uint8  `json:"feet,omitempty"`
	Addons   uint8  `json:"addons,omitempty"`
	Mount    uint16 `json:"mount,omitempty"`
	// LookItem shows an item instead of an outfit (server id).
	LookItem uint16 `json:"lookItem,omitempty"`
}

// CreatureType is a monster or NPC available in the palette.
type CreatureType struct {
	Name   string `json:"name"`
	NPC    bool   `json:"npc,omitempty"`
	Outfit Outfit `json:"outfit"`
}

// New returns empty materials.
func New() *Materials { return &Materials{Version: FormatVersion} }

// Load reads materials from a JSON file.
func Load(path string) (*Materials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse decodes materials JSON.
func Parse(data []byte) (*Materials, error) {
	materials := New()
	if err := json.Unmarshal(data, materials); err != nil {
		return nil, fmt.Errorf("materials: %w", err)
	}
	if materials.Version > FormatVersion {
		return nil, fmt.Errorf("materials: version %d is newer than this editor supports", materials.Version)
	}
	materials.Version = FormatVersion
	return materials, nil
}

// Save writes materials as indented JSON.
func (m *Materials) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, data)
}

// Brush returns a brush by name (case-insensitive) or nil.
func (m *Materials) Brush(name string) *Brush {
	for _, brush := range m.Brushes {
		if strings.EqualFold(brush.Name, name) {
			return brush
		}
	}
	return nil
}

// Border returns a border by id or nil.
func (m *Materials) Border(id uint32) *Border {
	for _, border := range m.Borders {
		if border.ID == id {
			return border
		}
	}
	return nil
}

// Tileset returns a tileset by name or nil.
func (m *Materials) Tileset(name string) *Tileset {
	for _, tileset := range m.Tilesets {
		if strings.EqualFold(tileset.Name, name) {
			return tileset
		}
	}
	return nil
}

// Creature returns a creature type by name (case-insensitive) or nil.
func (m *Materials) Creature(name string) *CreatureType {
	for _, creature := range m.Creatures {
		if strings.EqualFold(creature.Name, name) {
			return creature
		}
	}
	return nil
}

// NextBorderID returns an unused border id.
func (m *Materials) NextBorderID() uint32 {
	var id uint32
	for _, border := range m.Borders {
		id = max(id, border.ID)
	}
	return id + 1
}

// RemoveBrush deletes a brush and its tileset entries.
func (m *Materials) RemoveBrush(name string) {
	m.Brushes = slices.DeleteFunc(m.Brushes, func(brush *Brush) bool { return strings.EqualFold(brush.Name, name) })
	for _, tileset := range m.Tilesets {
		matchesName := func(entry string) bool { return strings.EqualFold(entry, name) }
		tileset.Terrain = slices.DeleteFunc(tileset.Terrain, matchesName)
		tileset.Doodads = slices.DeleteFunc(tileset.Doodads, matchesName)
	}
}

// RenameBrush renames a brush and every reference to it.
func (m *Materials) RenameBrush(oldName, name string) {
	isOld := func(entry string) bool { return strings.EqualFold(entry, oldName) }
	for _, brush := range m.Brushes {
		if isOld(brush.Name) {
			brush.Name = name
		}
		for i := range brush.Friends {
			if isOld(brush.Friends[i]) {
				brush.Friends[i] = name
			}
		}
		if isOld(brush.Redirect) {
			brush.Redirect = name
		}
		for i := range brush.Borders {
			if isOld(brush.Borders[i].To) {
				brush.Borders[i].To = name
			}
		}
	}
	for _, tileset := range m.Tilesets {
		for i := range tileset.Terrain {
			if isOld(tileset.Terrain[i]) {
				tileset.Terrain[i] = name
			}
		}
		for i := range tileset.Doodads {
			if isOld(tileset.Doodads[i]) {
				tileset.Doodads[i] = name
			}
		}
	}
}

// Merge adds what other has and m does not (by brush name, border id,
// tileset name and creature name). Tilesets with the same name get the
// missing entries.
func (m *Materials) Merge(other *Materials) {
	renumbered := map[uint32]uint32{}
	for _, border := range other.Borders {
		if m.Border(border.ID) == nil {
			m.Borders = append(m.Borders, border)
			continue
		}
		// Clashing ids: renumber the imported border.
		renumberedBorder := *border
		renumberedBorder.ID = m.NextBorderID()
		renumbered[border.ID] = renumberedBorder.ID
		m.Borders = append(m.Borders, &renumberedBorder)
	}
	remap := func(id uint32) uint32 {
		if newID, ok := renumbered[id]; ok {
			return newID
		}
		return id
	}
	for _, brush := range other.Brushes {
		if m.Brush(brush.Name) != nil {
			continue
		}
		for i := range brush.Borders {
			brush.Borders[i].Border = remap(brush.Borders[i].Border)
		}
		if brush.Optional != nil {
			brush.Optional.Border = remap(brush.Optional.Border)
		}
		m.Brushes = append(m.Brushes, brush)
	}
	for _, tileset := range other.Tilesets {
		existing := m.Tileset(tileset.Name)
		if existing == nil {
			m.Tilesets = append(m.Tilesets, tileset)
			continue
		}
		existing.Terrain = appendMissing(existing.Terrain, tileset.Terrain)
		existing.Doodads = appendMissing(existing.Doodads, tileset.Doodads)
		existing.Items = appendMissingIDs(existing.Items, tileset.Items)
		existing.Raw = appendMissingIDs(existing.Raw, tileset.Raw)
	}
	for _, creature := range other.Creatures {
		if m.Creature(creature.Name) == nil {
			m.Creatures = append(m.Creatures, creature)
		}
	}
	m.MetaItems = appendMissingIDs(m.MetaItems, other.MetaItems)
}

func appendMissing(list, additions []string) []string {
	for _, entry := range additions {
		if !slices.ContainsFunc(list, func(present string) bool { return strings.EqualFold(present, entry) }) {
			list = append(list, entry)
		}
	}
	return list
}

func appendMissingIDs(list, additions []uint16) []uint16 {
	for _, entry := range additions {
		if !slices.Contains(list, entry) {
			list = append(list, entry)
		}
	}
	return list
}
