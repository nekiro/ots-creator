package mapedit

import (
	"errors"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
)

// DoodadBrush returns a doodad brush that places the clip as one composite,
// relative to its top-left tile. Its look is the top item of the first tile.
func (c *Clip) DoodadBrush(name string) (materials.Brush, error) {
	composite := materials.Composite{Chance: 1}
	for _, tile := range c.Tiles {
		compositeTile := materials.CompositeTile{X: tile.DX, Y: tile.DY, Z: tile.DZ}
		if tile.Ground != nil {
			compositeTile.Items = append(compositeTile.Items, materials.ItemSpec{ID: tile.Ground.ID, Subtype: tile.Ground.Subtype})
		}
		for _, item := range tile.Items {
			compositeTile.Items = append(compositeTile.Items, materials.ItemSpec{
				ID: item.ID, Subtype: item.Subtype, ActionID: item.ActionID(), UniqueID: item.UniqueID(),
			})
		}
		if len(compositeTile.Items) > 0 {
			composite.Tiles = append(composite.Tiles, compositeTile)
		}
	}
	if len(composite.Tiles) == 0 {
		return materials.Brush{}, errors.New("the selection holds no items")
	}
	firstItems := composite.Tiles[0].Items
	return materials.Brush{
		Name:         name,
		Type:         materials.TypeDoodad,
		Look:         firstItems[len(firstItems)-1].ID,
		Doodad:       &materials.DoodadOptions{OnBlocking: true, OneSize: true},
		Alternatives: []materials.Alternative{{Composites: []materials.Composite{composite}}},
	}, nil
}
