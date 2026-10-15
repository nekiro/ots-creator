package project

import (
	"os"
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

	res, err := p.OptimizeSprites(OptimizeOptions{Duplicates: true, Unused: true, Empty: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 3 removed addresses, red and green data (Bytes is checked against a real spr below).
	want := OptimizeResult{Before: 5, After: 2, Duplicates: 1, Unused: 1, Empty: 1, Things: 1, Bytes: res.Bytes}
	if res != want || res.Bytes <= 3*4 {
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
	res, _ := p.OptimizeSprites(OptimizeOptions{Duplicates: true}, nil)
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
	if res, _ := q.OptimizeSprites(OptimizeOptions{Unused: true}, nil); res.After != 0 {
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

func TestAnalyzeSpritesChangesNothing(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(1, 0, 0), solid(1, 0, 0), make([]byte, 32*32*4)})
	res, err := p.AnalyzeSprites(OptimizeOptions{Duplicates: true, Empty: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Before != 3 || res.After != 1 || res.Duplicates != 1 || res.Empty != 1 {
		t.Fatalf("%+v", res)
	}
	if p.Info().Counts.Sprites != 3 {
		t.Fatal("analyze changed the project")
	}
}

// Sprites moved from the spr file read their old data and compile to it.
func TestOptimizeMovesBaseSprites(t *testing.T) {
	dir := t.TempDir()
	p := New(v1098(), client.Features{})
	// 1 red, 2 red (duplicate), 3 empty, 4 blue.
	p.AddSprites([][]byte{solid(9, 0, 0), solid(9, 0, 0), make([]byte, 32*32*4), solid(0, 0, 9)})
	it, _ := p.Thing(thing.CategoryItem, 100)
	it.FrameGroups[0].Resize(2, 2, 1, 1, 1, 1, 1, 0)
	copy(it.FrameGroups[0].Sprites, []uint32{1, 2, 3, 4})
	if err := p.UpdateThing(it); err != nil {
		t.Fatal(err)
	}
	o := CompileOptions{DatPath: dir + "/Tibia.dat", SprPath: dir + "/Tibia.spr", Version: v1098(), Features: p.Info().Features}
	if err := p.Compile(o); err != nil {
		t.Fatal(err)
	}
	q, err := Open(OpenOptions{DatPath: o.DatPath, SprPath: o.SprPath})
	if err != nil {
		t.Fatal(err)
	}
	res, _ := q.OptimizeSprites(OptimizeOptions{Duplicates: true, Empty: true, Unused: true}, nil)
	if res.After != 2 {
		t.Fatalf("%+v", res)
	}
	check := func(r *Project, want map[uint32][3]byte) {
		t.Helper()
		for id, w := range want {
			if px, err := r.SpritePixels(id); err != nil || [3]byte(px[:3]) != w {
				t.Fatalf("sprite %d %v %v", id, px[:4], err)
			}
		}
	}
	check(q, map[uint32][3]byte{1: {9, 0, 0}, 2: {0, 0, 9}})
	q.Undo()
	check(q, map[uint32][3]byte{1: {9, 0, 0}, 2: {9, 0, 0}, 4: {0, 0, 9}})
	q.Redo()
	check(q, map[uint32][3]byte{1: {9, 0, 0}, 2: {0, 0, 9}})

	o2 := o
	o2.DatPath, o2.SprPath = dir+"/Out.dat", dir+"/Out.spr"
	if err := q.Compile(o2); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(o.SprPath)
	after, _ := os.Stat(o2.SprPath)
	if saved := before.Size() - after.Size(); res.Bytes != saved {
		t.Fatalf("bytes %d, spr shrank by %d", res.Bytes, saved)
	}
	r, err := Open(OpenOptions{DatPath: o2.DatPath, SprPath: o2.SprPath})
	if err != nil {
		t.Fatal(err)
	}
	if r.Info().Counts.Sprites != 2 {
		t.Fatalf("compiled count %d", r.Info().Counts.Sprites)
	}
	check(r, map[uint32][3]byte{1: {9, 0, 0}, 2: {0, 0, 9}})
}
