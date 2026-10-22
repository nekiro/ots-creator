package mapedit

import (
	"reflect"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func conversionEditor() (*Editor, *otitems.DB) {
	server := otitems.NewDB(otitems.ModeOTB, []*otitems.Type{
		{ID: 1000, ClientID: 101}, {ID: 101, ClientID: 100},
	})
	client := otitems.NewDB(otitems.ModeClient, []*otitems.Type{
		{ID: 100, ClientID: 100}, {ID: 101, ClientID: 101},
	})
	return New(otmap.New(512, 512), server, nil), client
}

func TestConvertWorkingMapToClientIDsAndUndo(t *testing.T) {
	e, client := conversionEditor()
	pos := otmap.Position{X: 10, Y: 10, Z: 7}
	original := &otmap.Tile{
		Ground: otmap.Item{ID: 1000}, HouseID: 42, Flags: otmap.FlagPZ,
		Items: []otmap.Item{{ID: 101, Selected: true, Ext: &otmap.ItemExt{
			ActionID: 1234, UniqueID: 4321, Text: "keep", Dest: &pos,
			Items: []otmap.Item{{ID: 1000, Subtype: 253}},
		}}},
		Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat"}}, Spawn: &otmap.Spawn{Radius: 2}},
	}
	e.Map.SetTile(pos, original.Clone())
	e.MarkSaved()
	// A prior unsaved edit must remain independently undoable.
	tx := e.Begin("Edit text", 0)
	tx.Edit(pos).Items[0].Ext.Text = "unsaved text"
	tx.Commit()
	before := e.Map.Tile(pos).Clone()
	count, err := e.ConvertToClientIDs(client, nil)
	if err != nil || count != 3 || !e.Changed() || e.Items != client {
		t.Fatalf("conversion: %d %v", count, err)
	}
	want := before.Clone()
	want.Ground.ID, want.Items[0].ID, want.Items[0].Ext.Items[0].ID = 101, 100, 101
	if !reflect.DeepEqual(e.Map.Tile(pos), want) {
		t.Fatal("attributes, selection or unsaved edits changed")
	}
	if _, err := e.ConvertToClientIDs(client, nil); err == nil {
		t.Fatal("converted twice")
	}
	if label := e.Undo(); label != "Convert to client IDs" || e.Items.Mode != otitems.ModeOTB || !reflect.DeepEqual(e.Map.Tile(pos), before) {
		t.Fatalf("undo did not restore IDs and data: %s", label)
	}
	if !e.Changed() {
		t.Fatal("conversion undo lost the prior unsaved edit")
	}
	e.Redo()
	if e.Items != client || !reflect.DeepEqual(e.Map.Tile(pos), want) {
		t.Fatal("redo did not restore client IDs")
	}
	e.Undo()
	e.Undo()
	if e.Changed() || !reflect.DeepEqual(e.Map.Tile(pos), original) {
		t.Fatal("history before conversion was lost")
	}
}

func TestConvertWorkingMapValidatesBeforeEditing(t *testing.T) {
	for _, missingClient := range []bool{false, true} {
		e, client := conversionEditor()
		if missingClient {
			client = otitems.NewDB(otitems.ModeClient, nil)
		}
		pos := otmap.Position{X: 1, Y: 1, Z: 7}
		tile := &otmap.Tile{Ground: otmap.Item{ID: 1000}, Items: []otmap.Item{{ID: 101, Ext: &otmap.ItemExt{Items: []otmap.Item{{ID: 999}}}}}}
		e.Map.SetTile(pos, tile.Clone())
		e.MarkSaved()
		count, err := e.ConvertToClientIDs(client, nil)
		if err == nil || count != 0 || e.Changed() || e.CanUndo() || e.Items.Mode != otitems.ModeOTB || !reflect.DeepEqual(e.Map.Tile(pos), tile) {
			t.Fatalf("failure left a partial conversion: %v", err)
		}
	}
}

func TestConvertEmptyMapChangesModeAndHistory(t *testing.T) {
	e, client := conversionEditor()
	count, err := e.ConvertToClientIDs(client, nil)
	if err != nil || count != 0 || !e.Changed() || e.Items.Mode != otitems.ModeClient {
		t.Fatalf("empty map conversion: %d %v", count, err)
	}
	e.Undo()
	if e.Changed() || e.Items.Mode != otitems.ModeOTB {
		t.Fatal("empty map conversion cannot be undone")
	}
}
