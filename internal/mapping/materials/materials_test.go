package materials

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/otitems"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportRME(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "1098")
	writeFiles(t, dir, map[string]string{
		"materials.xml": `<materials><metaitem id="80"/><include file="borders.xml"/><include file="brushes.xml"/></materials>`,
		"borders.xml": `<materials><border id="2" group="1">
			<borderitem edge="n" item="10"/><borderitem edge="cnw" item="14"/><borderitem edge="bogus" item="99"/>
		</border></materials>`,
		"brushes.xml": `<materials>
			<brush name="dirt" type="ground"/>
			<brush name="grass" type="ground" server_lookid="200" z-order="3500">
				<item id="200" chance="2500"/><item id="201" chance="10"/>
				<border align="outer" id="2"/>
				<border align="inner" to="none" id="2">
					<specific><conditions><match_border id="2" edge="n"/></conditions>
					<actions><replace_border id="2" edge="n" with="77"/></actions></specific>
				</border>
				<friend name="dirt"/>
			</brush>
			<brush name="stone wall" type="wall" lookid="1060">
				<wall type="horizontal"><item id="300" chance="10"/><door id="310" type="normal" open="false"/><door id="311" type="any window"/></wall>
				<wall type="south T"><item id="305" chance="10"/></wall>
			</brush>
			<brush name="trees" type="doodad" server_lookid="400" draggable="true" thickness="12/100">
				<item id="400" chance="10"/>
				<composite chance="5"><tile x="0" y="0"><item id="401"/></tile><tile x="-1" y="0"><item id="402" aid="100"/></tile></composite>
			</brush>
			<brush name="rug" type="carpet" server_lookid="500"><carpet align="n" id="501"/><carpet align="center"><item id="500" chance="5"/></carpet></brush>
			<brush name="log" type="table" server_lookid="600"><table align="north"><item id="601" chance="10"/></table></brush>
			<tileset name="Nature"><terrain><brush name="grass"/></terrain><doodad><brush name="trees"/></doodad>
				<items_and_raw><item fromid="700" toid="702"/></items_and_raw><raw><item id="800"/></raw></tileset>
		</materials>`,
		"creatures.xml": `<creatures><creature name="Rat" type="monster" looktype="21"/><creature name="Bob" type="npc" looktype="128" lookhead="10"/></creatures>`,
	})
	found, err := FindRMEData(root, 1100)
	if err != nil || found != dir {
		t.Fatalf("FindRMEData = %q %v", found, err)
	}
	m, warns, err := ImportRME(dir, func(c uint16) (uint16, bool) {
		if c == 1060 {
			return 300, true
		}
		return 0, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if m.Brush("dirt") != nil {
		t.Fatal("undefined forward declaration kept")
	}
	if b := m.Border(2); b == nil || b.Group != 1 || b.Edges["n"] != 10 || b.Edges["cnw"] != 14 || len(b.Edges) != 2 {
		t.Fatalf("border = %+v", b)
	}
	g := m.Brush("grass")
	if g == nil || g.Type != TypeGround || g.ZOrder != 3500 || len(g.Items) != 2 || len(g.Borders) != 2 || g.Look != 200 {
		t.Fatalf("grass = %+v", g)
	}
	if r := g.Borders[1]; r.Align != "inner" || r.To != "none" || len(r.Specific) != 1 || r.Specific[0].Actions[0].With != 77 {
		t.Fatalf("inner rule = %+v", r)
	}
	w := m.Brush("stone wall")
	if w.Look != 300 || len(w.Walls["horizontal"].Doors) != 3 || w.Walls["south T"] == nil {
		t.Fatalf("wall = %+v", w)
	}
	d := m.Brush("trees")
	if d.Doodad == nil || !d.Doodad.Draggable || d.Doodad.Thickness != 12 || len(d.Alternatives) != 1 ||
		len(d.Alternatives[0].Composites) != 1 || d.Alternatives[0].Composites[0].Tiles[1].Items[0].ActionID != 100 {
		t.Fatalf("doodad = %+v", d)
	}
	if c := m.Brush("rug"); len(c.Parts["n"]) != 1 || len(c.Parts["c"]) != 1 {
		t.Fatalf("carpet = %+v", c.Parts)
	}
	if tb := m.Brush("log"); len(tb.Parts["north"]) != 1 {
		t.Fatalf("table = %+v", tb.Parts)
	}
	ts := m.Tileset("nature")
	if ts == nil || len(ts.Terrain) != 1 || len(ts.Doodads) != 1 || len(ts.Items) != 3 || len(ts.Raw) != 4 {
		t.Fatalf("tileset = %+v", ts)
	}
	if len(m.Creatures) != 2 || !m.Creatures[1].NPC || m.Creatures[1].Outfit.Head != 10 {
		t.Fatalf("creatures = %+v", m.Creatures)
	}
	if len(m.MetaItems) != 1 {
		t.Fatal("meta items lost")
	}

	// JSON round trip.
	p := filepath.Join(t.TempDir(), "m.json")
	if err := m.Save(p); err != nil {
		t.Fatal(err)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Brushes) != len(m.Brushes) || back.Brush("grass").Borders[1].Specific[0].Conditions[0].Edge != "n" {
		t.Fatal("round trip lost data")
	}

	// Merge renumbers clashing border ids.
	other := New()
	other.Borders = []*Border{{ID: 2, Edges: map[string]uint16{"n": 50}}}
	other.Brushes = []*Brush{{Name: "sand", Type: TypeGround, Items: []Chance{{ID: 900, Chance: 1}}, Borders: []BorderRule{{Align: "outer", Border: 2}}}}
	m.Merge(other)
	if s := m.Brush("sand"); s == nil || s.Borders[0].Border == 2 || m.Border(s.Borders[0].Border).Edges["n"] != 50 {
		t.Fatal("merge did not renumber")
	}
	m.RenameBrush("grass", "lawn")
	if m.Brush("lawn") == nil || m.Tileset("Nature").Terrain[0] != "lawn" {
		t.Fatal("rename")
	}
	m.RemoveBrush("lawn")
	if m.Brush("lawn") != nil || len(m.Tileset("Nature").Terrain) != 0 {
		t.Fatal("remove")
	}
}

func TestScanCreatures(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"monster/rat.xml":         `<?xml version="1.0"?><monster name="Rat" race="blood"><look type="21" corpse="5964"/></monster>`,
		"npc/bob.xml":             `<npc name="Bob"><look type="128" head="20" body="30" legs="40" feet="50" addons="3"/></npc>`,
		"monster/dragon.lua":      "local mType = Game.createMonsterType(\"Dragon\")\nlocal monster = {}\nmonster.outfit = {\n\tlookType = 34,\n\tlookHead = 0,\n\tlookMount = 7\n}\n",
		"npc/sam.lua":             "local internalNpcName = \"Sam\"\nlocal npcType = Game.createNpcType(internalNpcName)\nlocal npcConfig = {}\nnpcConfig.outfit = { lookType = 131, lookBody = 5 }\n",
		"monster/monsters.xml":    `<monsters><monster name="Rat" file="rat.xml"/><monster name="cave rat" file="rat.xml"/><monster name="Ghost" file="missing.xml"/></monsters>`,
		"scripts/other.lua":       "print('hi')",
		"items/items.xml":         `<items/>`,
		"monster/item_golem.xml":  `<monster name="Golem"><look typeex="1234"/></monster>`,
		"world/ignored/rat2.xml":  `<monster name="Hidden"><look type="1"/></monster>`,
		"monster/broken_file.xml": `<monster name="Broken"`,
	})
	got, err := ScanCreatures(dir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*CreatureType{}
	for _, c := range got {
		byName[c.Name] = c
	}
	if len(got) != 6 {
		names := []string{}
		for _, c := range got {
			names = append(names, c.Name)
		}
		t.Fatalf("found %v", names)
	}
	if r := byName["Rat"]; r.NPC || r.Outfit.LookType != 21 {
		t.Fatalf("rat = %+v", r)
	}
	if b := byName["Bob"]; !b.NPC || b.Outfit.Feet != 50 || b.Outfit.Addons != 3 {
		t.Fatalf("bob = %+v", b)
	}
	if d := byName["Dragon"]; d.Outfit.LookType != 34 || d.Outfit.Mount != 7 {
		t.Fatalf("dragon = %+v", d)
	}
	if s := byName["Sam"]; !s.NPC || s.Outfit.LookType != 131 || s.Outfit.Body != 5 {
		t.Fatalf("sam = %+v", s)
	}
	if g := byName["Golem"]; g.Outfit.LookItem != 1234 {
		t.Fatalf("golem = %+v", g)
	}
	// The registry names a creature the file calls otherwise.
	if c := byName["cave rat"]; c == nil || c.Outfit.LookType != 21 {
		t.Fatalf("cave rat = %+v", c)
	}
}

func TestAutoTilesets(t *testing.T) {
	db := otitems.NewDB(otitems.ModeClient, []*otitems.Type{
		{ID: 100, ClientID: 100, Flags: otitems.FlagGround},
		{ID: 101, ClientID: 101, TopOrder: 1},
		{ID: 102, ClientID: 102, Flags: otitems.FlagPickupable},
		{ID: 103, ClientID: 103, Flags: otitems.FlagMeta},
	})
	sets := AutoTilesets(db)
	names := map[string]int{}
	for _, s := range sets {
		names[s.Name] = len(s.Raw)
	}
	if names["Grounds"] != 1 || names["Borders"] != 1 || names["Pickupable items"] != 1 || len(sets) != 3 {
		t.Fatalf("sets = %v", names)
	}
}
