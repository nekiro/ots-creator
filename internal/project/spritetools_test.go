package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestFindSpritesAndReplaceRefs(t *testing.T) {
	p := New(v1098(), client.Features{})
	p.AddSprites([][]byte{solid(255, 0, 0), make([]byte, 32*32*4), solid(255, 0, 0), solid(0, 255, 0)})
	setItem(t, p, 100, false, 1, 3)
	setItem(t, p, 101, false, 3)

	for filter, want := range map[string][]uint32{
		SpritesUnused:    {2, 4},
		SpritesEmpty:     {2},
		SpritesDuplicate: {3},
	} {
		got, err := p.FindSprites(filter)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("%s: %v %v, want %v", filter, got, err, want)
		}
	}
	if _, err := p.FindSprites("bogus"); err == nil {
		t.Fatal("unknown filter accepted")
	}
	if users := p.SpriteUsers(3); len(users) != 2 {
		t.Fatalf("users %v", users)
	}

	n, err := p.ReplaceSpriteRefs([]uint32{3}, 1)
	if err != nil || n != 2 {
		t.Fatalf("replaced %d %v", n, err)
	}
	it, _ := p.Thing(thing.CategoryItem, 100)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{1, 1}) {
		t.Fatalf("sprites %v", it.FrameGroups[0].Sprites)
	}
	if _, err := p.ReplaceSpriteRefs([]uint32{1}, 99); err == nil {
		t.Fatal("missing target accepted")
	}
	p.Undo()
	if it, _ = p.Thing(thing.CategoryItem, 101); it.FrameGroups[0].Sprites[0] != 3 {
		t.Fatal("undo did not restore")
	}
}
