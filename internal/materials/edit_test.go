package materials

import (
	"slices"
	"testing"
)

func TestSaveBorder(t *testing.T) {
	m := New()
	first := m.SaveBorder(Border{})
	second := m.SaveBorder(Border{})
	if first == 0 || second == first || len(m.Borders) != 2 {
		t.Fatalf("ids %d %d, borders %d", first, second, len(m.Borders))
	}
	if m.SaveBorder(Border{ID: first, Group: 5}) != first || m.Border(first).Group != 5 || len(m.Borders) != 2 {
		t.Fatal("border not replaced")
	}
}

func TestSaveBrush(t *testing.T) {
	m := New()
	if err := m.SaveBrush("", Brush{Name: "  "}, nil); err == nil {
		t.Fatal("brush without a name saved")
	}
	if err := m.SaveBrush("", Brush{Name: " grass ", Type: TypeGround}, []string{"Nature", " "}); err != nil {
		t.Fatal(err)
	}
	if m.Brush("grass") == nil || m.Tileset("Nature") == nil || !slices.Equal(m.Tileset("Nature").Terrain, []string{"grass"}) {
		t.Fatalf("brush or tileset missing: %+v", m.Tilesets)
	}
	if err := m.SaveBrush("", Brush{Name: "GRASS"}, nil); err == nil {
		t.Fatal("duplicate name saved")
	}
	// Rename and move to another tileset.
	if err := m.SaveBrush("grass", Brush{Name: "lawn", Type: TypeDoodad}, []string{"Garden"}); err != nil {
		t.Fatal(err)
	}
	if m.Brush("grass") != nil || m.Brush("lawn") == nil {
		t.Fatal("brush not renamed")
	}
	if len(m.Tileset("Nature").Terrain) != 0 || !slices.Equal(m.Tileset("Garden").Doodads, []string{"lawn"}) {
		t.Fatalf("tilesets not synced: %+v %+v", m.Tileset("Nature"), m.Tileset("Garden"))
	}
	// nil tilesets keep the collections.
	if err := m.SaveBrush("lawn", Brush{Name: "lawn", Type: TypeDoodad, Look: 7}, nil); err != nil {
		t.Fatal(err)
	}
	if m.Brush("lawn").Look != 7 || len(m.Tileset("Garden").Doodads) != 1 || len(m.Brushes) != 1 {
		t.Fatal("brush not replaced in place")
	}
}

func TestTilesets(t *testing.T) {
	m := New()
	if err := m.SaveTileset("", Tileset{Name: ""}); err == nil {
		t.Fatal("tileset without a name saved")
	}
	if err := m.SaveTileset("", Tileset{Name: "Walls", Auto: true}); err != nil {
		t.Fatal(err)
	}
	if m.Tileset("walls").Auto {
		t.Fatal("saved tileset is automatic")
	}
	if err := m.SaveTileset("", Tileset{Name: "walls"}); err == nil {
		t.Fatal("duplicate tileset saved")
	}
	if err := m.SaveTileset("Walls", Tileset{Name: "Stone walls"}); err != nil || m.Tileset("Stone walls") == nil || len(m.Tilesets) != 1 {
		t.Fatalf("rename: %v %+v", err, m.Tilesets)
	}
	m.AddRawItems("Stone walls", []uint16{10, 11})
	m.AddRawItems("Stone walls", []uint16{11, 12})
	m.AddRawItems("Other", []uint16{5})
	if !slices.Equal(m.Tileset("Stone walls").Raw, []uint16{10, 11, 12}) || m.Tileset("Other") == nil {
		t.Fatalf("raw items: %+v", m.Tilesets)
	}
	m.RemoveTileset("STONE WALLS")
	if len(m.Tilesets) != 1 || m.Tileset("Other") == nil {
		t.Fatal("tileset not removed")
	}
}

func TestSaveCreature(t *testing.T) {
	m := New()
	if err := m.SaveCreature(CreatureType{Name: " "}); err == nil {
		t.Fatal("creature without a name saved")
	}
	m.SaveCreature(CreatureType{Name: "Rat"})
	m.SaveCreature(CreatureType{Name: "rat", NPC: true})
	if len(m.Creatures) != 1 || !m.Creature("Rat").NPC {
		t.Fatalf("creatures = %+v", m.Creatures)
	}
}
