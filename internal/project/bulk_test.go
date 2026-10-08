package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestPatchThings(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddThing(thing.CategoryItem)
	p.AddThing(thing.CategoryItem)
	before, _ := p.Thing(thing.CategoryItem, 102)
	before.Props.Market.Name = "keep"
	before.Props.Stackable = true
	p.UpdateThing(before)

	n, err := p.PatchThings(thing.CategoryItem, []uint32{101, 102}, PropsPatch{
		"pickupable": true,
		"stackable":  false,
		"hasLight":   true,
		"lightLevel": float64(7),
		"market":     map[string]any{"category": float64(3)},
	})
	if err != nil || n != 2 {
		t.Fatalf("patch %d %v", n, err)
	}
	got, _ := p.Thing(thing.CategoryItem, 102)
	pr := got.Props
	if !pr.Pickupable || pr.Stackable || !pr.HasLight || pr.LightLevel != 7 || pr.Market.Category != 3 || pr.Market.Name != "keep" {
		t.Fatalf("props %+v", pr)
	}
	if untouched, _ := p.Thing(thing.CategoryItem, 100); untouched.Props.Pickupable {
		t.Fatal("item 100 was not selected")
	}
	if p.Undo() == "" {
		t.Fatal("patch must be undoable")
	}
	if got, _ := p.Thing(thing.CategoryItem, 102); got.Props.Pickupable || !got.Props.Stackable {
		t.Fatal("undo did not restore item 102")
	}

	if _, err := p.PatchThings(thing.CategoryItem, []uint32{101}, PropsPatch{"bogus": true}); err == nil {
		t.Fatal("unknown key must fail")
	}
	if _, err := p.PatchThings(thing.CategoryItem, []uint32{101}, PropsPatch{"lightLevel": "x"}); err == nil {
		t.Fatal("wrong type must fail")
	}
}

func TestUpdateThings(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddThing(thing.CategoryItem)
	a, _ := p.Thing(thing.CategoryItem, 100)
	b, _ := p.Thing(thing.CategoryItem, 101)
	a.Props.Ground, b.Props.Ground = true, true
	if err := p.UpdateThings([]*thing.Thing{a, b}); err != nil {
		t.Fatal(err)
	}
	p.Undo()
	if x, _ := p.Thing(thing.CategoryItem, 101); x.Props.Ground {
		t.Fatal("one undo must revert both")
	}
}

func TestSetDurations(t *testing.T) {
	p := New(v1098(), client.Features{})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Resize(1, 1, 1, 1, 1, 1, 3, 500)
	p.UpdateThing(it)
	n, err := p.SetDurations([]thing.Category{thing.CategoryItem, thing.CategoryEffect}, 200, 300)
	if err != nil || n != 1 {
		t.Fatalf("set %d %v", n, err)
	}
	got, _ := p.Thing(thing.CategoryItem, 100)
	for _, d := range got.FrameGroups[0].Durations {
		if d.Min != 200 || d.Max != 300 {
			t.Fatalf("durations %v", got.FrameGroups[0].Durations)
		}
	}
	if n, _ := p.SetDurations([]thing.Category{thing.CategoryItem}, 200, 300); n != 0 {
		t.Fatal("unchanged things must be skipped")
	}
	if _, err := p.SetDurations(nil, 5, 1); err == nil {
		t.Fatal("max below min must fail")
	}
}

func TestConvertFrameGroups(t *testing.T) {
	p := New(v1098(), client.Features{})
	o, _ := p.Thing(thing.CategoryOutfit, 1)
	g := o.FrameGroups[0]
	g.Resize(1, 1, 1, 4, 1, 1, 3, 300)
	for i := range g.Sprites {
		g.Sprites[i] = uint32(i + 1)
	}
	g.Durations[1] = thing.FrameDuration{Min: 111, Max: 111}
	p.AddSprites(make([][]byte, 0))
	for range 12 {
		p.AddSprites([][]byte{solid(1, 1, 1)})
	}
	if err := p.UpdateThing(o); err != nil {
		t.Fatal(err)
	}

	res := p.ConvertFrameGroups(true)
	if res.Converted != 1 {
		t.Fatalf("split %+v", res)
	}
	split, _ := p.Thing(thing.CategoryOutfit, 1)
	idle, walk := split.FrameGroups[0], split.FrameGroups[1]
	if len(split.FrameGroups) != 2 || idle.Frames != 1 || walk.Frames != 2 || walk.Type != thing.FrameGroupWalking {
		t.Fatalf("groups %+v %+v", idle, walk)
	}
	if !slices.Equal(idle.Sprites, []uint32{1, 2, 3, 4}) || !slices.Equal(walk.Sprites, []uint32{5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatalf("sprites %v %v", idle.Sprites, walk.Sprites)
	}
	if walk.Durations[0].Min != 111 {
		t.Fatalf("walk durations %v", walk.Durations)
	}
	if err := split.Validate(); err != nil {
		t.Fatal(err)
	}

	if res := p.ConvertFrameGroups(false); res.Converted != 1 {
		t.Fatalf("merge %+v", res)
	}
	merged, _ := p.Thing(thing.CategoryOutfit, 1)
	if len(merged.FrameGroups) != 1 || !slices.Equal(merged.FrameGroups[0].Sprites, g.Sprites) || merged.FrameGroups[0].Durations[1].Min != 111 {
		t.Fatalf("round trip %+v", merged.FrameGroups[0])
	}
	if err := merged.Validate(); err != nil {
		t.Fatal(err)
	}
}
