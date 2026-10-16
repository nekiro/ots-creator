package mapedit

import (
	"testing"

	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func TestClipDoodadBrush(t *testing.T) {
	clip := &Clip{Tiles: []ClipTile{
		{DX: 0, DY: 0, Ground: &otmap.Item{ID: 100}, Items: []otmap.Item{{ID: 200, Subtype: 3}}},
		{DX: 1, DY: 0},
		{DX: 1, DY: 1, Items: []otmap.Item{{ID: 300}}},
	}}
	brush, err := clip.DoodadBrush("rocks")
	if err != nil {
		t.Fatal(err)
	}
	if brush.Name != "rocks" || brush.Type != materials.TypeDoodad || brush.Look != 200 || !brush.Doodad.OneSize {
		t.Fatalf("brush = %+v", brush)
	}
	tiles := brush.Alternatives[0].Composites[0].Tiles
	if len(tiles) != 2 || len(tiles[0].Items) != 2 || tiles[0].Items[1].Subtype != 3 || tiles[1].X != 1 || tiles[1].Y != 1 {
		t.Fatalf("tiles = %+v", tiles)
	}
	if _, err := (&Clip{Tiles: []ClipTile{{}}}).DoodadBrush("empty"); err == nil {
		t.Fatal("empty clip made a brush")
	}
}
