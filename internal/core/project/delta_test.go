package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

func TestDeltaTracksEdits(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(1, 0, 0), solid(0, 1, 0)})
	project.AddThing(thing.CategoryItem)
	it, _ := project.Thing(thing.CategoryItem, 101)
	it.FrameGroups[0].Sprites[0] = 2
	project.UpdateThing(it)
	project.TakeDelta()

	// Replacing sprite 2 touches the sprite and item 101 that uses it.
	project.ReplaceSprite(2, solid(9, 9, 9))
	d := project.TakeDelta()
	if d.All || !slices.Equal(d.Sprites, []uint32{2}) || !slices.Equal(d.Things, []ThingRef{{thing.CategoryItem, 101}}) {
		t.Fatalf("replace delta %+v", d)
	}
	if d := project.TakeDelta(); len(d.Things)+len(d.Sprites) != 0 || d.All {
		t.Fatalf("delta must reset, got %+v", d)
	}

	project.AddThing(thing.CategoryEffect)
	project.TakeDelta()
	project.Undo()
	if d := project.TakeDelta(); !slices.Contains(d.Things, ThingRef{thing.CategoryEffect, 2}) {
		t.Fatalf("undo delta %+v", d)
	}

	// Large sprite changes collapse into "all".
	many := make([][]byte, deltaLimit+1)
	for i := range many {
		many[i] = solid(byte(i), 1, 1)
	}
	project.AddSprites(many)
	if d := project.TakeDelta(); !d.All {
		t.Fatal("big change must report all")
	}
}
