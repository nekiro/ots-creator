package itemsxml

import (
	"errors"
	"strings"
	"testing"
)

const sample = `<?xml version="1.0" encoding="ISO-8859-1"?>
<items>
	<!-- grounds -->
	<item id="100" name="void" />
	<item fromid="101" toid="104" name="earth" />
	<item id="2160" article="a" name="crystal coin" plural="crystal coins" editorsuffix=" (x)">
		<attribute key="weight" value="10" />
		<attribute key="field" value="fire">
			<attribute key="damage" value="20" />
		</attribute>
	</item>
</items>
`

func parse(t *testing.T, text string) *File {
	t.Helper()
	file, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestParse(t *testing.T) {
	file := parse(t, sample)
	if file.Items() != 6 || file.MaxID() != 2160 {
		t.Fatalf("items %d, max %d", file.Items(), file.MaxID())
	}
	earth, ok := file.Get(103)
	if !ok || earth.Name != "earth" {
		t.Fatalf("103: %+v", earth)
	}
	coin, _ := file.Get(2160)
	want := &Item{Name: "crystal coin", Article: "a", Plural: "crystal coins", Fields: []Field{{"editorsuffix", " (x)"}},
		Attributes: []Attribute{{Key: "weight", Value: "10"}, {Key: "field", Value: "fire", Children: []Attribute{{Key: "damage", Value: "20"}}}}}
	if !coin.Equal(want) {
		t.Fatalf("coin: %+v", coin)
	}
	if _, ok := file.Get(105); ok {
		t.Fatal("105 has no entry")
	}
	if _, err := Parse([]byte(`<other/>`)); !errors.Is(err, ErrNoItems) {
		t.Fatalf("err %v", err)
	}
	if _, err := Parse([]byte(`<items><item id="1"`)); err == nil {
		t.Fatal("a broken file is an error")
	}
}

func TestEncodeNothingKeepsBytes(t *testing.T) {
	file := parse(t, sample)
	if got := string(file.Encode(Changes{})); got != sample {
		t.Fatalf("changed:\n%s", got)
	}
}

func TestEncodeEditsAndAdds(t *testing.T) {
	file := parse(t, sample)
	coin, _ := file.Get(2160)
	coin.Name = `gold & "silver"`
	coin.SetAttribute("Weight", "25")
	got := string(file.Encode(Changes{Items: map[uint16]*Item{
		2160: coin,
		102:  {Name: "mud"},
		2161: {Name: "new item", Attributes: []Attribute{{Key: "weight", Value: "1"}}},
		50:   {Name: "first"},
	}}))
	want := `<?xml version="1.0" encoding="ISO-8859-1"?>
<items>
	<!-- grounds -->
	<item id="50" name="first" />
	<item id="100" name="void" />
	<item id="101" name="earth" />
	<item id="102" name="mud" />
	<item fromid="103" toid="104" name="earth" />
	<item id="2160" article="a" name="gold &amp; &quot;silver&quot;" plural="crystal coins" editorsuffix=" (x)">
		<attribute key="weight" value="25" />
		<attribute key="field" value="fire">
			<attribute key="damage" value="20" />
		</attribute>
	</item>
	<item id="2161" name="new item">
		<attribute key="weight" value="1" />
	</item>
</items>
`
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
	// What was written reads back.
	again := parse(t, got)
	if item, _ := again.Get(2160); !item.Equal(coin) {
		t.Fatalf("read back %+v", item)
	}
}

func TestEncodeFirstItemOfEmptyFile(t *testing.T) {
	file := parse(t, "<items>\r\n</items>\r\n")
	got := string(file.Encode(Changes{Items: map[uint16]*Item{7: {Name: "seven"}, 5: {Name: "five"}}}))
	want := "<items>\r\n\t<item id=\"5\" name=\"five\" />\r\n\t<item id=\"7\" name=\"seven\" />\r\n</items>\r\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestEncodeMoves(t *testing.T) {
	file := parse(t, sample)
	// 102 and 2160 swap, 100 moves onto 104 (whose item is gone).
	got := string(file.Encode(Changes{Moves: map[uint16]uint16{102: 2160, 2160: 102, 100: 104}}))
	again := parse(t, got)
	for id, name := range map[uint16]string{101: "earth", 102: "crystal coin", 103: "earth", 104: "void", 2160: "earth"} {
		item, ok := again.Get(id)
		if !ok || item.Name != name {
			t.Fatalf("%d: %+v\n%s", id, item, got)
		}
	}
	if again.Has(100) || again.Items() != 5 {
		t.Fatalf("items %d\n%s", again.Items(), got)
	}
	if !strings.Contains(got, "<!-- grounds -->") {
		t.Fatal("comment lost")
	}
}

func TestLatin1(t *testing.T) {
	file := parse(t, "<items>\n\t<item id=\"1\" name=\"caf\xe9\" />\n\t<item id=\"2\" name=\"x\" />\n</items>\n")
	item, _ := file.Get(1)
	if item.Name != "café" {
		t.Fatalf("name %q", item.Name)
	}
	got := string(file.Encode(Changes{Items: map[uint16]*Item{2: {Name: "thé"}}}))
	if !strings.Contains(got, "name=\"caf\xe9\"") || !strings.Contains(got, "name=\"th\xe9\"") {
		t.Fatalf("got %q", got)
	}
}
