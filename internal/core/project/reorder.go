package project

import (
	"fmt"
	"math"
	"slices"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

// MoveOptions controls MoveThings.
type MoveOptions struct {
	// Insert puts the things before id at and shifts the things between;
	// otherwise the things at the targets swap places with them.
	Insert bool `json:"insert"`
	// UpdateRefs changes item ids stored in items (market trade as and show
	// as, former object, NPC currency) to the new ids.
	UpdateRefs bool `json:"updateRefs"`
}

// MoveThings moves things of category category to the ids from at on, in
// id order, as one undoable edit. No thing is lost: by default the things at
// the targets take the freed ids (in order); with Insert the moved things
// go before at and the things between shift to close the gap. Targets past
// the end of the category extend it. It returns the new ids of ids.
func (p *Project) MoveThings(category thing.Category, ids []uint32, at uint32, options MoveOptions) ([]uint32, error) {
	sorted, err := sortedMoveIDs(category, ids, at)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	moves, err := p.planMoves(category, sorted, at, options.Insert)
	if err != nil {
		return nil, err
	}
	newIDs := make([]uint32, len(ids))
	for i, id := range ids {
		newIDs[i] = id
		if to, ok := moves[id]; ok {
			newIDs[i] = to
		}
	}
	if len(moves) == 0 {
		return newIDs, nil
	}
	next := p.movedThings(category, moves, options.UpdateRefs)
	p.recordMoves(fmt.Sprintf("Move %d %s(s)", len(ids), category), category, moves, next)
	return newIDs, nil
}

// sortedMoveIDs checks a move request and returns ids sorted.
func sortedMoveIDs(category thing.Category, ids []uint32, at uint32) ([]uint32, error) {
	if !category.Valid() {
		return nil, fmt.Errorf("invalid category %d", category)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("nothing to move")
	}
	if at < category.MinID() {
		return nil, fmt.Errorf("%s id %d is below %d", category, at, category.MinID())
	}
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	if len(slices.Compact(slices.Clone(sorted))) != len(sorted) {
		return nil, fmt.Errorf("an id is moved twice")
	}
	return sorted, nil
}

// planMoves maps the ids that change to their new ids. The caller holds
// the lock.
func (p *Project) planMoves(category thing.Category, sorted []uint32, at uint32, insert bool) (map[uint32]uint32, error) {
	last := p.things.MaxID(category)
	for _, id := range sorted {
		if id < category.MinID() || id > last {
			return nil, fmt.Errorf("%s %d does not exist", category, id)
		}
	}
	var moves map[uint32]uint32
	if insert {
		moves = insertMoves(sorted, min(at, last+1))
	} else {
		moves = swapMoves(sorted, at)
	}
	for from, to := range moves {
		if from == to {
			delete(moves, from)
		}
	}
	return moves, nil
}

// movedThings returns the new things by target id; ids past the end are
// empty things. With updateRefs, items that reference a moved item are
// included with their references changed.
func (p *Project) movedThings(category thing.Category, moves map[uint32]uint32, updateRefs bool) map[uint32]*thing.Thing {
	thingOrEmpty := func(id uint32) *thing.Thing {
		if object := p.things.Get(category, id); object != nil {
			return object
		}
		return thing.New(id, category)
	}
	next := map[uint32]*thing.Thing{}
	for from, to := range moves {
		object := thingOrEmpty(from).Clone()
		object.ID = to
		next[to] = object
	}
	if updateRefs && category == thing.CategoryItem {
		for id := category.MinID(); id <= p.things.MaxID(category); id++ {
			if _, ok := next[id]; ok {
				continue
			}
			if object := p.things.Get(category, id); object != nil && hasRefs(object, moves) {
				next[id] = object.Clone()
			}
		}
		for _, object := range next {
			remapRefs(object, moves)
		}
	}
	return next
}

// recordMoves stores the moved things as one undoable edit.
func (p *Project) recordMoves(label string, category thing.Category, moves map[uint32]uint32, next map[uint32]*thing.Thing) {
	edit := p.record(label)
	if category == thing.CategoryItem {
		edit.entry.itemMoves = moves
	}
	targets := make([]uint32, 0, len(next))
	for id := range next {
		targets = append(targets, id)
	}
	slices.Sort(targets)
	// Empty things fill any gap before targets past the end.
	for id := p.things.MaxID(category) + 1; id < targets[len(targets)-1]; id++ {
		if _, ok := next[id]; !ok {
			edit.setThing(category, id, thing.New(id, category))
		}
	}
	for _, id := range targets {
		edit.setThing(category, id, next[id])
	}
	edit.commit()
}

// swapMoves moves sorted to at, at+1, …; the things at those ids that do
// not move themselves take the freed ids, both in id order.
func swapMoves(sorted []uint32, at uint32) map[uint32]uint32 {
	moves := map[uint32]uint32{}
	moving := map[uint32]bool{}
	targets := map[uint32]bool{}
	for i, id := range sorted {
		moving[id] = true
		targets[at+uint32(i)] = true
		moves[id] = at + uint32(i)
	}
	var displaced, freed []uint32
	for i := range sorted {
		if target := at + uint32(i); !moving[target] {
			displaced = append(displaced, target)
		}
	}
	for _, id := range sorted {
		if !targets[id] {
			freed = append(freed, id)
		}
	}
	for i, id := range displaced {
		moves[id] = freed[i]
	}
	return moves
}

// insertMoves puts sorted before id at (at most one past the last id) and
// shifts the things between: the ids from the lowest of them and at to the
// highest are filled in their new order.
func insertMoves(sorted []uint32, at uint32) map[uint32]uint32 {
	low := min(sorted[0], at)
	high := max(sorted[len(sorted)-1], at-1)
	moving := map[uint32]bool{}
	for _, id := range sorted {
		moving[id] = true
	}
	var before, after []uint32
	for id := low; id <= high; id++ {
		switch {
		case moving[id]:
		case id < at:
			before = append(before, id)
		default:
			after = append(after, id)
		}
	}
	order := slices.Concat(before, sorted, after)
	moves := map[uint32]uint32{}
	for i, id := range order {
		moves[id] = low + uint32(i)
	}
	return moves
}

// refs returns pointers to the item ids stored in an item.
func refs(object *thing.Thing) (small []*uint16, big []*uint32) {
	small = []*uint16{&object.Props.Market.TradeAs, &object.Props.Market.ShowAs, &object.Props.FormerObjectID}
	for i := range object.NpcSales {
		big = append(big, &object.NpcSales[i].CurrencyObjectID)
	}
	return small, big
}

func hasRefs(object *thing.Thing, moves map[uint32]uint32) bool {
	small, big := refs(object)
	for _, ref := range small {
		if _, ok := moves[uint32(*ref)]; ok && *ref != 0 {
			return true
		}
	}
	for _, ref := range big {
		if _, ok := moves[*ref]; ok && *ref != 0 {
			return true
		}
	}
	return false
}

func remapRefs(object *thing.Thing, moves map[uint32]uint32) {
	small, big := refs(object)
	for _, ref := range small {
		if to, ok := moves[uint32(*ref)]; ok && *ref != 0 && to <= math.MaxUint16 {
			*ref = uint16(to)
		}
	}
	for _, ref := range big {
		if to, ok := moves[*ref]; ok && *ref != 0 {
			*ref = to
		}
	}
}
