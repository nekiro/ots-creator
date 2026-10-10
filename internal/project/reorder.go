package project

import (
	"fmt"
	"slices"

	"github.com/nekiro/ots-creator/internal/thing"
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

// MoveThings moves things of category c to the ids from at on, in id
// order, as one undoable edit. No thing is lost: by default the things at
// the targets take the freed ids (in order); with Insert the moved things
// go before at and the things between shift to close the gap. Targets past
// the end of the category extend it. It returns the new ids of ids.
func (p *Project) MoveThings(c thing.Category, ids []uint32, at uint32, o MoveOptions) ([]uint32, error) {
	if !c.Valid() {
		return nil, fmt.Errorf("invalid category %d", c)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("nothing to move")
	}
	if at < c.MinID() {
		return nil, fmt.Errorf("%s id %d is below %d", c, at, c.MinID())
	}
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	if len(slices.Compact(slices.Clone(sorted))) != len(sorted) {
		return nil, fmt.Errorf("an id is moved twice")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	last := p.things.MaxID(c)
	for _, id := range sorted {
		if id < c.MinID() || id > last {
			return nil, fmt.Errorf("%s %d does not exist", c, id)
		}
	}
	var moves map[uint32]uint32
	if o.Insert {
		moves = insertMoves(sorted, min(at, last+1))
	} else {
		moves = swapMoves(sorted, at)
	}
	for from, to := range moves {
		if from == to {
			delete(moves, from)
		}
	}
	out := make([]uint32, len(ids))
	for i, id := range ids {
		out[i] = id
		if to, ok := moves[id]; ok {
			out[i] = to
		}
	}
	if len(moves) == 0 {
		return out, nil
	}

	// New things by target id; ids past the end are empty things.
	get := func(id uint32) *thing.Thing {
		if t := p.things.Get(c, id); t != nil {
			return t
		}
		return thing.New(id, c)
	}
	next := map[uint32]*thing.Thing{}
	for from, to := range moves {
		t := get(from).Clone()
		t.ID = to
		next[to] = t
	}
	if o.UpdateRefs && c == thing.CategoryItem {
		for id := c.MinID(); id <= p.things.MaxID(c); id++ {
			if _, ok := next[id]; ok {
				continue
			}
			if t := p.things.Get(c, id); t != nil && hasRefs(t, moves) {
				next[id] = t.Clone()
			}
		}
		for _, t := range next {
			remapRefs(t, moves)
		}
	}

	r := p.record(fmt.Sprintf("Move %d %s(s)", len(ids), c))
	targets := make([]uint32, 0, len(next))
	for id := range next {
		targets = append(targets, id)
	}
	slices.Sort(targets)
	// Empty things fill any gap before targets past the end.
	for id := p.things.MaxID(c) + 1; id < targets[len(targets)-1]; id++ {
		if _, ok := next[id]; !ok {
			r.setThing(c, id, thing.New(id, c))
		}
	}
	for _, id := range targets {
		r.setThing(c, id, next[id])
	}
	r.commit()
	return out, nil
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
		if t := at + uint32(i); !moving[t] {
			displaced = append(displaced, t)
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
	lo := min(sorted[0], at)
	hi := max(sorted[len(sorted)-1], at-1)
	moving := map[uint32]bool{}
	for _, id := range sorted {
		moving[id] = true
	}
	var before, after []uint32
	for id := lo; id <= hi; id++ {
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
		moves[id] = lo + uint32(i)
	}
	return moves
}

// refs returns pointers to the item ids stored in an item.
func refs(t *thing.Thing) (small []*uint16, big []*uint32) {
	small = []*uint16{&t.Props.Market.TradeAs, &t.Props.Market.ShowAs, &t.Props.FormerObjectID}
	for i := range t.NpcSales {
		big = append(big, &t.NpcSales[i].CurrencyObjectID)
	}
	return small, big
}

func hasRefs(t *thing.Thing, moves map[uint32]uint32) bool {
	small, big := refs(t)
	for _, v := range small {
		if _, ok := moves[uint32(*v)]; ok && *v != 0 {
			return true
		}
	}
	for _, v := range big {
		if _, ok := moves[*v]; ok && *v != 0 {
			return true
		}
	}
	return false
}

func remapRefs(t *thing.Thing, moves map[uint32]uint32) {
	small, big := refs(t)
	for _, v := range small {
		if to, ok := moves[uint32(*v)]; ok && *v != 0 && to <= 0xFFFF {
			*v = uint16(to)
		}
	}
	for _, v := range big {
		if to, ok := moves[*v]; ok && *v != 0 {
			*v = to
		}
	}
}
