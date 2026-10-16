package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

func TestPatchThings(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddThing(thing.CategoryItem)
	project.AddThing(thing.CategoryItem)
	before, _ := project.Thing(thing.CategoryItem, 102)
	before.Props.Market.Name = "keep"
	before.Props.Stackable = true
	project.UpdateThing(before)

	changed, err := project.PatchThings(thing.CategoryItem, []uint32{101, 102}, PropsPatch{
		"pickupable": true,
		"stackable":  false,
		"hasLight":   true,
		"lightLevel": float64(7),
		"market":     map[string]any{"category": float64(3)},
	})
	if err != nil || changed != 2 {
		t.Fatalf("patch %d %v", changed, err)
	}
	got, _ := project.Thing(thing.CategoryItem, 102)
	pr := got.Props
	if !pr.Pickupable || pr.Stackable || !pr.HasLight || pr.LightLevel != 7 || pr.Market.Category != 3 || pr.Market.Name != "keep" {
		t.Fatalf("props %+v", pr)
	}
	if untouched, _ := project.Thing(thing.CategoryItem, 100); untouched.Props.Pickupable {
		t.Fatal("item 100 was not selected")
	}
	if project.Undo() == "" {
		t.Fatal("patch must be undoable")
	}
	if got, _ := project.Thing(thing.CategoryItem, 102); got.Props.Pickupable || !got.Props.Stackable {
		t.Fatal("undo did not restore item 102")
	}

	if _, err := project.PatchThings(thing.CategoryItem, []uint32{101}, PropsPatch{"bogus": true}); err == nil {
		t.Fatal("unknown key must fail")
	}
	if _, err := project.PatchThings(thing.CategoryItem, []uint32{101}, PropsPatch{"lightLevel": "x"}); err == nil {
		t.Fatal("wrong type must fail")
	}
}

func TestUpdateThings(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddThing(thing.CategoryItem)
	objectA, _ := project.Thing(thing.CategoryItem, 100)
	objectB, _ := project.Thing(thing.CategoryItem, 101)
	objectA.Props.Ground, objectB.Props.Ground = true, true
	if err := project.UpdateThings([]*thing.Thing{objectA, objectB}); err != nil {
		t.Fatal(err)
	}
	project.Undo()
	if x, _ := project.Thing(thing.CategoryItem, 101); x.Props.Ground {
		t.Fatal("one undo must revert both")
	}
}

func TestSetDurations(t *testing.T) {
	project := New(v1098(), client.Features{})
	it, _ := project.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Resize(1, 1, 1, 1, 1, 1, 3, 500)
	project.UpdateThing(it)
	changed, err := project.SetDurations([]thing.Category{thing.CategoryItem, thing.CategoryEffect}, 200, 300)
	if err != nil || changed != 1 {
		t.Fatalf("set %d %v", changed, err)
	}
	got, _ := project.Thing(thing.CategoryItem, 100)
	for _, duration := range got.FrameGroups[0].Durations {
		if duration.Min != 200 || duration.Max != 300 {
			t.Fatalf("durations %v", got.FrameGroups[0].Durations)
		}
	}
	if changed, _ := project.SetDurations([]thing.Category{thing.CategoryItem}, 200, 300); changed != 0 {
		t.Fatal("unchanged things must be skipped")
	}
	if _, err := project.SetDurations(nil, 5, 1); err == nil {
		t.Fatal("max below min must fail")
	}
}

func TestConvertFrameGroups(t *testing.T) {
	project := New(v1098(), client.Features{})
	o, _ := project.Thing(thing.CategoryOutfit, 1)
	group := o.FrameGroups[0]
	group.Resize(1, 1, 1, 4, 1, 1, 3, 300)
	for i := range group.Sprites {
		group.Sprites[i] = uint32(i + 1)
	}
	group.Durations[1] = thing.FrameDuration{Min: 111, Max: 111}
	project.AddSprites(make([][]byte, 0))
	for range 12 {
		project.AddSprites([][]byte{solid(1, 1, 1)})
	}
	if err := project.UpdateThing(o); err != nil {
		t.Fatal(err)
	}

	res := project.ConvertFrameGroups(true)
	if res.Converted != 1 {
		t.Fatalf("split %+v", res)
	}
	split, _ := project.Thing(thing.CategoryOutfit, 1)
	idle, walking := split.FrameGroups[0], split.FrameGroups[1]
	if len(split.FrameGroups) != 2 || idle.Frames != 1 || walking.Frames != 2 || walking.Type != thing.FrameGroupWalking {
		t.Fatalf("groups %+v %+v", idle, walking)
	}
	if !slices.Equal(idle.Sprites, []uint32{1, 2, 3, 4}) || !slices.Equal(walking.Sprites, []uint32{5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatalf("sprites %v %v", idle.Sprites, walking.Sprites)
	}
	if walking.Durations[0].Min != 111 {
		t.Fatalf("walk durations %v", walking.Durations)
	}
	if err := split.Validate(); err != nil {
		t.Fatal(err)
	}

	if res := project.ConvertFrameGroups(false); res.Converted != 1 {
		t.Fatalf("merge %+v", res)
	}
	merged, _ := project.Thing(thing.CategoryOutfit, 1)
	if len(merged.FrameGroups) != 1 || !slices.Equal(merged.FrameGroups[0].Sprites, group.Sprites) || merged.FrameGroups[0].Durations[1].Min != 111 {
		t.Fatalf("round trip %+v", merged.FrameGroups[0])
	}
	if err := merged.Validate(); err != nil {
		t.Fatal(err)
	}
}
