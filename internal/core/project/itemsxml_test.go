package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/itemsxml"
)

const xmlClientIDs = `<?xml version="1.0" encoding="UTF-8"?>
<items>
	<!-- kept -->
	<item id="100" name="void" />
	<item fromid="101" toid="103" name="earth" />
	<item id="104" article="a" name="sword">
		<attribute key="weight" value="3500" />
	</item>
	<item id="105" name="last" />
</items>
`

// attachXML writes an items.xml next to the client and attaches it.
func attachXML(t *testing.T, project *Project, text string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(project.Info().DatPath), "items.xml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.AttachXML(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// xmlProject compiles a client with items 100..105 and attaches an
// items.xml on client ids.
func xmlProject(t *testing.T) (*Project, string) {
	t.Helper()
	project := reorderProject(t)
	dir := t.TempDir()
	if err := project.Compile(CompileOptions{DatPath: filepath.Join(dir, "Tibia.dat"), SprPath: filepath.Join(dir, "Tibia.spr"), Version: v1098(), Features: project.Info().Features}); err != nil {
		t.Fatal(err)
	}
	return project, attachXML(t, project, xmlClientIDs)
}

func (p *Project) compileXML(t *testing.T) *XMLReport {
	t.Helper()
	in := p.Info()
	var st CompileStats
	if err := p.Compile(CompileOptions{DatPath: in.DatPath, SprPath: in.SprPath, Version: in.Version, Features: in.Features, Stats: &st}); err != nil {
		t.Fatal(err)
	}
	if st.XML == nil {
		t.Fatal("no items.xml report")
	}
	if st.XML.Error != "" {
		t.Fatal(st.XML.Error)
	}
	return st.XML
}

func readXML(t *testing.T, path string) (*itemsxml.File, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := itemsxml.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return file, string(data)
}

func xmlEntry(t *testing.T, project *Project, id uint32) XMLItem {
	t.Helper()
	entry, err := project.ItemXML(id)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestXMLEditAndUndo(t *testing.T) {
	project, path := xmlProject(t)
	if info := project.Info().ItemsXML; info == nil || info.Items != 6 || info.ServerIDs {
		t.Fatalf("info %+v", info)
	}
	sword := xmlEntry(t, project, 104)
	if sword.ID != 104 || !sword.Exists || sword.Item.Name != "sword" || len(sword.Item.Attributes) != 1 {
		t.Fatalf("sword %+v", sword)
	}
	// Setting what the file has is no edit.
	if err := project.SetItemXML(104, sword.Item); err != nil {
		t.Fatal(err)
	}
	if project.Info().Changed {
		t.Fatal("nothing changed")
	}
	sword.Item.Name = "long sword"
	sword.Item.SetAttribute("attack", "17")
	if err := project.SetItemXML(104, sword.Item); err != nil {
		t.Fatal(err)
	}
	if !project.Info().Changed || xmlEntry(t, project, 104).Item.Name != "long sword" {
		t.Fatal("the edit is not shown")
	}
	if names := project.Names(thing.CategoryItem); names[104] != "long sword" || names[102] != "earth" {
		t.Fatalf("names %v", names)
	}
	project.Undo()
	if xmlEntry(t, project, 104).Item.Name != "sword" || project.Info().Changed {
		t.Fatal("undo keeps the edit")
	}
	project.Redo()
	before, _ := os.ReadFile(path)
	if report, _ := project.XMLPreview(); report.Edited != 1 || report.Added != 0 {
		t.Fatalf("preview %+v", report)
	}
	if now, _ := os.ReadFile(path); string(now) != string(before) {
		t.Fatal("the file changed before compiling")
	}
	if report := project.compileXML(t); !report.Written || report.Edited != 1 {
		t.Fatalf("report %+v", report)
	}
	_, text := readXML(t, path)
	want := strings.Replace(xmlClientIDs, `<item id="104" article="a" name="sword">
		<attribute key="weight" value="3500" />`, `<item id="104" article="a" name="long sword">
		<attribute key="weight" value="3500" />
		<attribute key="attack" value="17" />`, 1)
	if text != want {
		t.Fatalf("written:\n%s", text)
	}
	// Compiling again writes nothing.
	if report := project.compileXML(t); report.Written || report.Changed() {
		t.Fatalf("second report %+v", report)
	}
}

func TestXMLNewItemsAndMapping(t *testing.T) {
	project, path := xmlProject(t)
	if err := project.SetXMLMapping([]XMLMapping{{Client: "nope", Key: "x"}}); err == nil {
		t.Fatal("an unknown property is an error")
	}
	mapping := []XMLMapping{{Client: "pickupable", Key: "pickupable"}, {Client: "groundSpeed", Key: "speed"}, {Client: "unpassable", Key: "blocking", Value: "true"}, {Client: "unmoveable", Key: "moveable", Not: true}, {Client: "hasElevation", Key: "hasheight", Aliases: []string{"Weight", " "}}}
	if err := project.SetXMLMapping(mapping); err != nil {
		t.Fatal(err)
	}
	mappingPath := filepath.Join(filepath.Dir(path), "items.attributes.json")
	if info := project.Info().ItemsXML; len(info.Mapping) != 5 || len(info.Mapping[4].Aliases) != 1 || info.MappingPath != mappingPath || !fileExists(mappingPath) {
		t.Fatalf("mapping %+v", info)
	}
	// A new item, and changed properties of 104 (has an entry) and 102 (in
	// a range).
	setItem(t, project, 106, true, 1)
	added, _ := project.Thing(thing.CategoryItem, 106)
	added.Props.Ground, added.Props.GroundSpeed = true, 150
	if err := project.UpdateThing(added); err != nil {
		t.Fatal(err)
	}
	setItem(t, project, 104, true, 5)
	blocking, _ := project.Thing(thing.CategoryItem, 102)
	blocking.Props.Unpassable, blocking.Props.Unmoveable = true, true
	if err := project.UpdateThing(blocking); err != nil {
		t.Fatal(err)
	}
	if entry := xmlEntry(t, project, 106); entry.Exists || entry.ID != 106 {
		t.Fatalf("new entry %+v", entry)
	}
	report := project.compileXML(t)
	if !report.Written || report.Added != 1 || report.Updated != 2 || report.Edited != 0 {
		t.Fatalf("report %+v", report)
	}
	_, text := readXML(t, path)
	want := `<?xml version="1.0" encoding="UTF-8"?>
<items>
	<!-- kept -->
	<item id="100" name="void" />
	<item id="101" name="earth" />
	<item id="102" name="earth">
		<attribute key="blocking" value="true" />
	</item>
	<item id="103" name="earth" />
	<item id="104" article="a" name="sword">
		<attribute key="weight" value="3500" />
		<attribute key="pickupable" value="1" />
	</item>
	<item id="105" name="last" />
	<item id="106" name="">
		<attribute key="pickupable" value="1" />
		<attribute key="speed" value="150" />
		<attribute key="moveable" value="1" />
	</item>
</items>
`
	if text != want {
		t.Fatalf("written:\n%s", text)
	}
	// A flag that is turned off takes its attribute away.
	setItem(t, project, 104, false, 5)
	if report := project.compileXML(t); report.Updated != 1 {
		t.Fatalf("report %+v", report)
	}
	file, _ := readXML(t, path)
	if sword, _ := file.Get(104); len(sword.Attributes) != 1 || sword.Attributes[0].Key != "weight" {
		t.Fatalf("sword %+v", sword)
	}
	// An alias of a key is the attribute: it gets the value, and is the
	// one removed.
	raised, _ := project.Thing(thing.CategoryItem, 104)
	raised.Props.HasElevation = true
	if err := project.UpdateThing(raised); err != nil {
		t.Fatal(err)
	}
	project.compileXML(t)
	file, _ = readXML(t, path)
	if sword, _ := file.Get(104); len(sword.Attributes) != 1 || sword.Attributes[0].Key != "weight" || sword.Attributes[0].Value != "1" {
		t.Fatalf("aliased %+v", sword)
	}
	raised.Props.HasElevation = false
	if err := project.UpdateThing(raised); err != nil {
		t.Fatal(err)
	}
	project.compileXML(t)
	file, _ = readXML(t, path)
	if sword, _ := file.Get(104); len(sword.Attributes) != 0 {
		t.Fatalf("alias kept %+v", sword)
	}
	// An empty map removes its file.
	if err := project.SetXMLMapping(nil); err != nil || fileExists(mappingPath) {
		t.Fatalf("mapping file kept: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestXMLFollowsMoves(t *testing.T) {
	project, path := xmlProject(t)
	// 104 (the sword) and 100 swap places; the edit of the sword follows.
	if _, err := project.MoveThings(thing.CategoryItem, []uint32{104}, 100, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	sword := xmlEntry(t, project, 100)
	if sword.Item.Name != "sword" || sword.ID != 100 {
		t.Fatalf("moved %+v", sword)
	}
	sword.Item.Name = "moved sword"
	if err := project.SetItemXML(100, sword.Item); err != nil {
		t.Fatal(err)
	}
	if report := project.compileXML(t); report.Moved != 2 || report.Edited != 1 {
		t.Fatalf("report %+v", report)
	}
	file, text := readXML(t, path)
	for id, name := range map[uint16]string{100: "moved sword", 101: "earth", 103: "earth", 104: "void", 105: "last"} {
		if item, ok := file.Get(id); !ok || item.Name != name {
			t.Fatalf("%d: %+v\n%s", id, item, text)
		}
	}
	if xmlEntry(t, project, 100).Item.Name != "moved sword" || project.Info().Changed {
		t.Fatal("the compiled file is not shown")
	}
}

func TestXMLServerIDs(t *testing.T) {
	project, _ := otbProject(t)
	path := attachXML(t, project, `<items>
	<item id="1100" name="void" />
	<item id="1104" name="sword" />
</items>
`)
	if !project.Info().ItemsXML.ServerIDs {
		t.Fatal("ids are server ids with an items.otb")
	}
	if sword := xmlEntry(t, project, 104); sword.ID != 1104 || sword.Item.Name != "sword" {
		t.Fatalf("sword %+v", sword)
	}
	// A moved item keeps its server id, so its entry stays.
	if _, err := project.MoveThings(thing.CategoryItem, []uint32{104}, 100, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	if sword := xmlEntry(t, project, 100); sword.ID != 1104 || sword.Item.Name != "sword" {
		t.Fatalf("moved sword %+v", sword)
	}
	setItem(t, project, 106, false, 1)
	added := xmlEntry(t, project, 106)
	if added.ID != 1201 || added.Exists {
		t.Fatalf("new %+v", added)
	}
	added.Item.Name = "brand new"
	if err := project.SetItemXML(106, added.Item); err != nil {
		t.Fatal(err)
	}
	unnamed := xmlEntry(t, project, 101)
	unnamed.Item.Name = "earth"
	if err := project.SetItemXML(101, unnamed.Item); err != nil {
		t.Fatal(err)
	}
	report := project.compileXML(t)
	if report.Added != 1 || report.Edited != 1 || report.Moved != 0 {
		t.Fatalf("report %+v", report)
	}
	_, text := readXML(t, path)
	want := `<items>
	<item id="1100" name="void" />
	<item id="1101" name="earth" />
	<item id="1104" name="sword" />
	<item id="1201" name="brand new" />
</items>
`
	if text != want {
		t.Fatalf("written:\n%s", text)
	}
}

func TestXMLReadAgainWhenChangedOnDisk(t *testing.T) {
	project, path := xmlProject(t)
	changed := strings.Replace(xmlClientIDs, `name="last"`, `name="edited outside"`, 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if entry := xmlEntry(t, project, 105); entry.Item.Name != "edited outside" {
		t.Fatalf("entry %+v", entry)
	}
	project.DetachXML()
	if _, err := project.ItemXML(105); err == nil || project.Info().ItemsXML != nil {
		t.Fatal("detached")
	}
}
