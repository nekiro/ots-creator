package mapedit

import (
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/materials"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Item ids of the test materials.
const (
	grassA, grassB        = 100, 101
	sandID                = 110
	borderN, borderE      = 200, 201
	borderS, borderW      = 202, 203
	borderCNW, borderCNE  = 204, 205
	borderCSW, borderCSE  = 206, 207
	borderDNW, borderDNE  = 208, 209
	borderDSE, borderDSW  = 210, 211
	wallH, wallV, wallC   = 300, 301, 302
	wallPoleID            = 303
	doorHClosed, doorHOpn = 310, 311
	rugC, rugN            = 400, 401
	tableAloneID, tableH  = 500, 501
	tableWestID, tableEID = 502, 503
	treeID                = 600
	stoneID               = 700
	bagID                 = 800
	coinID                = 801
	foreignBorder         = 900 // a border item no material knows
)

func testEditor(t *testing.T) *Editor {
	t.Helper()
	var types []*otitems.Type
	add := func(id uint16, f otitems.Flags, top uint8) {
		types = append(types, &otitems.Type{ID: id, ClientID: id, Flags: f, TopOrder: top, Name: "item"})
	}
	for _, id := range []uint16{grassA, grassB, sandID} {
		add(id, otitems.FlagGround, 0)
	}
	for id := uint16(borderN); id <= borderDSW; id++ {
		add(id, 0, 1)
	}
	for _, id := range []uint16{wallH, wallV, wallC, wallPoleID, doorHClosed, doorHOpn} {
		add(id, otitems.FlagUnpassable, 2)
	}
	for _, id := range []uint16{rugC, rugN, tableAloneID, tableH, tableWestID, tableEID, treeID} {
		add(id, otitems.FlagUnpassable, 0)
	}
	add(foreignBorder, 0, 1)
	add(stoneID, otitems.FlagMoveable|otitems.FlagPickupable, 0)
	add(bagID, otitems.FlagContainer|otitems.FlagMoveable, 0)
	add(coinID, otitems.FlagStackable|otitems.FlagMoveable, 0)
	db := otitems.NewDB(otitems.ModeOTB, types)
	types[len(types)-2].Kind = otitems.KindContainer
	db.Get(doorHClosed).Kind = otitems.KindDoor
	db.Get(doorHOpn).Kind = otitems.KindDoor

	m := materials.New()
	m.Borders = []*materials.Border{{ID: 1, Edges: map[string]uint16{
		"n": borderN, "e": borderE, "s": borderS, "w": borderW, "cnw": borderCNW, "cne": borderCNE,
		"csw": borderCSW, "cse": borderCSE, "dnw": borderDNW, "dne": borderDNE, "dse": borderDSE, "dsw": borderDSW,
	}}}
	m.Brushes = []*materials.Brush{
		{Name: "grass", Type: materials.TypeGround, ZOrder: 10, Items: []materials.Chance{{ID: grassA, Chance: 10}, {ID: grassB, Chance: 10}},
			Borders: []materials.BorderRule{{Align: "outer", Border: 1}}},
		{Name: "sand", Type: materials.TypeGround, ZOrder: 5, Items: []materials.Chance{{ID: sandID, Chance: 1}}},
		{Name: "stone wall", Type: materials.TypeWall, Walls: map[string]*materials.WallPart{
			"horizontal": {Items: []materials.Chance{{ID: wallH, Chance: 1}}, Doors: []materials.Door{{ID: doorHClosed, Type: "normal"}, {ID: doorHOpn, Type: "normal", Open: true}}},
			"vertical":   {Items: []materials.Chance{{ID: wallV, Chance: 1}}},
			"corner":     {Items: []materials.Chance{{ID: wallC, Chance: 1}}},
			"pole":       {Items: []materials.Chance{{ID: wallPoleID, Chance: 1}}},
		}},
		{Name: "rug", Type: materials.TypeCarpet, Parts: map[string][]materials.Chance{"c": {{ID: rugC, Chance: 1}}, "n": {{ID: rugN, Chance: 1}}}},
		{Name: "desk", Type: materials.TypeTable, Parts: map[string][]materials.Chance{
			"alone": {{ID: tableAloneID, Chance: 1}}, "horizontal": {{ID: tableH, Chance: 1}},
			"west": {{ID: tableWestID, Chance: 1}}, "east": {{ID: tableEID, Chance: 1}},
		}},
		{Name: "trees", Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{Draggable: true, Thickness: 100, ThicknessCeiling: 100},
			Alternatives: []materials.Alternative{{Singles: []materials.Single{{Chance: 1, Item: materials.ItemSpec{ID: treeID}}}}}},
		// A stamp made from a selection: sand with a stone, and a lone tree.
		{Name: "pit", Type: materials.TypeDoodad, Doodad: &materials.DoodadOptions{OnBlocking: true, OneSize: true},
			Alternatives: []materials.Alternative{{Composites: []materials.Composite{{Chance: 1, Tiles: []materials.CompositeTile{
				{X: 0, Y: 0, Items: []materials.ItemSpec{{ID: sandID}, {ID: stoneID}, {ID: borderN}}},
				{X: 1, Y: 0, Items: []materials.ItemSpec{{ID: treeID}}},
			}}}}}},
	}
	mp := otmap.New(1000, 1000)
	e := New(mp, db, NewBrushes(db, m))
	e.Seed(1)
	return e
}

func at(x, y int) otmap.Position { return posAt(x, y, 7) }

func itemIDs(t *otmap.Tile) []uint16 {
	if t == nil {
		return nil
	}
	var ids []uint16
	for _, it := range t.Items {
		ids = append(ids, it.ID)
	}
	return ids
}

// rect is a rectangle on floor 7.
func rect(x0, y0, x1, y1 int) *Rect { return &Rect{From: at(x0, y0), To: at(x1, y1)} }

// points lists brush centers.
func points(positions ...otmap.Position) []otmap.Position { return positions }

// ptr returns a pointer to a copy of value (for patches).
func ptr[T any](value T) *T { return &value }

// errText is an error's message, or nil, for comparing results.
func errText(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}
