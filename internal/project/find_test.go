package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestFindThings(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(1, 2, 3), make([]byte, 32*32*4)})
	for range 3 {
		if _, err := p.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	// 101 has pixels and is ground, 102 points at a blank sprite, 103 has none.
	for id, spr := range map[uint32]uint32{101: 1, 102: 2} {
		it, _ := p.Thing(thing.CategoryItem, id)
		it.FrameGroups[0].Sprites[0] = spr
		it.Props.Ground = id == 101
		if err := p.UpdateThing(it); err != nil {
			t.Fatal(err)
		}
	}

	check := func(f Filter, want []uint32) {
		t.Helper()
		got, err := p.FindThings(thing.CategoryItem, f)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("%+v: got %v %v, want %v", f, got, err, want)
		}
	}
	check(Filter{Content: ContentUsed}, []uint32{101})
	check(Filter{Content: ContentEmpty}, []uint32{100, 102, 103})
	check(Filter{Flag: "ground"}, []uint32{101})
	check(Filter{Content: ContentEmpty, Flag: "ground"}, []uint32{})
	if all, _ := p.FindThings(thing.CategoryItem, Filter{}); len(all) != 4 {
		t.Fatalf("all %v", all)
	}
	if _, err := p.FindThings(thing.CategoryItem, Filter{Flag: "groundSpeed"}); err == nil {
		t.Fatal("non-boolean flag must fail")
	}
	if _, err := p.FindThings(thing.CategoryItem, Filter{Content: "bogus"}); err == nil {
		t.Fatal("unknown content must fail")
	}
}
