package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestDeltaTracksEdits(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(1, 0, 0), solid(0, 1, 0)})
	p.AddThing(thing.CategoryItem)
	it, _ := p.Thing(thing.CategoryItem, 101)
	it.FrameGroups[0].Sprites[0] = 2
	p.UpdateThing(it)
	p.TakeDelta()

	// Replacing sprite 2 touches the sprite and item 101 that uses it.
	p.ReplaceSprite(2, solid(9, 9, 9))
	d := p.TakeDelta()
	if d.All || !slices.Equal(d.Sprites, []uint32{2}) || !slices.Equal(d.Things, []ThingRef{{thing.CategoryItem, 101}}) {
		t.Fatalf("replace delta %+v", d)
	}
	if d := p.TakeDelta(); len(d.Things)+len(d.Sprites) != 0 || d.All {
		t.Fatalf("delta must reset, got %+v", d)
	}

	// Undo of adding a thing touches the removed slot.
	p.AddThing(thing.CategoryEffect)
	p.TakeDelta()
	p.Undo()
	if d := p.TakeDelta(); !slices.Contains(d.Things, ThingRef{thing.CategoryEffect, 2}) {
		t.Fatalf("undo delta %+v", d)
	}

	// Large sprite changes collapse into "all".
	many := make([][]byte, deltaLimit+1)
	for i := range many {
		many[i] = solid(byte(i), 1, 1)
	}
	p.AddSprites(many)
	if d := p.TakeDelta(); !d.All {
		t.Fatal("big change must report all")
	}
}
