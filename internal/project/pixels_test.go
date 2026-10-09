package project

import (
	"errors"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

// dot returns a transparent sprite with one red pixel at (x, y).
func dot(x, y int) []byte {
	px := make([]byte, 32*32*4)
	o := (y*32 + x) * 4
	px[o], px[o+3] = 255, 255
	return px
}

func TestSetSlotPixelsNeverEditsSharedSprites(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, p, 100, false, 1, 1) // both slots share sprite 1
	setItem(t, p, 101, false, 1)

	if err := p.SetSlotPixels(thing.CategoryItem, 100, 0, []int{0, 1}, [][]byte{dot(1, 1), solid(0, 0, 255)}, "Paint"); err != nil {
		t.Fatal(err)
	}
	it, _ := p.Thing(thing.CategoryItem, 100)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{2, 1}) {
		t.Fatalf("sprites %v", it.FrameGroups[0].Sprites)
	}
	if px, _ := p.SpritePixels(1); px[2] != 255 {
		t.Fatal("shared sprite 1 was changed")
	}
	// Clearing a slot and an unchanged edit.
	if err := p.SetSlotPixels(thing.CategoryItem, 100, 0, []int{0}, [][]byte{make([]byte, 32*32*4)}, "Erase"); err != nil {
		t.Fatal(err)
	}
	it, _ = p.Thing(thing.CategoryItem, 100)
	if it.FrameGroups[0].Sprites[0] != 0 || p.Info().Counts.Sprites != 2 {
		t.Fatalf("after erase %v, %d sprites", it.FrameGroups[0].Sprites, p.Info().Counts.Sprites)
	}
	p.Undo()
	p.Undo()
	it, _ = p.Thing(thing.CategoryItem, 100)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{1, 1}) || p.Info().Counts.Sprites != 1 {
		t.Fatalf("after undo %v", it.FrameGroups[0].Sprites)
	}
	if err := p.SetSlotPixels(thing.CategoryItem, 100, 0, []int{5}, [][]byte{dot(0, 0)}, "Paint"); err == nil {
		t.Fatal("bad slot accepted")
	}
}

func TestShiftPixels(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{dot(31, 5)})
	// A 2x1 item: tile 0 is the right one, so the dot sits at x 63.
	setItem(t, p, 100, false, 1, 0)

	if err := p.ShiftPixels(thing.CategoryItem, 100, 0, 1, 0, true, TexturePos{}); !errors.Is(err, ErrShiftCut) {
		t.Fatalf("err = %v", err)
	}
	// Moving left by 32 carries the dot into the left tile.
	if err := p.ShiftPixels(thing.CategoryItem, 100, 0, -32, 2, true, TexturePos{}); err != nil {
		t.Fatal(err)
	}
	it, _ := p.Thing(thing.CategoryItem, 100)
	g := it.FrameGroups[0]
	if g.Sprites[0] != 0 || g.Sprites[1] == 0 {
		t.Fatalf("sprites %v", g.Sprites)
	}
	px, _ := p.SpritePixels(g.Sprites[1])
	if px[(7*32+31)*4] != 255 {
		t.Fatal("dot not moved to (31, 7) of the left tile")
	}
}
