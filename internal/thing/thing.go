// Package thing contains the version-independent model of client objects
// (items, outfits, effects and missiles). Codecs translate between this
// model and concrete file formats.
package thing

import (
	"fmt"
	"slices"
)

// Category identifies the kind of a thing. Numeric values match OBD files.
type Category uint8

const (
	CategoryInvalid Category = 0
	CategoryItem    Category = 1
	CategoryOutfit  Category = 2
	CategoryEffect  Category = 3
	CategoryMissile Category = 4
)

// Categories lists all valid categories in file order.
var Categories = []Category{CategoryItem, CategoryOutfit, CategoryEffect, CategoryMissile}

// String returns the lowercase name used by ObjectBuilder ("item", "outfit"...).
func (c Category) String() string {
	switch c {
	case CategoryItem:
		return "item"
	case CategoryOutfit:
		return "outfit"
	case CategoryEffect:
		return "effect"
	case CategoryMissile:
		return "missile"
	}
	return "invalid"
}

// ParseCategory converts a name produced by String back to a Category.
func ParseCategory(s string) (Category, error) {
	for _, c := range Categories {
		if c.String() == s {
			return c, nil
		}
	}
	return CategoryInvalid, fmt.Errorf("invalid thing category %q", s)
}

// Valid reports whether c is one of the four known categories.
func (c Category) Valid() bool { return c >= CategoryItem && c <= CategoryMissile }

// MinID returns the first id used by a category in Tibia.dat.
func (c Category) MinID() uint32 {
	if c == CategoryItem {
		return 100
	}
	return 1
}

// DefaultDuration returns the default frame duration in milliseconds used
// when a client has no per-frame durations.
func (c Category) DefaultDuration() uint32 {
	switch c {
	case CategoryItem:
		return 500
	case CategoryOutfit:
		return 300
	case CategoryEffect:
		return 100
	case CategoryMissile:
		return 75
	}
	return 100
}

// FrameGroupType identifies an outfit frame group.
type FrameGroupType uint8

const (
	FrameGroupDefault FrameGroupType = 0 // idle
	FrameGroupWalking FrameGroupType = 1
)

// Direction indexes outfit directions. Order matches pattern X in outfits.
type Direction uint8

const (
	North Direction = 0
	East  Direction = 1
	South Direction = 2
	West  Direction = 3
)

// Point is a signed 2D offset.
type Point struct {
	X int16 `json:"x"`
	Y int16 `json:"y"`
}

// Market holds the market flag payload.
type Market struct {
	Category           uint16 `json:"category"`
	TradeAs            uint16 `json:"tradeAs"`
	ShowAs             uint16 `json:"showAs"`
	Name               string `json:"name"`
	RestrictProfession uint16 `json:"restrictProfession"`
	RestrictLevel      uint16 `json:"restrictLevel"`
}

// Properties holds every flag a thing can have across all supported client
// versions. Boolean "Has..." fields gate their payload fields.
type Properties struct {
	Ground           bool   `json:"ground"`
	GroundSpeed      uint16 `json:"groundSpeed"`
	GroundBorder     bool   `json:"groundBorder"`
	OnBottom         bool   `json:"onBottom"`
	OnTop            bool   `json:"onTop"`
	Container        bool   `json:"container"`
	Stackable        bool   `json:"stackable"`
	ForceUse         bool   `json:"forceUse"`
	MultiUse         bool   `json:"multiUse"`
	HasCharges       bool   `json:"hasCharges"`
	Writable         bool   `json:"writable"`
	MaxTextLength    uint16 `json:"maxTextLength"`
	WritableOnce     bool   `json:"writableOnce"`
	MaxReadLength    uint16 `json:"maxReadLength"`
	FluidContainer   bool   `json:"fluidContainer"`
	Fluid            bool   `json:"fluid"`
	Unpassable       bool   `json:"unpassable"`
	Unmoveable       bool   `json:"unmoveable"`
	BlockMissile     bool   `json:"blockMissile"`
	BlockPathfind    bool   `json:"blockPathfind"`
	NoMoveAnimation  bool   `json:"noMoveAnimation"`
	Pickupable       bool   `json:"pickupable"`
	Hangable         bool   `json:"hangable"`
	HookSouth        bool   `json:"hookSouth"` // "vertical" in older tools
	HookEast         bool   `json:"hookEast"`  // "horizontal" in older tools
	Rotatable        bool   `json:"rotatable"`
	HasLight         bool   `json:"hasLight"`
	LightLevel       uint16 `json:"lightLevel"`
	LightColor       uint16 `json:"lightColor"`
	DontHide         bool   `json:"dontHide"`
	Translucent      bool   `json:"translucent"`
	FloorChange      bool   `json:"floorChange"`
	HasOffset        bool   `json:"hasOffset"`
	OffsetX          int16  `json:"offsetX"`
	OffsetY          int16  `json:"offsetY"`
	HasElevation     bool   `json:"hasElevation"`
	Elevation        uint16 `json:"elevation"`
	LyingObject      bool   `json:"lyingObject"`
	AnimateAlways    bool   `json:"animateAlways"`
	MiniMap          bool   `json:"miniMap"`
	MiniMapColor     uint16 `json:"miniMapColor"`
	LensHelp         bool   `json:"lensHelp"`
	LensHelpValue    uint16 `json:"lensHelpValue"`
	FullGround       bool   `json:"fullGround"`
	IgnoreLook       bool   `json:"ignoreLook"`
	Cloth            bool   `json:"cloth"`
	ClothSlot        uint16 `json:"clothSlot"`
	IsMarket         bool   `json:"isMarket"`
	Market           Market `json:"market"`
	HasDefaultAction bool   `json:"hasDefaultAction"`
	DefaultAction    uint16 `json:"defaultAction"`
	Wrappable        bool   `json:"wrappable"`
	Unwrappable      bool   `json:"unwrappable"`
	TopEffect        bool   `json:"topEffect"`
	Usable           bool   `json:"usable"`
	HasBones         bool   `json:"hasBones"`
	// Bones holds offsets indexed by Direction (north, east, south, west).
	Bones [4]Point `json:"bones"`
}

// AnimationMode controls how an animation starts.
type AnimationMode uint8

const (
	AnimationAsync AnimationMode = 0
	AnimationSync  AnimationMode = 1
)

// FrameDuration is the duration range of one animation frame in milliseconds.
type FrameDuration struct {
	Min uint32 `json:"min"`
	Max uint32 `json:"max"`
}

// FrameGroup holds the sprite layout and animation of one frame group.
type FrameGroup struct {
	Type       FrameGroupType  `json:"type"`
	Width      uint8           `json:"width"`
	Height     uint8           `json:"height"`
	ExactSize  uint8           `json:"exactSize"`
	Layers     uint8           `json:"layers"`
	PatternX   uint8           `json:"patternX"`
	PatternY   uint8           `json:"patternY"`
	PatternZ   uint8           `json:"patternZ"`
	Frames     uint8           `json:"frames"`
	Mode       AnimationMode   `json:"mode"`
	LoopCount  int32           `json:"loopCount"`
	StartFrame int8            `json:"startFrame"`
	Durations  []FrameDuration `json:"durations"`
	Sprites    []uint32        `json:"sprites"`
}

// NewFrameGroup returns a 1x1 group with one empty sprite slot.
func NewFrameGroup() *FrameGroup {
	g := &FrameGroup{Width: 1, Height: 1, ExactSize: 32, Layers: 1, PatternX: 1, PatternY: 1, PatternZ: 1, Frames: 1}
	g.Sprites = make([]uint32, 1)
	return g
}

// IsAnimation reports whether the group has more than one frame.
func (g *FrameGroup) IsAnimation() bool { return g.Frames > 1 }

// TotalSprites returns the number of sprite slots described by the layout.
func (g *FrameGroup) TotalSprites() int {
	return int(g.Width) * int(g.Height) * int(g.Layers) * int(g.PatternX) * int(g.PatternY) * int(g.PatternZ) * int(g.Frames)
}

// TotalTextures returns the number of composed textures (sprites per tile area).
func (g *FrameGroup) TotalTextures() int {
	return int(g.Layers) * int(g.PatternX) * int(g.PatternY) * int(g.PatternZ) * int(g.Frames)
}

// SpriteIndex returns the slot index of one 32x32 piece. Parameter order
// mirrors the client: w/h are tile offsets inside the texture.
func (g *FrameGroup) SpriteIndex(w, h, layer, px, py, pz, frame int) int {
	frames := max(int(g.Frames), 1)
	return ((((((frame%frames)*int(g.PatternZ)+pz)*int(g.PatternY)+py)*int(g.PatternX)+px)*int(g.Layers)+layer)*int(g.Height)+h)*int(g.Width) + w
}

// TextureIndex returns the index of a composed texture.
func (g *FrameGroup) TextureIndex(layer, px, py, pz, frame int) int {
	frames := max(int(g.Frames), 1)
	return ((((frame%frames)*int(g.PatternZ)+pz)*int(g.PatternY)+py)*int(g.PatternX)+px)*int(g.Layers) + layer
}

// SheetSize returns the pixel size of the sprite sheet used for import/export.
// Columns: patternZ*patternX*layers textures, rows: frames*patternY textures.
func (g *FrameGroup) SheetSize(spriteSize int) (w, h int) {
	w = int(g.PatternZ) * int(g.PatternX) * int(g.Layers) * int(g.Width) * spriteSize
	h = int(g.Frames) * int(g.PatternY) * int(g.Height) * spriteSize
	return
}

// Clone returns a deep copy.
func (g *FrameGroup) Clone() *FrameGroup {
	c := *g
	c.Durations = slices.Clone(g.Durations)
	c.Sprites = slices.Clone(g.Sprites)
	return &c
}

// EnsureDurations makes the durations slice match the frame count, filling
// new entries with the given default.
func (g *FrameGroup) EnsureDurations(def uint32) {
	n := int(g.Frames)
	if n <= 1 {
		g.Durations = nil
		return
	}
	for len(g.Durations) < n {
		g.Durations = append(g.Durations, FrameDuration{Min: def, Max: def})
	}
	g.Durations = g.Durations[:n]
}

// Resize changes the layout and keeps existing sprite ids where the
// (w,h,layer,px,py,pz,frame) coordinates still exist.
func (g *FrameGroup) Resize(width, height, layers, px, py, pz, frames uint8, defDuration uint32) {
	old := g.Clone()
	g.Width, g.Height, g.Layers = width, height, layers
	g.PatternX, g.PatternY, g.PatternZ, g.Frames = px, py, pz, frames
	g.Sprites = make([]uint32, g.TotalSprites())
	for f := 0; f < int(min(old.Frames, frames)); f++ {
		for z := 0; z < int(min(old.PatternZ, pz)); z++ {
			for y := 0; y < int(min(old.PatternY, py)); y++ {
				for x := 0; x < int(min(old.PatternX, px)); x++ {
					for l := 0; l < int(min(old.Layers, layers)); l++ {
						for hh := 0; hh < int(min(old.Height, height)); hh++ {
							for ww := 0; ww < int(min(old.Width, width)); ww++ {
								g.Sprites[g.SpriteIndex(ww, hh, l, x, y, z, f)] = old.Sprites[old.SpriteIndex(ww, hh, l, x, y, z, f)]
							}
						}
					}
				}
			}
		}
	}
	g.EnsureDurations(defDuration)
	if int(g.StartFrame) >= int(frames) {
		g.StartFrame = 0
	}
}

// Validate checks internal consistency.
func (g *FrameGroup) Validate() error {
	if g.Width == 0 || g.Height == 0 || g.Layers == 0 || g.PatternX == 0 || g.PatternY == 0 || g.PatternZ == 0 || g.Frames == 0 {
		return fmt.Errorf("frame group has a zero dimension")
	}
	if len(g.Sprites) != g.TotalSprites() {
		return fmt.Errorf("frame group has %d sprites, layout needs %d", len(g.Sprites), g.TotalSprites())
	}
	if g.IsAnimation() && len(g.Durations) != int(g.Frames) {
		return fmt.Errorf("frame group has %d durations for %d frames", len(g.Durations), g.Frames)
	}
	return nil
}

// Thing is one client object.
type Thing struct {
	ID          uint32        `json:"id"`
	Category    Category      `json:"category"`
	Props       Properties    `json:"props"`
	FrameGroups []*FrameGroup `json:"frameGroups"`
}

// New returns an empty thing with one default frame group. Outfits get four
// directions like ObjectBuilder does.
func New(id uint32, c Category) *Thing {
	g := NewFrameGroup()
	if c == CategoryOutfit {
		g.PatternX = 4
		g.Sprites = make([]uint32, g.TotalSprites())
	}
	return &Thing{ID: id, Category: c, FrameGroups: []*FrameGroup{g}}
}

// Group returns the frame group of the given type, falling back to the first.
func (t *Thing) Group(ft FrameGroupType) *FrameGroup {
	if int(ft) < len(t.FrameGroups) {
		return t.FrameGroups[ft]
	}
	if len(t.FrameGroups) > 0 {
		return t.FrameGroups[0]
	}
	return nil
}

// Clone returns a deep copy.
func (t *Thing) Clone() *Thing {
	c := *t
	c.FrameGroups = make([]*FrameGroup, len(t.FrameGroups))
	for i, g := range t.FrameGroups {
		c.FrameGroups[i] = g.Clone()
	}
	return &c
}

// SpriteIDs returns every sprite id referenced by all frame groups.
func (t *Thing) SpriteIDs() []uint32 {
	var ids []uint32
	for _, g := range t.FrameGroups {
		ids = append(ids, g.Sprites...)
	}
	return ids
}

// Validate checks the thing and its frame groups.
func (t *Thing) Validate() error {
	if !t.Category.Valid() {
		return fmt.Errorf("thing %d: invalid category %d", t.ID, t.Category)
	}
	if len(t.FrameGroups) == 0 {
		return fmt.Errorf("%s %d: no frame groups", t.Category, t.ID)
	}
	if t.Category != CategoryOutfit && len(t.FrameGroups) > 1 {
		return fmt.Errorf("%s %d: only outfits can have more than one frame group", t.Category, t.ID)
	}
	for i, g := range t.FrameGroups {
		if err := g.Validate(); err != nil {
			return fmt.Errorf("%s %d group %d: %w", t.Category, t.ID, i, err)
		}
	}
	return nil
}
