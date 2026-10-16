package project

import (
	"errors"
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/client"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// dot returns a transparent sprite with one red pixel at (x, y).
func dot(x, y int) []byte {
	pixels := make([]byte, 32*32*4)
	offset := (y*32 + x) * 4
	pixels[offset], pixels[offset+3] = 255, 255
	return pixels
}

func TestSetSlotPixelsNeverEditsSharedSprites(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(0, 0, 255)})
	setItem(t, project, 100, false, 1, 1) // both slots share sprite 1
	setItem(t, project, 101, false, 1)

	if err := project.SetSlotPixels(thing.CategoryItem, 100, 0, []int{0, 1}, [][]byte{dot(1, 1), solid(0, 0, 255)}, "Paint"); err != nil {
		t.Fatal(err)
	}
	it, _ := project.Thing(thing.CategoryItem, 100)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{2, 1}) {
		t.Fatalf("sprites %v", it.FrameGroups[0].Sprites)
	}
	if pixels, _ := project.SpritePixels(1); pixels[2] != 255 {
		t.Fatal("shared sprite 1 was changed")
	}
	// Clearing a slot and an unchanged edit.
	if err := project.SetSlotPixels(thing.CategoryItem, 100, 0, []int{0}, [][]byte{make([]byte, 32*32*4)}, "Erase"); err != nil {
		t.Fatal(err)
	}
	it, _ = project.Thing(thing.CategoryItem, 100)
	if it.FrameGroups[0].Sprites[0] != 0 || project.Info().Counts.Sprites != 2 {
		t.Fatalf("after erase %v, %d sprites", it.FrameGroups[0].Sprites, project.Info().Counts.Sprites)
	}
	project.Undo()
	project.Undo()
	it, _ = project.Thing(thing.CategoryItem, 100)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{1, 1}) || project.Info().Counts.Sprites != 1 {
		t.Fatalf("after undo %v", it.FrameGroups[0].Sprites)
	}
	if err := project.SetSlotPixels(thing.CategoryItem, 100, 0, []int{5}, [][]byte{dot(0, 0)}, "Paint"); err == nil {
		t.Fatal("bad slot accepted")
	}
}

func TestShiftPixels(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{dot(31, 5)})
	// A 2x1 item: tile 0 is the right one, so the dot sits at x 63.
	setItem(t, project, 100, false, 1, 0)

	if err := project.ShiftPixels(thing.CategoryItem, 100, 0, 1, 0, true, TexturePos{}); !errors.Is(err, ErrShiftCut) {
		t.Fatalf("err = %v", err)
	}
	// Moving left by 32 carries the dot into the left tile.
	if err := project.ShiftPixels(thing.CategoryItem, 100, 0, -32, 2, true, TexturePos{}); err != nil {
		t.Fatal(err)
	}
	it, _ := project.Thing(thing.CategoryItem, 100)
	group := it.FrameGroups[0]
	if group.Sprites[0] != 0 || group.Sprites[1] == 0 {
		t.Fatalf("sprites %v", group.Sprites)
	}
	pixels, _ := project.SpritePixels(group.Sprites[1])
	if pixels[(7*32+31)*4] != 255 {
		t.Fatal("dot not moved to (31, 7) of the left tile")
	}
}
