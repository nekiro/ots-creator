package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

func TestFindThings(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(1, 2, 3), make([]byte, 32*32*4)})
	for range 3 {
		if _, err := project.AddThing(thing.CategoryItem); err != nil {
			t.Fatal(err)
		}
	}
	// 101 has pixels and is ground, 102 points at a blank sprite, 103 has none.
	for id, spr := range map[uint32]uint32{101: 1, 102: 2} {
		it, _ := project.Thing(thing.CategoryItem, id)
		it.FrameGroups[0].Sprites[0] = spr
		it.Props.Ground = id == 101
		if err := project.UpdateThing(it); err != nil {
			t.Fatal(err)
		}
	}

	check := func(filter Filter, want []uint32) {
		t.Helper()
		got, err := project.FindThings(thing.CategoryItem, filter)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("%+v: got %v %v, want %v", filter, got, err, want)
		}
	}
	check(Filter{Content: ContentUsed}, []uint32{101})
	check(Filter{Content: ContentEmpty}, []uint32{100, 102, 103})
	check(Filter{Flag: "ground"}, []uint32{101})
	check(Filter{Content: ContentEmpty, Flag: "ground"}, []uint32{})
	if all, _ := project.FindThings(thing.CategoryItem, Filter{}); len(all) != 4 {
		t.Fatalf("all %v", all)
	}
	if _, err := project.FindThings(thing.CategoryItem, Filter{Flag: "groundSpeed"}); err == nil {
		t.Fatal("non-boolean flag must fail")
	}
	if _, err := project.FindThings(thing.CategoryItem, Filter{Content: "bogus"}); err == nil {
		t.Fatal("unknown content must fail")
	}
}

func TestNames(t *testing.T) {
	project := New(v1098(), client.Features{})
	if _, err := project.AddThing(thing.CategoryItem); err != nil {
		t.Fatal(err)
	}
	named, _ := project.Thing(thing.CategoryItem, 101)
	named.Name = "sword"
	if err := project.UpdateThing(named); err != nil {
		t.Fatal(err)
	}
	names := project.Names(thing.CategoryItem)
	if len(names) != 1 || names[101] != "sword" {
		t.Fatalf("Names = %v", names)
	}
	if len(project.Names(thing.CategoryOutfit)) != 0 {
		t.Fatal("outfits have no names")
	}
}
