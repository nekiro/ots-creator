package project

import (
	"slices"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

func TestFindSpritesAndReplaceRefs(t *testing.T) {
	project := New(v1098(), client.Features{})
	project.AddSprites([][]byte{solid(255, 0, 0), make([]byte, 32*32*4), solid(255, 0, 0), solid(0, 255, 0)})
	setItem(t, project, 100, false, 1, 3)
	setItem(t, project, 101, false, 3)

	for filter, want := range map[string][]uint32{
		SpritesUnused:    {2, 4},
		SpritesEmpty:     {2},
		SpritesDuplicate: {3},
	} {
		got, err := project.FindSprites(filter)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("%s: %v %v, want %v", filter, got, err, want)
		}
	}
	if _, err := project.FindSprites("bogus"); err == nil {
		t.Fatal("unknown filter accepted")
	}
	if users := project.SpriteUsers(3); len(users) != 2 {
		t.Fatalf("users %v", users)
	}

	changed, err := project.ReplaceSpriteRefs([]uint32{3}, 1)
	if err != nil || changed != 2 {
		t.Fatalf("replaced %d %v", changed, err)
	}
	it, _ := project.Thing(thing.CategoryItem, 100)
	if !slices.Equal(it.FrameGroups[0].Sprites, []uint32{1, 1}) {
		t.Fatalf("sprites %v", it.FrameGroups[0].Sprites)
	}
	if _, err := project.ReplaceSpriteRefs([]uint32{1}, 99); err == nil {
		t.Fatal("missing target accepted")
	}
	project.Undo()
	if it, _ = project.Thing(thing.CategoryItem, 101); it.FrameGroups[0].Sprites[0] != 3 {
		t.Fatal("undo did not restore")
	}
}
