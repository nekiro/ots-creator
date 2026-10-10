package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

// order returns the first sprite id of items from..to, which tells the
// items apart in these tests.
func order(t *testing.T, p *Project, from, to uint32) []uint32 {
	t.Helper()
	var out []uint32
	for id := from; id <= to; id++ {
		it, err := p.Thing(thing.CategoryItem, id)
		if err != nil {
			t.Fatal(err)
		}
		if it.ID != id {
			t.Fatalf("item %d has id %d", id, it.ID)
		}
		out = append(out, it.FrameGroups[0].Sprites[0])
	}
	return out
}

func reorderProject(t *testing.T) *Project {
	t.Helper()
	p := New(v1098(), client.Features{})
	px := make([][]byte, 6)
	for i := range px {
		px[i] = solid(uint8(i*40), 0, 0)
	}
	p.AddSprites(px)
	for i := uint32(0); i < 6; i++ {
		setItem(t, p, 100+i, false, i+1)
	}
	return p
}

func TestMoveThingsSwap(t *testing.T) {
	p := reorderProject(t)
	// 101 and 102 go to 104 and 105; those two take the freed ids.
	got, err := p.MoveThings(thing.CategoryItem, []uint32{102, 101}, 104, MoveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []uint32{105, 104}) {
		t.Fatalf("new ids %v", got)
	}
	if o := order(t, p, 100, 105); !slices.Equal(o, []uint32{1, 5, 6, 4, 2, 3}) {
		t.Fatalf("order %v", o)
	}
	if p.Undo() == "" {
		t.Fatal("no undo")
	}
	if o := order(t, p, 100, 105); !slices.Equal(o, []uint32{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("after undo %v", o)
	}
	// Overlapping block, and past the end.
	if _, err := p.MoveThings(thing.CategoryItem, []uint32{104, 105}, 105, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	if p.Info().Counts.Items != 106 {
		t.Fatalf("items %d", p.Info().Counts.Items)
	}
	if o := order(t, p, 103, 106); !slices.Equal(o, []uint32{4, 0, 5, 6}) {
		t.Fatalf("order %v", o)
	}
}

func TestMoveThingsInsert(t *testing.T) {
	p := reorderProject(t)
	// 104 goes before 101; 101-103 shift up.
	if _, err := p.MoveThings(thing.CategoryItem, []uint32{104}, 101, MoveOptions{Insert: true}); err != nil {
		t.Fatal(err)
	}
	if o := order(t, p, 100, 105); !slices.Equal(o, []uint32{1, 5, 2, 3, 4, 6}) {
		t.Fatalf("order %v", o)
	}
	// 100 and 101 go to the end.
	got, err := p.MoveThings(thing.CategoryItem, []uint32{100, 101}, 999, MoveOptions{Insert: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []uint32{104, 105}) {
		t.Fatalf("new ids %v", got)
	}
	if o := order(t, p, 100, 105); !slices.Equal(o, []uint32{2, 3, 4, 6, 1, 5}) {
		t.Fatalf("order %v", o)
	}
}

func TestMoveThingsRefs(t *testing.T) {
	p := reorderProject(t)
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.Props.Market.TradeAs = 101
	it.NpcSales = []thing.NpcSale{{CurrencyObjectID: 105}}
	p.UpdateThing(it)
	if _, err := p.MoveThings(thing.CategoryItem, []uint32{101}, 105, MoveOptions{UpdateRefs: true}); err != nil {
		t.Fatal(err)
	}
	it, _ = p.Thing(thing.CategoryItem, 100)
	if it.Props.Market.TradeAs != 105 || it.NpcSales[0].CurrencyObjectID != 101 {
		t.Fatalf("refs %d %d", it.Props.Market.TradeAs, it.NpcSales[0].CurrencyObjectID)
	}
	if _, err := p.MoveThings(thing.CategoryItem, []uint32{100, 100}, 103, MoveOptions{}); err == nil {
		t.Fatal("moved twice")
	}
	if _, err := p.MoveThings(thing.CategoryItem, []uint32{500}, 103, MoveOptions{}); err == nil {
		t.Fatal("missing item")
	}
}
