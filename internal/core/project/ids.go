package project

import (
	"fmt"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

// FreeIDs returns count ids of category from id from on that are free: past
// the end of the category, or blank things (no name, default properties
// and no sprite with pixels).
func (p *Project) FreeIDs(category thing.Category, from uint32, count int) ([]uint32, error) {
	if !category.Valid() {
		return nil, fmt.Errorf("invalid category %d", category)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]uint32, 0, count)
	for id := max(from, category.MinID()); len(ids) < count; id++ {
		if object := p.things.Get(category, id); object == nil || p.isBlank(object) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// TakenIDs returns the ids of ids that hold a thing in category that is not
// blank.
func (p *Project) TakenIDs(category thing.Category, ids []uint32) []uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := []uint32{}
	for _, id := range ids {
		if object := p.things.Get(category, id); object != nil && !p.isBlank(object) {
			out = append(out, id)
		}
	}
	return out
}

func (p *Project) isBlank(object *thing.Thing) bool {
	return object.Name == "" && object.Description == "" && object.Props == (thing.Properties{}) && len(object.NpcSales) == 0 && !p.hasPixels(object)
}
