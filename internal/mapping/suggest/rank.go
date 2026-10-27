package suggest

import (
	"math"
	"slices"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	// minPairCount drops pairs seen fewer times: single sightings are noise.
	minPairCount = 2
	// A part completes an object when it lies in the same direction of the
	// object at least minCompletionCount times and in minCompletionShare of
	// the places, both ways.
	minCompletionCount = 3
	minCompletionShare = 0.6
	// maxCompanionShare tells companions from substitutes: an item that lies
	// on the same tile as the other one in more of its places goes with it.
	maxCompanionShare = 0.1
	// maxSubstituteCandidates is the number of items with the most alike
	// grounds whose neighbors are compared.
	maxSubstituteCandidates = 64
)

// Tuning weighs the parts of the "here" ranking; eval_test.go measures them
// on real maps.
type Tuning struct {
	// Ground, Tile and Near weigh what lies on the ground of the place,
	// with the items of its tile and next to the items around it.
	Ground, Tile, Near float64
	// LocalWeight weighs the counts of the area within LocalRadius blocks
	// of the place (0: none): the style of the town, cave or swamp being
	// mapped, not of the whole map.
	LocalWeight float64
	LocalRadius int
	// Directional takes the side a neighbor lies on into account.
	Directional bool
	// Lift raises what comes with a context more often than by chance;
	// without it the most frequent items win.
	Lift bool
}

// DefaultTuning is the best of eval_test.go on a large real map: the
// ground weighs as little as the rest, the directions of the neighbors and
// the area around matter most.
var DefaultTuning = Tuning{Ground: 1, Tile: 2, Near: 2, LocalWeight: 3, LocalRadius: 1, Directional: true, Lift: true}

// Cell is the ground and items of a tile.
type Cell struct {
	Ground uint16
	Items  []uint16
}

// Context is a place on the map: its tile and the tiles around it, by
// direction (see Offsets).
type Context struct {
	Pos       otmap.Position
	Ground    uint16
	Items     []uint16
	Neighbors [8]Cell
}

// Ranked is a suggested id with its score and the context id that added the
// most to it; Local tells that the area around the place added the most.
type Ranked struct {
	ID     uint16
	Score  float64
	Via    uint16
	Reason Reason
	Local  bool
}

type Reason uint8

const (
	ReasonGround Reason = iota + 1
	ReasonTile
	ReasonNear
	ReasonWith
	ReasonAlike
)

// view is the counts of the whole map or of the area around a place, with
// its weight.
type view struct {
	sums   *counts
	weight float64
	local  bool
}

func (index *Index) views(pos otmap.Position) []view {
	views := []view{{index.total, 1, false}}
	if index.tuning.LocalWeight > 0 {
		views = append(views, view{index.around(BlockOf(pos), index.tuning.LocalRadius), index.tuning.LocalWeight, true})
	}
	return views
}

// ranking sums the score parts of the candidates.
type ranking struct {
	candidates map[uint16]*Ranked
	best       map[uint16]float64
	lift       bool
}

func newRanking() *ranking {
	return &ranking{candidates: map[uint16]*Ranked{}, best: map[uint16]float64{}, lift: true}
}

// add scores the ids seen with a context id: weight times how often they
// come with it, raised by how much more often than by chance (lift).
// denominator is the number of chances to see them with it; chance gives
// the share of an id anywhere.
func (rank *ranking) add(seen map[uint16]int32, denominator, weight float64, via uint16, reason Reason, chance func(id uint16) float64, local bool) {
	if denominator <= 0 || weight <= 0 {
		return
	}
	for id, count := range seen {
		if count < minPairCount {
			continue
		}
		conditional := float64(count) / denominator
		anywhere := chance(id)
		if anywhere <= 0 {
			continue
		}
		part := weight * conditional
		if rank.lift {
			part *= math.Log2(1 + conditional/anywhere)
		}
		rank.score(id, part, via, reason, local)
	}
}

func (rank *ranking) score(id uint16, part float64, via uint16, reason Reason, local bool) {
	candidate := rank.candidates[id]
	if candidate == nil {
		candidate = &Ranked{ID: id}
		rank.candidates[id] = candidate
	}
	candidate.Score += part
	if part > rank.best[id] {
		rank.best[id] = part
		candidate.Via, candidate.Reason, candidate.Local = via, reason, local
	}
}

func (rank *ranking) top(limit int, skip func(id uint16) bool) []Ranked {
	list := make([]Ranked, 0, len(rank.candidates))
	for _, candidate := range rank.candidates {
		if skip == nil || !skip(candidate.ID) {
			list = append(list, *candidate)
		}
	}
	slices.SortFunc(list, func(left, right Ranked) int {
		if left.Score != right.Score {
			if left.Score > right.Score {
				return -1
			}
			return 1
		}
		return int(left.ID) - int(right.ID)
	})
	return list[:min(limit, len(list))]
}

// Here ranks the items that fit a place, leaving out the items already on
// its tile.
func (index *Index) Here(place Context, limit int) []Ranked {
	index.mu.Lock()
	defer index.mu.Unlock()
	rank := newRanking()
	rank.lift = index.tuning.Lift
	tuning := index.tuning
	tileItems := unique(place.Items)
	var neighborItems [8][]uint16
	neighborCount := 0
	for direction, cell := range place.Neighbors {
		neighborItems[direction] = unique(cell.Items)
		neighborCount += len(neighborItems[direction])
	}
	for _, view := range index.views(place.Pos) {
		sums := view.sums
		if place.Ground != 0 {
			rank.add(sums.onGround[place.Ground], float64(sums.grounds[place.Ground]), tuning.Ground*view.weight, place.Ground, ReasonGround, sums.itemChance, view.local)
		}
		for _, item := range tileItems {
			rank.add(sums.stacked[item], float64(sums.items[item]), tuning.Tile*view.weight/float64(len(tileItems)), item, ReasonTile, sums.itemChance, view.local)
		}
		for direction, items := range neighborItems {
			for _, item := range items {
				// The place lies in the opposite direction from its neighbor.
				seen, chances := sums.near.toward(item, opposite(direction)), float64(sums.items[item])
				if !tuning.Directional {
					seen, chances = sums.near.around(item), chances*8
				}
				rank.add(seen, chances, tuning.Near*view.weight/float64(neighborCount), item, ReasonNear, sums.itemChance, view.local)
			}
		}
	}
	return rank.top(limit, func(id uint16) bool { return slices.Contains(tileItems, id) })
}

// With ranks the items that lie with the given items, on their tile or next
// to it, around a position; the given items are left out.
func (index *Index) With(ids []uint16, pos otmap.Position, limit int) []Ranked {
	index.mu.Lock()
	defer index.mu.Unlock()
	rank := newRanking()
	ids = unique(ids)
	for _, view := range index.views(pos) {
		sums := view.sums
		for _, item := range ids {
			weight := view.weight / float64(len(ids))
			rank.add(sums.stacked[item], float64(sums.items[item]), 2*weight, item, ReasonWith, sums.itemChance, view.local)
			rank.add(sums.near.around(item), float64(sums.items[item])*8, weight, item, ReasonWith, sums.itemChance, view.local)
		}
	}
	return rank.top(limit, func(id uint16) bool { return slices.Contains(ids, id) })
}

// Grounds ranks the grounds that border the grounds of a place, leaving the
// ground of its tile out.
func (index *Index) Grounds(place Context, limit int) []Ranked {
	index.mu.Lock()
	defer index.mu.Unlock()
	rank := newRanking()
	context := []uint16{place.Ground}
	for _, cell := range place.Neighbors {
		context = append(context, cell.Ground)
	}
	context = unique(context)
	for _, view := range index.views(place.Pos) {
		sums := view.sums
		for _, ground := range context {
			rank.add(sums.groundNear[ground], float64(sums.grounds[ground])*8, view.weight/float64(len(context)), ground, ReasonNear, sums.groundChance, view.local)
		}
	}
	return rank.top(limit, func(id uint16) bool { return id == place.Ground })
}

// Substitutes ranks the items that take the same places as an item: they
// lie on the same grounds and next to the same items, but seldom with it.
func (index *Index) Substitutes(id uint16, limit int) []Ranked {
	index.mu.Lock()
	defer index.mu.Unlock()
	sums := index.total
	if sums.items[id] < minPairCount {
		return nil
	}
	// The item next to itself (a row of bushes) matches the candidate next
	// to itself (a row of ferns): both count as "self".
	nearOf := func(item uint16) map[uint16]int32 {
		around := sums.near.around(item)
		if count, ok := around[item]; ok {
			delete(around, item)
			around[0] = count
		}
		return around
	}
	// The candidates lie on a ground of the item; one pass over the grounds
	// gives the grounds of all of them.
	targetGrounds := map[uint16]int32{}
	for ground, seen := range sums.onGround {
		if count := seen[id]; count > 0 {
			targetGrounds[ground] = count
		}
	}
	groundsOf := map[uint16]map[uint16]int32{}
	for ground := range targetGrounds {
		for item := range sums.onGround[ground] {
			if item != id && sums.items[item] >= minPairCount {
				groundsOf[item] = map[uint16]int32{}
			}
		}
	}
	for ground, seen := range sums.onGround {
		for item, count := range seen {
			if vector := groundsOf[item]; vector != nil {
				vector[ground] = count
			}
		}
	}
	// The grounds pick the best candidates, the neighbors decide among them.
	byGround := newRanking()
	for candidate, vector := range groundsOf {
		if score := cosine(targetGrounds, vector); score > 0 && !index.companion(id, candidate) {
			byGround.score(candidate, score, id, ReasonAlike, false)
		}
	}
	targetNear := nearOf(id)
	rank := newRanking()
	for _, candidate := range byGround.top(maxSubstituteCandidates, nil) {
		rank.score(candidate.ID, (candidate.Score+cosine(targetNear, nearOf(candidate.ID)))/2, id, ReasonAlike, false)
	}
	return rank.top(limit, nil)
}

// companion reports whether other goes with id instead of taking its place:
// it lies on the same tile in more than maxCompanionShare of the places of
// id, or next to it in one direction in most of them.
func (index *Index) companion(id, other uint16) bool {
	sums := index.total
	places := float64(sums.items[id])
	if float64(sums.stacked[id][other]) > maxCompanionShare*places {
		return true
	}
	for direction := range Offsets {
		if float64(sums.near[id][uint32(direction)<<16|uint32(other)]) > minCompletionShare*places {
			return true
		}
	}
	return false
}

// Completion is a part that completes an object: ID lies in Direction (see
// Offsets) of the object item in Share of its places.
type Completion struct {
	Direction int
	ID        uint16
	Share     float64
}

// Completions returns the parts that lie next to an item almost wherever it
// lies, while the item lies next to them almost wherever they lie: the
// other parts of objects made of several items.
func (index *Index) Completions(id uint16) []Completion {
	index.mu.Lock()
	defer index.mu.Unlock()
	sums := index.total
	total := float64(sums.items[id])
	if total < minCompletionCount {
		return nil
	}
	var out []Completion
	for direction := range Offsets {
		var best uint16
		var bestCount int32
		for second, count := range sums.near.toward(id, direction) {
			if second != id && (count > bestCount || count == bestCount && second < best) {
				best, bestCount = second, count
			}
		}
		if bestCount < minCompletionCount {
			continue
		}
		share := float64(bestCount) / total
		back := float64(sums.near.toward(best, opposite(direction))[id]) / float64(sums.items[best])
		if share >= minCompletionShare && back >= minCompletionShare {
			out = append(out, Completion{Direction: direction, ID: best, Share: share})
		}
	}
	return out
}

func cosine(left, right map[uint16]int32) float64 {
	var dot, leftNorm, rightNorm float64
	for id, count := range left {
		leftNorm += float64(count) * float64(count)
		dot += float64(count) * float64(right[id])
	}
	for _, count := range right {
		rightNorm += float64(count) * float64(count)
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / math.Sqrt(leftNorm*rightNorm)
}

func unique(ids []uint16) []uint16 {
	out := make([]uint16, 0, len(ids))
	for _, id := range ids {
		if id != 0 && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
