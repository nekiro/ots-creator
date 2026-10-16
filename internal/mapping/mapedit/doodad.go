package mapedit

import (
	"cmp"
	"math"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// DoodadTile is one tile of a doodad placement, relative to the cursor.
type DoodadTile struct {
	X     int          `json:"x"`
	Y     int          `json:"y"`
	Z     int          `json:"z"`
	Items []otmap.Item `json:"items"`
	// Replace clears the tile first: a tile of a group that brings its own
	// ground (a stamp made from a selection) replaces what is there.
	Replace bool `json:"replace,omitempty"`
}

// DoodadBuffer is the next placement of a doodad brush. It is generated
// before the click so the editor can preview exactly what will be placed.
type DoodadBuffer struct {
	Brush     string       `json:"brush"`
	Variation int          `json:"variation"`
	Size      int          `json:"size"`
	Shape     string       `json:"shape"`
	Tiles     []DoodadTile `json:"tiles"`
}

func itemFromSpec(spec materials.ItemSpec) otmap.Item {
	item := otmap.Item{ID: spec.ID, Subtype: spec.Subtype}
	if spec.ActionID != 0 || spec.UniqueID != 0 || spec.Text != "" {
		item.Ext = &otmap.ItemExt{ActionID: spec.ActionID, UniqueID: spec.UniqueID, Text: spec.Text}
	}
	return item
}

func (a *doodadAlt) pickSingle(e *Editor) (otmap.Item, bool) {
	if len(a.singles) == 0 {
		return otmap.Item{}, false
	}
	if a.singleTotal <= 0 {
		return itemFromSpec(a.singles[0].item), true
	}
	roll := e.rng.IntN(a.singleTotal) + 1
	for _, single := range a.singles {
		if roll <= single.chance {
			return itemFromSpec(single.item), true
		}
		roll -= single.chance
	}
	return itemFromSpec(a.singles[0].item), true
}

func (a *doodadAlt) pickComposite(e *Editor) []materials.CompositeTile {
	if len(a.composites) == 0 {
		return nil
	}
	if a.compositeTotal <= 0 {
		return a.composites[0].tiles
	}
	roll := e.rng.IntN(a.compositeTotal) + 1
	for _, composite := range a.composites {
		if roll <= composite.chance {
			return composite.tiles
		}
	}
	return a.composites[0].tiles
}

// useComposite decides between a composite and a single object.
func (a *doodadAlt) useComposite(e *Editor) bool {
	if !a.hasComposite {
		return false
	}
	if !a.hasSingleObject {
		return true
	}
	total := a.compositeTotal + a.singleTotal
	if total <= 0 {
		return true
	}
	return e.rng.IntN(total) < a.compositeTotal
}

// doodadPlacement collects the tiles of a placement being generated, keyed
// by their offset from the cursor.
type doodadPlacement struct {
	editor *Editor
	alt    *doodadAlt
	tiles  map[[3]int]*DoodadTile
}

// put adds an item to the tile at an offset, creating the tile.
func (p *doodadPlacement) put(x, y, z int, item otmap.Item) {
	key := [3]int{x, y, z}
	tile := p.tiles[key]
	if tile == nil {
		tile = &DoodadTile{X: x, Y: y, Z: z}
		p.tiles[key] = tile
	}
	tile.Items = append(tile.Items, item)
}

// placeAt places one object (a composite or a single item) at an offset. It
// reports false when the spot is taken or nothing was placed.
func (p *doodadPlacement) placeAt(x, y int) bool {
	editor := p.editor
	if p.alt.useComposite(editor) {
		composite := p.alt.pickComposite(editor)
		for _, compositeTile := range composite {
			if _, taken := p.tiles[[3]int{x + compositeTile.X, y + compositeTile.Y, compositeTile.Z}]; taken {
				return false
			}
		}
		for _, compositeTile := range composite {
			for _, spec := range compositeTile.Items {
				p.put(x+compositeTile.X, y+compositeTile.Y, compositeTile.Z, itemFromSpec(spec))
				if editor.isGround(spec.ID) {
					p.tiles[[3]int{x + compositeTile.X, y + compositeTile.Y, compositeTile.Z}].Replace = true
				}
			}
		}
		return len(composite) > 0
	}
	if _, taken := p.tiles[[3]int{x, y, 0}]; taken {
		return false
	}
	item, ok := p.alt.pickSingle(editor)
	if ok {
		p.put(x, y, 0, item)
	}
	return ok
}

// scatter places count objects at random spots of the brush area.
func (p *doodadPlacement) scatter(size int, shape string, count int) {
	rng := p.editor.rng
	for object := 0; object < count; object++ {
		for retry := 0; retry < doodadPlaceRetries; retry++ {
			x := rng.IntN(2*size+1) - size
			y := rng.IntN(2*size+1) - size
			if shape == shapeCircle && math.Sqrt(float64(x*x+y*y)) >= float64(size)+circleEdgeSlack {
				continue
			}
			if p.placeAt(x, y) {
				break
			}
		}
	}
}

// sortedTiles returns the tiles ordered by floor, row and column.
func (p *doodadPlacement) sortedTiles() []DoodadTile {
	var tiles []DoodadTile
	for _, tile := range p.tiles {
		tiles = append(tiles, *tile)
	}
	// Map order is random: placing in a fixed order keeps undo steps and
	// border updates the same for the same placement.
	slices.SortFunc(tiles, func(first, second DoodadTile) int {
		return cmp.Or(cmp.Compare(first.Z, second.Z), cmp.Compare(first.Y, second.Y), cmp.Compare(first.X, second.X))
	})
	return tiles
}

// doodadArea is the number of tiles a brush of this size and shape covers.
func doodadArea(size int, shape string) int {
	if shape != shapeCircle {
		return (2*size+1)*(2*size+1) + 1
	}
	if size == 1 {
		return doodadSmallCircleArea
	}
	return int(0.5 + float64(size*size)*math.Pi)
}

// doodadScatterCount picks how many objects a brush scatters. Thickness
// overrides the brush's own thickness when above 0.
func (e *Editor) doodadScatterCount(brush *DoodadBrush, size int, shape string, thickness float64) int {
	area := doodadArea(size, shape)
	var objects int
	if thickness > 0 {
		objects = int(float64(area) * thickness)
	} else {
		objects = brush.opts.Thickness * area / max(brush.opts.ThicknessCeiling, 1)
	}
	return max(1, objects+e.rng.IntN(max(objects, 0)+1))
}

// GenerateDoodad builds a new placement for a doodad brush. Thickness sets
// how many objects a bigger brush scatters.
func (e *Editor) GenerateDoodad(name string, variation, size int, shape string, thickness float64) *DoodadBuffer {
	brush := e.Brushes.Doodad(name)
	buffer := &DoodadBuffer{Brush: name, Variation: variation, Size: size, Shape: shape}
	var alt *doodadAlt
	if brush != nil {
		alt = brush.alt(variation)
	}
	if alt == nil {
		e.doodad = buffer
		return buffer
	}
	placement := &doodadPlacement{editor: e, alt: alt, tiles: map[[3]int]*DoodadTile{}}
	if size <= 0 || brush.opts.OneSize {
		placement.placeAt(0, 0)
	} else {
		placement.scatter(size, shape, e.doodadScatterCount(brush, size, shape, thickness))
	}
	buffer.Tiles = placement.sortedTiles()
	e.doodad = buffer
	return buffer
}

// doodadFits reports whether a doodad tile may be placed on the current
// tile. alt places on blocking tiles and duplicates.
func (e *Editor) doodadFits(brush *DoodadBrush, current *otmap.Tile, replace, alt bool) bool {
	if !replace && !(brush.opts.OnBlocking || alt) && (current == nil || e.blocking(current)) {
		return false
	}
	if current != nil && !replace && !brush.opts.OnDuplicate && !alt {
		duplicate := slices.ContainsFunc(current.Items, func(item otmap.Item) bool { return brush.owns(item.ID) })
		if duplicate {
			return false
		}
	}
	return true
}

// copyStampTile copies a stamp tile as it was: its ground and items in their
// order. Placing item by item would let ground equivalents of borders and
// stack rules change it.
func (e *Editor) copyStampTile(tile *otmap.Tile, items []otmap.Item) {
	tile.Ground = otmap.Item{}
	tile.Items = nil
	for i, item := range items {
		if i == 0 && e.isGround(item.ID) {
			tile.Ground = item.Clone()
			continue
		}
		tile.Items = append(tile.Items, item.Clone())
	}
}

// reborderAround updates borders and walls of the positions and their
// neighbors, each tile once.
func (e *Editor) reborderAround(tx *Tx, positions []otmap.Position) {
	seen := map[otmap.Position]bool{}
	for _, pos := range positions {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if neighbor, ok := pos.Add(dx, dy, 0); ok && !seen[neighbor] {
					seen[neighbor] = true
					e.borderize(tx, neighbor)
					e.wallize(tx, neighbor)
				}
			}
		}
	}
}

// placeDoodad puts the current buffer at the cursor and generates the next
// one. alt places on blocking tiles and duplicates.
func (e *Editor) placeDoodad(tx *Tx, center otmap.Position, alt bool, thickness float64) {
	buffer := e.doodad
	if buffer == nil {
		return
	}
	brush := e.Brushes.Doodad(buffer.Brush)
	if brush == nil {
		return
	}
	var reborder []otmap.Position
	for _, doodadTile := range buffer.Tiles {
		pos, ok := center.Add(doodadTile.X, doodadTile.Y, doodadTile.Z)
		if !ok {
			continue
		}
		if !e.doodadFits(brush, tx.Get(pos), doodadTile.Replace, alt) {
			continue
		}
		tile := tx.Edit(pos)
		if doodadTile.Replace {
			e.copyStampTile(tile, doodadTile.Items)
		} else {
			for _, item := range doodadTile.Items {
				// Walls of the doodad replace walls already there.
				if wall, _ := e.Brushes.wallOf(item.ID); wall != nil {
					e.cleanWalls(tile, nil)
				}
				if brush.opts.RedoBorders && e.isGround(item.ID) {
					reborder = append(reborder, pos)
				}
				e.addItem(tile, item.Clone())
			}
		}
		if brush.opts.RemoveOptionalBorder {
			tile.State &^= otmap.StateOptionalBorder
		}
	}
	if len(reborder) > 0 && e.Settings.Automagic {
		e.reborderAround(tx, reborder)
	}
	e.GenerateDoodad(buffer.Brush, buffer.Variation, buffer.Size, buffer.Shape, thickness)
}

// eraseDoodad removes doodad items from a tile: items of the brush only
// (DoodadEraseLike) or of any doodad brush.
func (e *Editor) eraseDoodad(tile *otmap.Tile, brush *DoodadBrush) {
	keep := tile.Items[:0]
	for _, item := range tile.Items {
		info := e.Brushes.info[item.ID]
		owned := info != nil && info.doodad != nil
		if owned && item.Complex() && e.Settings.EraserLeaveUnique {
			owned = false
		}
		if owned && e.Settings.DoodadEraseLike {
			owned = brush.owns(item.ID)
		}
		if !owned {
			keep = append(keep, item)
		}
	}
	tile.Items = keep
	if info := e.Brushes.info[tile.Ground.ID]; tile.Ground.ID != 0 && info != nil && info.doodad != nil {
		if !e.Settings.DoodadEraseLike || brush.owns(tile.Ground.ID) {
			tile.Ground = otmap.Item{}
		}
	}
}

// Doodad returns the current doodad preview.
func (e *Editor) Doodad() *DoodadBuffer { return e.doodad }
