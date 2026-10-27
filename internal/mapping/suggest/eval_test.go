package suggest

import (
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// itemClass groups the samples: walls and other bottom items, objects
// that stand (blocking or not) and items a player can pick up.
func itemClass(items *otitems.DB, id uint16) string {
	itemType := items.Get(id)
	switch {
	case itemType == nil:
		return "unknown"
	case itemType.TopOrder == 2:
		return "walls"
	case itemType.Flags.Has(otitems.FlagPickupable):
		return "loose items"
	case itemType.Flags.Has(otitems.FlagUnpassable):
		return "blocking objects"
	default:
		return "decoration"
	}
}

// TestSuggestionQuality measures the suggestions on a real map: it takes
// one item off random tiles, counts the map without them and asks what fits
// each tile. A hit at rank k means the item taken off is among the first k
// suggestions, the way a mapper looking for it would see it.
//
//	OTS_MAP=path/map.otbm OTS_ITEMS=path/items.otb|OTS_CLIENT=path/client [OTS_SAMPLES=2000] go test -run TestSuggestionQuality -v ./internal/mapping/suggest
func TestSuggestionQuality(t *testing.T) {
	samples := 2000
	if count, err := strconv.Atoi(os.Getenv("OTS_SAMPLES")); err == nil && count > 0 {
		samples = count
	}
	gameMap, items, ignore := loadEnvMap(t)
	removed := takeSamples(gameMap, ignore, samples, 1)
	index := New()
	index.Refresh(gameMap, ignore, 1<<30)
	t.Logf("%d samples on %d tiles", len(removed), gameMap.TileCount())

	configs := []struct {
		name   string
		tuning Tuning
	}{
		{"default", DefaultTuning},
		{"most common on the ground", Tuning{Ground: 1}},
		{"no local area", with(DefaultTuning, func(tuning *Tuning) { tuning.LocalWeight = 0 })},
		{"local x1", with(DefaultTuning, func(tuning *Tuning) { tuning.LocalWeight = 1 })},
		{"local x10", with(DefaultTuning, func(tuning *Tuning) { tuning.LocalWeight = 10 })},
		{"local radius 0", with(DefaultTuning, func(tuning *Tuning) { tuning.LocalRadius = 0 })},
		{"local radius 2", with(DefaultTuning, func(tuning *Tuning) { tuning.LocalRadius = 2 })},
		{"no directions", with(DefaultTuning, func(tuning *Tuning) { tuning.Directional = false })},
		{"no lift", with(DefaultTuning, func(tuning *Tuning) { tuning.Lift = false })},
		{"ground 3", with(DefaultTuning, func(tuning *Tuning) { tuning.Ground = 3 })},
		{"ground 6", with(DefaultTuning, func(tuning *Tuning) { tuning.Ground = 6 })},
		{"near 4", with(DefaultTuning, func(tuning *Tuning) { tuning.Near = 4 })},
		{"near 0", with(DefaultTuning, func(tuning *Tuning) { tuning.Near = 0 })},
		{"tile 0", with(DefaultTuning, func(tuning *Tuning) { tuning.Tile = 0 })},
		{"tile 4", with(DefaultTuning, func(tuning *Tuning) { tuning.Tile = 4 })},
		{"ground 1, near 4", with(DefaultTuning, func(tuning *Tuning) { tuning.Ground, tuning.Near = 1, 4 })},
		{"ground 1, near 4, r2", with(DefaultTuning, func(tuning *Tuning) { tuning.Ground, tuning.Near, tuning.LocalRadius = 1, 4, 2 })},
		{"ground 0.5, near 4, r2", with(DefaultTuning, func(tuning *Tuning) { tuning.Ground, tuning.Near, tuning.LocalRadius = 0.5, 4, 2 })},
	}
	classes := []string{"all", "walls", "blocking objects", "decoration", "loose items"}
	for _, config := range configs {
		index.tuning = config.tuning
		start := time.Now()
		scores := map[string]*score{}
		for _, class := range classes {
			scores[class] = &score{}
		}
		for _, sample := range removed {
			ranked := index.Here(contextAt(gameMap, sample.pos, ignore), 12)
			rank := slices.IndexFunc(ranked, func(entry Ranked) bool { return entry.ID == sample.item })
			scores["all"].add(rank)
			if class := scores[itemClass(items, sample.item)]; class != nil {
				class.add(rank)
			}
		}
		t.Logf("%-26s all %v | decoration %v  (%v per query)", config.name, scores["all"], scores["decoration"], time.Since(start)/time.Duration(len(removed)))
		if config.name == "default" {
			for _, class := range classes[1:] {
				t.Logf("  %-24s %v", class, scores[class])
			}
		}
	}

	// Completions: the parts the items around a tile predict for it, and
	// how many of them are the item taken off.
	index.tuning = DefaultTuning
	var predicted, right, found int
	for _, sample := range removed {
		hit := false
		for direction, offset := range Offsets {
			neighbor := contextAt(gameMap, otmap.Position{X: uint16(int(sample.pos.X) + offset[0]), Y: uint16(int(sample.pos.Y) + offset[1]), Z: sample.pos.Z}, ignore)
			for _, item := range neighbor.Items {
				for _, part := range index.Completions(item) {
					if part.Direction != opposite(direction) || slices.Contains(contextAt(gameMap, sample.pos, ignore).Items, part.ID) {
						continue
					}
					predicted++
					if part.ID == sample.item {
						right++
						hit = true
					}
				}
			}
		}
		if hit {
			found++
		}
	}
	t.Logf("completions: %d ghosts, %d right (%.1f%%); the item taken off is predicted on %.1f%% of the tiles",
		predicted, right, 100*float64(right)/float64(max(predicted, 1)), 100*float64(found)/float64(len(removed)))
}

// score counts the ranks of the items taken off (-1: not suggested).
type score struct {
	count, top1, top4, top12 int
	reciprocal               float64
}

func (s *score) add(rank int) {
	s.count++
	if rank < 0 {
		return
	}
	s.reciprocal += 1 / float64(rank+1)
	s.top12++
	if rank < 4 {
		s.top4++
	}
	if rank < 1 {
		s.top1++
	}
}

func (s *score) String() string {
	count := float64(max(s.count, 1))
	return fmt.Sprintf("n %4d  top1 %5.1f%%  top4 %5.1f%%  top12 %5.1f%%  mrr %.3f", s.count,
		100*float64(s.top1)/count, 100*float64(s.top4)/count, 100*float64(s.top12)/count, s.reciprocal/count)
}

func with(tuning Tuning, change func(tuning *Tuning)) Tuning {
	change(&tuning)
	return tuning
}

type removal struct {
	pos  otmap.Position
	item uint16
}

// takeSamples takes one item off up to count random tiles with items, an
// item that is on its tile once.
func takeSamples(gameMap *otmap.Map, ignore func(id uint16) bool, count int, seed uint64) []removal {
	random := rand.New(rand.NewPCG(seed, seed))
	var chosen []removal
	seen := 0
	gameMap.Each(func(pos otmap.Position, tile *otmap.Tile) {
		var single []uint16
		for _, item := range tile.Items {
			if ignore(item.ID) {
				continue
			}
			copies := 0
			for _, other := range tile.Items {
				if other.ID == item.ID {
					copies++
				}
			}
			if copies == 1 {
				single = append(single, item.ID)
			}
		}
		if len(single) == 0 {
			return
		}
		sample := removal{pos, single[random.IntN(len(single))]}
		seen++
		if len(chosen) < count {
			chosen = append(chosen, sample)
		} else if slot := random.IntN(seen); slot < count {
			chosen[slot] = sample
		}
	})
	for _, sample := range chosen {
		tile := gameMap.Tile(sample.pos).Clone()
		tile.Items = slices.DeleteFunc(tile.Items, func(item otmap.Item) bool { return item.ID == sample.item })
		gameMap.SetTile(sample.pos, tile)
	}
	return chosen
}

// contextAt is the place at a position as the editor sees it.
func contextAt(gameMap *otmap.Map, pos otmap.Position, ignore func(id uint16) bool) Context {
	place := Context{Pos: pos}
	collect := func(tile *otmap.Tile) []uint16 {
		var ids []uint16
		for _, item := range tile.Items {
			if !ignore(item.ID) {
				ids = append(ids, item.ID)
			}
		}
		return ids
	}
	if tile := gameMap.Tile(pos); tile != nil {
		place.Ground, place.Items = tile.Ground.ID, collect(tile)
	}
	for direction, offset := range Offsets {
		if tile := gameMap.PeekAt(int(pos.X)+offset[0], int(pos.Y)+offset[1], int(pos.Z), &otmap.Tile{}, nil); tile != nil {
			place.Neighbors[direction] = Cell{Ground: tile.Ground.ID, Items: collect(tile)}
		}
	}
	return place
}
