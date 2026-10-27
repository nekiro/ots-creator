package app

import (
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func TestSuggestRanksItemsOfTheGroundAndFollowsEdits(t *testing.T) {
	ms, _ := mapTestService(t)
	info, err := ms.NewMap(NewMapOptions{Width: 256, Height: 256})
	if err != nil {
		t.Fatal(err)
	}
	center := otmap.Position{X: 10, Y: 10, Z: 7}
	if _, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{center}, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if got, err := ms.Suggest(info.ID, center, SuggestBrush{}); err != nil || len(got.Here) != 0 {
		t.Fatalf("Suggest on bare ground = %+v, %v; want no suggestions", got, err)
	}
	for _, pos := range []otmap.Position{{X: 9, Y: 9, Z: 7}, {X: 11, Y: 9, Z: 7}} {
		if _, err := ms.AddItem(info.ID, pos, 101); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ms.Suggest(info.ID, center, SuggestBrush{Kind: "raw", Item: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Here) != 1 || got.Here[0].Item != 101 || !strings.HasPrefix(got.Here[0].Reason, "lies on") || got.Pending != 0 {
		t.Fatalf("Suggest after adding items = %+v, want item 101 on the ground", got)
	}
}

func TestSwapItemThroughService(t *testing.T) {
	ms, _ := mapTestService(t)
	info, err := ms.NewMap(NewMapOptions{Width: 256, Height: 256})
	if err != nil {
		t.Fatal(err)
	}
	pos := otmap.Position{X: 5, Y: 5, Z: 7}
	if _, err := ms.AddItem(info.ID, pos, 101); err != nil {
		t.Fatal(err)
	}
	if variants, err := ms.Variants(info.ID, 101); err != nil || len(variants) == 0 || variants[0] != 101 {
		t.Fatalf("Variants(101) = %v, %v; want 101 first", variants, err)
	}
	if _, err := ms.SwapItem(info.ID, mapedit.ItemRef{Pos: pos, Index: 0}, 102, 1); err != nil {
		t.Fatal(err)
	}
	if tile, _ := ms.Tile(info.ID, pos); len(tile.Items) != 1 || tile.Items[0].ID != 102 {
		t.Fatalf("tile after swap = %+v", tile)
	}
}

func TestMakeMixAndPlaceItemsThroughService(t *testing.T) {
	ms, _ := mapTestService(t)
	info, err := ms.NewMap(NewMapOptions{Width: 256, Height: 256})
	if err != nil {
		t.Fatal(err)
	}
	name, err := ms.MakeMix(info.ID, []MixItem{{Item: 101, Weight: 2}})
	if err != nil {
		t.Fatal(err)
	}
	pos := otmap.Position{X: 5, Y: 5, Z: 7}
	if _, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{pos}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Draw(info.ID, mapedit.DrawRequest{Kind: mapedit.KindDoodad, Name: name, Points: []otmap.Position{pos}}); err != nil {
		t.Fatalf("drawing the mix %q: %v", name, err)
	}
	if tile, _ := ms.Tile(info.ID, pos); len(tile.Items) != 1 || tile.Items[0].ID != 101 {
		t.Fatalf("tile after the mix = %+v, want item 101", tile)
	}
	next := otmap.Position{X: 6, Y: 5, Z: 7}
	if _, err := ms.PlaceItems(info.ID, []mapedit.PlacedItem{{Pos: next, Item: 101}}, 0); err != nil {
		t.Fatal(err)
	}
	if tile, _ := ms.Tile(info.ID, next); len(tile.Items) != 1 {
		t.Fatalf("tile after PlaceItems = %+v", tile)
	}
}

func TestAverageLookCountsOpaquePixels(t *testing.T) {
	// A 2x1 texture of 32px sprites: one red opaque pixel, the rest clear.
	pixels := make([]byte, 64*32*4)
	copy(pixels, []byte{200, 0, 0, 255})
	look := averageLook(pixels, 64, 32, 32)
	if !look.ok || look.width != 2 || look.height != 1 || look.red != 200 || look.green != 0 || look.cover != 1.0/(64*32) {
		t.Fatalf("averageLook = %+v", look)
	}
	if empty := averageLook(make([]byte, 32*32*4), 32, 32, 32); empty.ok {
		t.Fatal("averageLook(clear texture) is ok, want not ok")
	}
}
