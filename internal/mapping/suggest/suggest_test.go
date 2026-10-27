package suggest

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

const (
	grass, snow            = 100, 101
	bush, flower           = 200, 201
	icicle, snowball       = 210, 211
	border                 = 300
	mapFloor         uint8 = 7
)

func ignoreBorder(id uint16) bool { return id == border }

func put(gameMap *otmap.Map, x, y int, ground uint16, items ...uint16) {
	tile := &otmap.Tile{Ground: otmap.Item{ID: ground}}
	for _, id := range items {
		tile.Items = append(tile.Items, otmap.Item{ID: id})
	}
	gameMap.SetTile(otmap.Position{X: uint16(x), Y: uint16(y), Z: mapFloor}, tile)
}

// sampleMap has a grass field with bushes and flowers next to each other
// at x 0-19 and a snow field with icicles and snowballs at x 40-59.
func sampleMap() *otmap.Map {
	gameMap := otmap.New(200, 200)
	for y := range 20 {
		for x := range 20 {
			switch {
			case x%4 == 0:
				put(gameMap, x, y, grass, bush, border)
			case x%4 == 1:
				put(gameMap, x, y, grass, flower)
			default:
				put(gameMap, x, y, grass)
			}
			switch {
			case x%4 == 0:
				put(gameMap, 40+x, y, snow, icicle, snowball)
			default:
				put(gameMap, 40+x, y, snow)
			}
		}
	}
	return gameMap
}

func refreshed(t *testing.T, gameMap *otmap.Map) *Index {
	t.Helper()
	index := New()
	if pending := index.Refresh(gameMap, ignoreBorder, 1000); pending != 0 {
		t.Fatalf("Refresh left %d blocks", pending)
	}
	return index
}

func ids(ranked []Ranked) []uint16 {
	out := make([]uint16, len(ranked))
	for i, entry := range ranked {
		out[i] = entry.ID
	}
	return out
}

func at(x, y int) otmap.Position { return otmap.Position{X: uint16(x), Y: uint16(y), Z: mapFloor} }

func TestHereRanksItemsOfTheGround(t *testing.T) {
	index := refreshed(t, sampleMap())
	got := ids(index.Here(Context{Pos: at(50, 5), Ground: snow}, 4))
	if len(got) != 2 || !slices.Contains(got, icicle) || !slices.Contains(got, snowball) {
		t.Fatalf("Here(on snow) = %v, want the icicle and the snowball", got)
	}
}

func TestHereSkipsItemsOnTheTileAndIgnoredItems(t *testing.T) {
	index := refreshed(t, sampleMap())
	got := ids(index.Here(Context{Pos: at(5, 5), Ground: grass, Items: []uint16{bush}}, 10))
	if slices.Contains(got, bush) || slices.Contains(got, border) {
		t.Fatalf("Here(grass with a bush) = %v, want neither the bush nor the border", got)
	}
}

func TestHereUsesNeighborsByDirection(t *testing.T) {
	index := refreshed(t, sampleMap())
	// Flowers lie east of bushes: a place east of a bush (the bush in its
	// west) gets the flower, a place west of one does not.
	east := Context{Pos: at(5, 5)}
	east.Neighbors[6] = Cell{Items: []uint16{bush}}
	if got := ids(index.Here(east, 1)); len(got) != 1 || got[0] != flower {
		t.Fatalf("Here(east of a bush) = %v, want the flower", got)
	}
	west := Context{Pos: at(5, 5)}
	west.Neighbors[2] = Cell{Items: []uint16{bush}}
	if got := ids(index.Here(west, 4)); slices.Contains(got, flower) {
		t.Fatalf("Here(west of a bush) = %v, want no flower", got)
	}
}

func TestLocalAreaWeighsMore(t *testing.T) {
	gameMap := otmap.New(2000, 2000)
	// Far away, many bushes on grass; here, a few flowers on grass.
	for y := range 40 {
		for x := range 40 {
			if x%2 == 0 {
				put(gameMap, 1000+x, y, grass, bush)
			}
		}
	}
	for x := range 10 {
		put(gameMap, x*2, 0, grass, flower)
	}
	index := refreshed(t, gameMap)
	if got := ids(index.Here(Context{Pos: at(5, 5), Ground: grass}, 1)); len(got) != 1 || got[0] != flower {
		t.Fatalf("Here(grass near the flowers) = %v, want the flower", got)
	}
	if got := ids(index.Here(Context{Pos: at(1010, 5), Ground: grass}, 1)); len(got) != 1 || got[0] != bush {
		t.Fatalf("Here(grass near the bushes) = %v, want the bush", got)
	}
}

func TestSubstitutesTakeTheSamePlaces(t *testing.T) {
	const fern, stone = 202, 203
	gameMap := otmap.New(200, 200)
	for y := range 20 {
		for x := range 30 {
			first := uint16(bush)
			if y >= 10 {
				first = fern
			}
			switch x % 3 {
			case 0:
				put(gameMap, x, y, grass, first)
			case 1:
				put(gameMap, x, y, grass, stone)
			default:
				put(gameMap, x, y, grass)
			}
		}
	}
	index := refreshed(t, gameMap)
	if got := ids(index.Substitutes(bush, 1)); len(got) != 1 || got[0] != fern {
		t.Fatalf("Substitutes(bush) = %v, want the fern", got)
	}
}

func TestCompletionsFindTheOtherPart(t *testing.T) {
	const left, right = 220, 221
	gameMap := otmap.New(200, 200)
	for y := range 10 {
		put(gameMap, 0, y*2, grass, left)
		put(gameMap, 1, y*2, grass, right)
		put(gameMap, 5, y*2, grass, bush)
	}
	index := refreshed(t, gameMap)
	got := index.Completions(left)
	if len(got) != 1 || got[0].ID != right || got[0].Direction != 2 {
		t.Fatalf("Completions(left part) = %+v, want the right part in the east", got)
	}
	if got := index.Completions(bush); len(got) != 0 {
		t.Fatalf("Completions(bush) = %+v, want none", got)
	}
}

func TestWithRanksItemsOfTheSameTile(t *testing.T) {
	index := refreshed(t, sampleMap())
	got := ids(index.With([]uint16{icicle}, at(50, 5), 1))
	if len(got) != 1 || got[0] != snowball {
		t.Fatalf("With(icicle) = %v, want the snowball", got)
	}
}

func TestGroundsRankBorderingGrounds(t *testing.T) {
	gameMap := sampleMap()
	for y := range 20 {
		for x := 20; x < 40; x++ {
			put(gameMap, x, y, snow)
		}
	}
	index := refreshed(t, gameMap)
	got := ids(index.Grounds(Context{Pos: at(5, 5), Ground: grass}, 3))
	if len(got) != 1 || got[0] != snow {
		t.Fatalf("Grounds(grass) = %v, want snow", got)
	}
}

func TestRefreshFollowsEdits(t *testing.T) {
	gameMap := sampleMap()
	index := refreshed(t, gameMap)
	for y := range 20 {
		for x := range 20 {
			put(gameMap, 40+x, y, snow)
		}
	}
	if pending := index.Refresh(gameMap, ignoreBorder, 1000); pending != 0 {
		t.Fatalf("Refresh left %d blocks", pending)
	}
	if got := index.Here(Context{Pos: at(50, 5), Ground: snow}, 4); len(got) != 0 {
		t.Fatalf("Here(on snow) after removing the items = %v, want none", ids(got))
	}
	if index.total.items[icicle] != 0 || index.total.onGround[snow] != nil {
		t.Fatalf("counts of removed items left: icicles %d, on snow %v", index.total.items[icicle], index.total.onGround[snow])
	}
}

func TestRefreshDropsRemovedChunks(t *testing.T) {
	gameMap := otmap.New(200, 200)
	put(gameMap, 100, 100, grass, bush)
	index := refreshed(t, gameMap)
	gameMap.SetTile(otmap.Position{X: 100, Y: 100, Z: mapFloor}, nil)
	gameMap.Prune(otmap.Position{X: 100, Y: 100, Z: mapFloor})
	index.Refresh(gameMap, ignoreBorder, 1000)
	if index.total.tiles != 0 || len(index.total.items) != 0 || len(index.blocks) != 0 {
		t.Fatalf("after removing the only tile: tiles %d, items %v, blocks %d", index.total.tiles, index.total.items, len(index.blocks))
	}
}

func TestRefreshKeepsBudget(t *testing.T) {
	gameMap := sampleMap()
	put(gameMap, 2*blockSide, 0, grass)
	index := New()
	if pending := index.Refresh(gameMap, ignoreBorder, 1); pending != 1 {
		t.Fatalf("Refresh of two blocks with a budget of one left %d blocks, want 1", pending)
	}
}
