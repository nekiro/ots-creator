package mapedit

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// ConvertToClientIDs converts the current working map as one undo step.
// Validate every item before touching any tile, including container contents.
func (e *Editor) ConvertToClientIDs(items *otitems.DB, brushes *Brushes) (int, error) {
	if e.Items.Mode != otitems.ModeOTB {
		return 0, fmt.Errorf("this map already uses client IDs")
	}
	if items == nil || items.Mode != otitems.ModeClient {
		return 0, fmt.Errorf("client item data is required")
	}
	pairs := map[uint16]uint16{}
	count := 0
	var validate func(otmap.Item) error
	validate = func(item otmap.Item) error {
		if item.ID != 0 {
			typeInfo := e.Items.Get(item.ID)
			if typeInfo == nil || typeInfo.ClientID == 0 {
				return fmt.Errorf("server item %d has no client ID in the loaded items.otb", item.ID)
			}
			if items.Get(typeInfo.ClientID) == nil {
				return fmt.Errorf("client item %d (server item %d) is missing from the open client", typeInfo.ClientID, item.ID)
			}
			pairs[item.ID] = typeInfo.ClientID
			count++
		}
		if item.Ext != nil {
			for _, child := range item.Ext.Items {
				if err := validate(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var invalid error
	e.Map.Each(func(_ otmap.Position, tile *otmap.Tile) {
		if invalid != nil {
			return
		}
		invalid = validate(tile.Ground)
		for _, item := range tile.Items {
			if invalid != nil {
				break
			}
			invalid = validate(item)
		}
	})
	if invalid != nil {
		return 0, invalid
	}
	beforeItems, beforeBrushes := e.Items, e.Brushes
	tx := e.Begin("Convert to client IDs", 0)
	tx.Meta() // An empty map still changes its ID mode.
	var rewrite func(*otmap.Item)
	rewrite = func(item *otmap.Item) {
		if item.ID != 0 {
			item.ID = pairs[item.ID]
		}
		if item.Ext != nil {
			for i := range item.Ext.Items {
				rewrite(&item.Ext.Items[i])
			}
		}
	}
	e.Map.Each(func(pos otmap.Position, _ *otmap.Tile) {
		tile := tx.Edit(pos)
		rewrite(&tile.Ground)
		for i := range tile.Items {
			rewrite(&tile.Items[i])
		}
	})
	tx.Commit()
	action := e.top()
	action.itemsBefore, action.brushesBefore = beforeItems, beforeBrushes
	action.itemsAfter, action.brushesAfter = items, brushes
	e.Items = items
	e.SetBrushes(brushes)
	return count, nil
}
