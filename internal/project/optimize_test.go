package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestOptimizeSprites(t *testing.T) {
	p := New(v1098(), client.Features{})
	// 1 red, 2 empty, 3 red (duplicate of 1), 4 green (unused), 5 blue.
	p.AddSprites([][]byte{solid(255, 0, 0), make([]byte, 32*32*4), solid(255, 0, 0), solid(0, 255, 0), solid(0, 0, 255)})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Resize(2, 2, 1, 1, 1, 1, 1, 0)
	copy(it.FrameGroups[0].Sprites, []uint32{1, 2, 3, 5})
	if err := p.UpdateThing(it); err != nil {
		t.Fatal(err)
	}

	res, err := p.OptimizeSprites(OptimizeOptions{Duplicates: true, Unused: true, Empty: true})
	if err != nil {
		t.Fatal(err)
	}
	want := OptimizeResult{Before: 5, After: 2, Duplicates: 1, Unused: 1, Empty: 1, Things: 1}
	if res != want {
		t.Fatalf("result %+v, want %+v", res, want)
	}
	got, _ := p.Thing(thing.CategoryItem, 100)
	if !slices.Equal(got.FrameGroups[0].Sprites, []uint32{1, 0, 1, 2}) {
		t.Fatalf("sprites %v", got.FrameGroups[0].Sprites)
	}
	if px, _ := p.SpritePixels(2); px[2] != 255 || px[0] != 0 {
		t.Fatalf("sprite 2 must be blue, got %v", px[:4])
	}
	if p.Info().Counts.Sprites != 2 {
		t.Fatalf("count %d", p.Info().Counts.Sprites)
	}

	p.Undo()
	if p.Info().Counts.Sprites != 5 {
		t.Fatalf("undo count %d", p.Info().Counts.Sprites)
	}
	for id, g := range map[uint32]byte{4: 255, 5: 0} {
		if px, _ := p.SpritePixels(id); px[1] != g {
			t.Fatalf("undo sprite %d pixels %v", id, px[:4])
		}
	}
	back, _ := p.Thing(thing.CategoryItem, 100)
	if !slices.Equal(back.FrameGroups[0].Sprites, []uint32{1, 2, 3, 5}) {
		t.Fatalf("undo sprites %v", back.FrameGroups[0].Sprites)
	}
	p.Redo()
	if p.Info().Counts.Sprites != 2 {
		t.Fatal("redo")
	}
}

func TestOptimizeKeepsUnusedWhenAsked(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(1, 0, 0), solid(1, 0, 0)})
	res, _ := p.OptimizeSprites(OptimizeOptions{Duplicates: true})
	if res.After != 1 || res.Unused != 0 || res.Duplicates != 1 {
		t.Fatalf("%+v", res)
	}
}

// Sprites from the spr file (not the edit overlay) must survive undo too.
func TestOptimizeUndoWithBaseFile(t *testing.T) {
	dir := t.TempDir()
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(9, 0, 0), solid(0, 9, 0), solid(0, 0, 9)})
	o := CompileOptions{DatPath: dir + "/Tibia.dat", SprPath: dir + "/Tibia.spr", Version: v1098(), Features: p.Info().Features}
	if err := p.Compile(o); err != nil {
		t.Fatal(err)
	}
	q, err := Open(OpenOptions{DatPath: o.DatPath, SprPath: o.SprPath})
	if err != nil {
		t.Fatal(err)
	}
	if res, _ := q.OptimizeSprites(OptimizeOptions{Unused: true}); res.After != 0 {
		t.Fatalf("%+v", res)
	}
	q.Undo()
	for id, want := range map[uint32][3]byte{1: {9, 0, 0}, 2: {0, 9, 0}, 3: {0, 0, 9}} {
		px, err := q.SpritePixels(id)
		if err != nil || [3]byte(px[:3]) != want {
			t.Fatalf("sprite %d %v %v", id, px[:4], err)
		}
	}
}
