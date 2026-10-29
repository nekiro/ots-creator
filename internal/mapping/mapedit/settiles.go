package mapedit

import "github.com/nekiro/ots-creator/internal/mapping/otmap"

// TileSpec is what a tile shows: its ground (0: none) and its items,
// bottom to top.
type TileSpec struct {
	Pos    otmap.Position `json:"pos"`
	Ground uint16         `json:"ground"`
	Items  []uint16       `json:"items"`
}

// SetTiles replaces the ground and items of tiles, keeping the items in the
// given order; creatures, spawns, flags and houses stay. Requests with the
// same non-zero stroke merge into one undo step. It returns the number of
// changed tiles.
func (e *Editor) SetTiles(specs []TileSpec, stroke int64) int {
	tx := e.Begin("Paint terrain", stroke)
	for _, spec := range specs {
		tile := tx.Edit(spec.Pos)
		tile.Ground = otmap.Item{}
		if spec.Ground != 0 {
			tile.Ground = e.newItem(spec.Ground)
		}
		tile.Items = tile.Items[:0]
		for _, id := range spec.Items {
			tile.Items = append(tile.Items, e.newItem(id))
		}
	}
	return tx.Commit()
}
