package materials

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	parsed, err := Parse([]byte(`{"version":0,"brushes":[{"name":"grass","type":"ground"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version != FormatVersion || parsed.Brush("GRASS") == nil {
		t.Fatalf("parsed %+v", parsed)
	}
	if _, err := Parse([]byte(`{"version":99}`)); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("newer version: %v", err)
	}
	if _, err := Parse([]byte(`{`)); err == nil {
		t.Fatal("broken JSON must fail")
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "materials.json")
	source := New()
	source.Borders = []*Border{{ID: 3, Edges: map[string]uint16{"n": 10}}}
	if err := source.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.Border(3) == nil || loaded.Border(3).Edges["n"] != 10 {
		t.Fatalf("loaded %+v %v", loaded, err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Fatal("missing file must fail")
	}
	if err := source.Save(filepath.Join(t.TempDir(), "missing", "dir", "materials.json")); err == nil {
		t.Fatal("missing folder must fail")
	}
}

func TestParseRMECreatures(t *testing.T) {
	creatures, err := ParseRMECreatures([]byte(`<creatures>
		<creature name="Rat" type="monster" looktype="21" lookhead="1" lookbody="2" looklegs="3" lookfeet="4" lookaddon="3" lookmount="5"/>
		<creature name="Chest" type="NPC" lookitem="1740"/>
		<creature name="" looktype="1"/>
		<other name="x"/>
	</creatures>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(creatures) != 2 {
		t.Fatalf("creatures %+v", creatures)
	}
	rat := creatures[0]
	want := Outfit{LookType: 21, Head: 1, Body: 2, Legs: 3, Feet: 4, Addons: 3, Mount: 5}
	if rat.Name != "Rat" || rat.NPC || rat.Outfit != want {
		t.Fatalf("rat %+v", rat)
	}
	if chest := creatures[1]; !chest.NPC || chest.Outfit.LookItem != 1740 {
		t.Fatalf("chest %+v", chest)
	}
	if _, err := ParseRMECreatures([]byte("<creatures")); err == nil {
		t.Fatal("broken xml must fail")
	}
}

func TestLookups(t *testing.T) {
	materials := New()
	if materials.NextBorderID() != 1 {
		t.Fatal("first border id must be 1")
	}
	materials.Borders = []*Border{{ID: 4}, {ID: 9}, {ID: 2}}
	if materials.NextBorderID() != 10 {
		t.Fatalf("next border id %d", materials.NextBorderID())
	}
	materials.Creatures = []*CreatureType{{Name: "Rat"}, {Name: "Cave Rat"}}
	if creature := materials.Creature("cave rat"); creature == nil || creature.Name != "Cave Rat" {
		t.Fatalf("creature %+v", creature)
	}
	if materials.Creature("Dragon") != nil || materials.Border(5) != nil || materials.Tileset("none") != nil {
		t.Fatal("missing entries must be nil")
	}
}

func TestRenameAndRemoveBrush(t *testing.T) {
	materials := New()
	materials.Brushes = []*Brush{
		{Name: "grass", Type: TypeGround, Friends: []string{"Dirt"}, Borders: []BorderRule{{To: "dirt"}}},
		{Name: "dirt", Type: TypeGround},
		{Name: "wall", Type: TypeWall, Redirect: "DIRT"},
	}
	materials.Tilesets = []*Tileset{{Name: "Nature", Terrain: []string{"grass", "dirt"}, Doodads: []string{"Dirt"}}}
	materials.RenameBrush("dirt", "mud")
	grass := materials.Brush("grass")
	if materials.Brush("mud") == nil || grass.Friends[0] != "mud" || grass.Borders[0].To != "mud" || materials.Brush("wall").Redirect != "mud" {
		t.Fatalf("renamed %+v", materials.Brushes)
	}
	if tileset := materials.Tileset("nature"); tileset.Terrain[1] != "mud" || tileset.Doodads[0] != "mud" {
		t.Fatalf("tileset %+v", tileset)
	}
	materials.RemoveBrush("MUD")
	if materials.Brush("mud") != nil || len(materials.Brushes) != 2 {
		t.Fatalf("brushes %+v", materials.Brushes)
	}
	if tileset := materials.Tileset("Nature"); len(tileset.Terrain) != 1 || len(tileset.Doodads) != 0 {
		t.Fatalf("tileset %+v", tileset)
	}
}

func TestMerge(t *testing.T) {
	materials := New()
	materials.Borders = []*Border{{ID: 1}}
	materials.Brushes = []*Brush{{Name: "grass"}}
	materials.Tilesets = []*Tileset{{Name: "Nature", Terrain: []string{"grass"}, Items: []uint16{1}}}
	materials.Creatures = []*CreatureType{{Name: "Rat"}}
	materials.MetaItems = []uint16{80}

	other := New()
	other.Borders = []*Border{{ID: 1, Group: 7}, {ID: 5}}
	other.Brushes = []*Brush{
		{Name: "GRASS"},
		{Name: "sand", Borders: []BorderRule{{Border: 1}, {Border: 5}}, Optional: &BorderRule{Border: 1}},
	}
	other.Tilesets = []*Tileset{{Name: "Nature", Terrain: []string{"Grass", "sand"}, Doodads: []string{"tree"}, Items: []uint16{1, 2}, Raw: []uint16{3}}, {Name: "Caves"}}
	other.Creatures = []*CreatureType{{Name: "rat"}, {Name: "Bat"}}
	other.MetaItems = []uint16{80, 81}
	materials.Merge(other)

	// The clashing border 1 gets the next free id (2); border 5 is free.
	if len(materials.Borders) != 3 || materials.Border(2) == nil || materials.Border(2).Group != 7 || materials.Border(5) == nil {
		t.Fatalf("borders %+v", materials.Borders)
	}
	sand := materials.Brush("sand")
	if sand == nil || sand.Borders[0].Border != 2 || sand.Borders[1].Border != 5 || sand.Optional.Border != 2 || len(materials.Brushes) != 2 {
		t.Fatalf("sand %+v", sand)
	}
	nature := materials.Tileset("Nature")
	if strings.Join(nature.Terrain, ",") != "grass,sand" || len(nature.Doodads) != 1 || len(nature.Items) != 2 || len(nature.Raw) != 1 {
		t.Fatalf("nature %+v", nature)
	}
	if materials.Tileset("Caves") == nil || len(materials.Creatures) != 2 || len(materials.MetaItems) != 2 {
		t.Fatalf("merged %+v", materials)
	}
}

func TestFindRMEDataFolders(t *testing.T) {
	root := t.TempDir()
	materialsXML := map[string]string{"materials.xml": "<materials/>"}
	writeFiles(t, filepath.Join(root, "860"), materialsXML)
	writeFiles(t, filepath.Join(root, "10100"), materialsXML)
	writeFiles(t, filepath.Join(root, "notes"), materialsXML)
	writeFiles(t, filepath.Join(root, "999"), map[string]string{"readme.txt": ""})

	cases := []struct {
		version int
		want    string
	}{
		{1098, "10100"}, // 10100 compares as 1010
		{1000, "860"},
		{700, "860"}, // nothing older: the lowest
	}
	for _, testCase := range cases {
		found, err := FindRMEData(root, testCase.version)
		if err != nil || found != filepath.Join(root, testCase.want) {
			t.Errorf("version %d: %q %v", testCase.version, found, err)
		}
	}

	direct := t.TempDir()
	writeFiles(t, filepath.Join(direct, "data", "materials"), materialsXML)
	if found, err := FindRMEData(direct, 1098); err != nil || found != filepath.Join(direct, "data", "materials") {
		t.Fatalf("data/materials: %q %v", found, err)
	}
	if _, err := FindRMEData(t.TempDir(), 1098); err == nil {
		t.Fatal("empty folder must fail")
	}
	if _, err := FindRMEData(filepath.Join(root, "missing"), 1098); err == nil {
		t.Fatal("missing folder must fail")
	}
}

func TestImportRMEDetails(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"materials.xml": `<materials>
			<include file=""/>
			<include file="missing.xml"/>
			<include file="materials.xml"/>
			<border group="1"/>
			<brush type="ground"/>
			<brush name="odd" type="bogus"/>
			<brush name="sea" type="ground" lookid="5">
				<item id="4608" chance="10"/>
				<optional ground_equivalent="4608"><borderitem edge="n" item="20"/></optional>
				<border align="outer" to="beach" ground_equivalent="4608" super="true"><borderitem edge="s" item="21"/>
					<specific><conditions><match_group group="2" edge="e"/><match_item id="9"/></conditions>
					<actions><replace_item id="9" with="10"/><delete_borders/></actions></specific>
				</border>
				<enemy name="lava"/>
			</brush>
			<brush name="cleared" type="ground"><item id="1" chance="1"/><border id="3"/><friend name="x"/><clear_borders/><clear_friends/>
				<optional id="8"/></brush>
			<brush name="stone" type="wall">
				<wall type="northwest diagonal"><door id="1" type="any door" hate="true"/><door id="2" type="any"/><door id="3" type="bogus"/></wall>
				<wall type="East T"><item id="4" chance="1"/></wall>
				<wall type="nowhere"/>
				<friend name="wood" redirect="true"/>
			</brush>
			<brush name="arch" type="wall"><alternate><item id="7" chance="1"/></alternate></brush>
			<brush name="rug" type="carpet"><carpet align="bogus" id="1"/></brush>
			<brush name="desk" type="table"><table align="bogus"><item id="1"/></table></brush>
		</materials>`,
	})
	materials, warnings, err := ImportRME(dir, func(clientID uint16) (uint16, bool) { return clientID + 1000, true })
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"include missing.xml", "border without an id", "brush without a name", `"odd" has unknown type`,
		"unknown door type", "unknown alignment \"nowhere\"", "carpet \"rug\"", "table \"desk\""} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning %q in:\n%s", want, joined)
		}
	}
	sea := materials.Brush("sea")
	if sea.Look != 1005 || !sea.HateFriends || sea.Optional.Inline == nil || sea.Optional.Inline.GroundEquivalent != 4608 {
		t.Fatalf("sea %+v", sea)
	}
	rule := sea.Borders[0]
	if rule.To != "beach" || !rule.Super || rule.Inline == nil || len(rule.Specific) != 1 ||
		len(rule.Specific[0].Conditions) != 2 || len(rule.Specific[0].Actions) != 2 {
		t.Fatalf("rule %+v", rule)
	}
	if cleared := materials.Brush("cleared"); cleared.Borders != nil || cleared.Friends != nil || cleared.Optional.Border != 8 {
		t.Fatalf("cleared %+v", cleared)
	}
	stone := materials.Brush("stone")
	if corner := stone.Walls["corner"]; corner == nil || len(corner.Doors) != 5+len(DoorTypes) || !corner.Doors[0].Hate {
		t.Fatalf("corner %+v", stone.Walls)
	}
	if stone.Walls["east T"] == nil || stone.Redirect != "wood" {
		t.Fatalf("stone %+v", stone)
	}
	if arch := materials.Brush("arch"); len(arch.Alternatives) != 1 {
		t.Fatalf("arch %+v", arch)
	}
}

func TestWarningCap(t *testing.T) {
	importer := &importer{}
	for range maxImportWarnings + 5 {
		importer.warn("warning")
	}
	if len(importer.warnings) != maxImportWarnings {
		t.Fatalf("%d warnings", len(importer.warnings))
	}
}
