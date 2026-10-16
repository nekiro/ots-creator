package app

import (
	"math"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

// PaletteBrush is a brush shown in the palette.
type PaletteBrush struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Look is the server id of the item shown.
	Look uint16 `json:"look"`
	// Preview shows a brush that places a group of tiles (a stamp made from
	// a selection) as the whole group; nil for one-tile brushes.
	Preview *BrushPreview `json:"preview,omitempty"`
}

// BrushPreview is the ground floor of the first group of a doodad brush,
// at most previewMax tiles a side: W x H tiles, items bottom to top.
type BrushPreview struct {
	W     int           `json:"w"`
	H     int           `json:"h"`
	Tiles []PreviewTile `json:"tiles"`
}

// PreviewTile is one tile of a BrushPreview.
type PreviewTile struct {
	X     int      `json:"x"`
	Y     int      `json:"y"`
	Items []uint16 `json:"items"`
}

const previewMax = 12

func brushPreview(brush *materials.Brush) *BrushPreview {
	if len(brush.Alternatives) == 0 || len(brush.Alternatives[0].Composites) == 0 {
		return nil
	}
	tiles := brush.Alternatives[0].Composites[0].Tiles
	minX, minY, maxX, maxY := math.MaxInt, math.MaxInt, math.MinInt, math.MinInt
	for _, tile := range tiles {
		if tile.Z == 0 {
			minX, minY, maxX, maxY = min(minX, tile.X), min(minY, tile.Y), max(maxX, tile.X), max(maxY, tile.Y)
		}
	}
	if minX > maxX || (minX == maxX && minY == maxY) {
		return nil
	}
	preview := &BrushPreview{W: min(maxX-minX+1, previewMax), H: min(maxY-minY+1, previewMax)}
	for _, tile := range tiles {
		x, y := tile.X-minX, tile.Y-minY
		if tile.Z != 0 || x >= preview.W || y >= preview.H {
			continue
		}
		previewTile := PreviewTile{X: x, Y: y}
		for _, item := range tile.Items {
			previewTile.Items = append(previewTile.Items, item.ID)
		}
		preview.Tiles = append(preview.Tiles, previewTile)
	}
	return preview
}

// PaletteTileset is a tileset with resolved brushes.
type PaletteTileset struct {
	Name    string         `json:"name"`
	Auto    bool           `json:"auto"`
	Terrain []PaletteBrush `json:"terrain"`
	Doodads []PaletteBrush `json:"doodads"`
	Items   []uint16       `json:"items"`
	Raw     []uint16       `json:"raw"`
}

// Palette is everything the brush palette shows.
type Palette struct {
	Tilesets  []PaletteTileset          `json:"tilesets"`
	Brushes   []PaletteBrush            `json:"brushes"`
	Creatures []*materials.CreatureType `json:"creatures"`
	DoorTypes []string                  `json:"doorTypes"`
}

// Palette returns tilesets, brushes and creatures.
func (ms *MapService) Palette() (palette Palette, err error) {
	err = ms.withData(func(data *mapData) error {
		palette = data.palette()
		return nil
	})
	return palette, err
}

func (data *mapData) palette() Palette {
	palette := Palette{DoorTypes: materials.DoorTypes, Creatures: data.allCreatures()}
	for _, brush := range data.materials.Brushes {
		if paletteBrush, ok := data.paletteBrush(brush.Name); ok {
			palette.Brushes = append(palette.Brushes, paletteBrush)
		}
	}
	tilesets := slices.Concat(data.materials.Tilesets, materials.AutoTilesets(data.items))
	for _, tileset := range tilesets {
		paletteTileset := PaletteTileset{Name: tileset.Name, Auto: tileset.Auto, Items: tileset.Items, Raw: tileset.Raw}
		paletteTileset.Terrain = data.paletteBrushes(tileset.Terrain)
		paletteTileset.Doodads = data.paletteBrushes(tileset.Doodads)
		palette.Tilesets = append(palette.Tilesets, paletteTileset)
	}
	return palette
}

// paletteBrushes resolves brush names, leaving out unknown ones.
func (data *mapData) paletteBrushes(names []string) []PaletteBrush {
	var out []PaletteBrush
	for _, name := range names {
		if paletteBrush, ok := data.paletteBrush(name); ok {
			out = append(out, paletteBrush)
		}
	}
	return out
}

func (data *mapData) paletteBrush(name string) (PaletteBrush, bool) {
	brush := data.materials.Brush(name)
	if brush == nil {
		return PaletteBrush{}, false
	}
	kind := brush.Type
	// A wall brush without walls only places its alternatives.
	if brush.Type == materials.TypeWall && len(brush.Walls) == 0 && len(brush.Alternatives) > 0 {
		kind = materials.TypeDoodad
	}
	look := data.brushes.Look(brush.Name)
	if look == 0 {
		look = brush.Look
	}
	return PaletteBrush{Name: brush.Name, Kind: kind, Look: look, Preview: brushPreview(brush)}, true
}
