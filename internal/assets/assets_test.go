package assets

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nekiro/ots-creator/internal/thing"
)

func TestSheetRoundTrip(t *testing.T) {
	px := make([]byte, sheetPixels)
	for i := 0; i < len(px); i += 4 {
		if (i/4)%7 == 0 {
			px[i], px[i+1], px[i+2], px[i+3] = byte(i), byte(i>>8), 0x40, 0xFF
		}
	}
	data, err := EncodeSheet(px)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data[:cipHeaderSize], cipMagic) {
		t.Fatalf("header % x", data[:cipHeaderSize])
	}
	back, err := DecodeSheet(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, px) {
		t.Fatal("pixels differ after round trip")
	}
}

func TestCatalog(t *testing.T) {
	src := `[
{"type":"appearances","file":"appearances-a.dat"},
{"type":"staticdata","file":"staticdata-b.dat"},
{"type":"sprite","file":"s2.bmp.lzma","spritetype":3,"firstspriteid":144,"lastspriteid":179,"area":0},
{"type":"sprite","file":"s1.bmp.lzma","spritetype":0,"firstspriteid":0,"lastspriteid":143,"area":0}
]`
	c, err := ParseCatalog([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if c.Appearances != "appearances-a.dat" || len(c.Sheets) != 2 || c.Sheets[0].File != "s1.bmp.lzma" || c.NextSpriteID() != 180 {
		t.Fatalf("%+v", c)
	}
	out, err := c.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := ParseCatalog(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Files()) != 4 || c2.Files()[1] != "staticdata-b.dat" {
		t.Fatalf("%v", c2.Files())
	}

	m := NewPieceMap(c)
	if m.Count() != 144+36*4 {
		t.Fatalf("count %d", m.Count())
	}
	first, w, h, ok := m.Pieces(145)
	if !ok || w != 2 || h != 2 || first != 145+4 {
		t.Fatalf("pieces %d %d %d %v", first, w, h, ok)
	}
	p, ok := m.Locate(first + 1) // tile (1,0): left of the bottom-right one
	if !ok || p.Sprite != 145 || p.Sheet != 1 || p.X != 64 || p.Y != 32 {
		t.Fatalf("%+v", p)
	}
	if id, ok := m.Sprite([]uint32{first, first + 1, first + 2, first + 3}, 2, 2); !ok || id != 145 {
		t.Fatal("sprite lookup")
	}
	if _, ok := m.Sprite([]uint32{first, first + 2, first + 1, first + 3}, 2, 2); ok {
		t.Fatal("shuffled pieces must not match")
	}
}

// fakeMap: sprites below 100 are 1x1, others 2x2.
type fakeMap struct{}

func (fakeMap) Pieces(id uint32) (uint32, int, int, bool) {
	if id < 100 {
		return id + 1, 1, 1, true
	}
	return 1000 + (id-100)*4, 2, 2, true
}

func TestAppearanceRoundTrip(t *testing.T) {
	flags := putBool(nil, 13, true)
	flags = putBytes(flags, fLight, putUint(putUint(nil, 1, 3), 2, 215))
	flags = putBytes(flags, fShift, putInt(putInt(nil, 1, -8), 2, -8))
	flags = putBytes(flags, fMarket, putBytes(putUint(putUint(nil, 1, 4), 5, 2), 7, []byte("bag")))
	flags = putUint(flags, 45, 1) // ammo: unknown to the model
	anim := putUint(nil, 2, 1)
	anim = putInt(anim, 4, -1)
	anim = putBytes(anim, 6, putUint(putUint(nil, 1, 100), 2, 200))
	anim = putBytes(anim, 6, putUint(putUint(nil, 1, 150), 2, 150))
	si := putUint(nil, 1, 1)
	si = putUint(si, 2, 1)
	si = putUint(si, 3, 1)
	si = putUint(si, 4, 1)
	si = putUint(si, 5, 100)
	si = putUint(si, 5, 101)
	si = putBytes(si, 6, anim)
	si = putUint(si, 7, 64)
	si = putUint(si, 8, 1)
	group := putBytes(putUint(putUint(nil, 1, 2), 2, 2), 3, si)
	app := putBytes(putBytes(putUint(nil, 1, 102), 2, group), 3, flags)
	app = putBytes(app, 4, []byte("Bag"))
	file := putBytes(nil, 1, app)
	file = putBytes(file, 5, putUint(nil, 1, 3031))

	f, err := Decode(file, fakeMap{})
	if err != nil {
		t.Fatal(err)
	}
	items := f.Things[thing.CategoryItem]
	if len(items) != 3 || !Blank(items[0]) || Blank(items[2]) {
		t.Fatalf("items %d", len(items))
	}
	it := items[2]
	g := it.FrameGroups[0]
	if g.Width != 2 || g.Height != 2 || g.Frames != 2 || g.LoopCount != -1 || g.Mode != thing.AnimationSync || g.ExactSize != 64 {
		t.Fatalf("group %+v", g)
	}
	if g.Sprites[0] != 1000 || g.Sprites[4] != 1004 || g.Durations[0] != (thing.FrameDuration{Min: 100, Max: 200}) {
		t.Fatalf("sprites %v %v", g.Sprites, g.Durations)
	}
	p := it.Props
	if !p.Unpassable || p.LightColor != 215 || p.OffsetX != -8 || !p.IsMarket || p.Market.Category != 4 || p.Market.Name != "bag" {
		t.Fatalf("props %+v", p)
	}
	resolve := func(pieces []uint32, w, h int) (uint32, error) {
		return 100 + (pieces[0]-1000)/4, nil
	}
	out, err := Encode(f, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, file) {
		t.Fatal("untouched file must be written back unchanged")
	}

	// Edit: unknown fields (name, ammo, vocation, special ids) survive.
	edited := it.Clone()
	edited.Props.Pickupable = true
	edited.Props.Market.Name = "big bag"
	edited.Description = "maître"
	items[2] = edited
	out, err = Encode(f, resolve)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := Decode(out, fakeMap{})
	if err != nil {
		t.Fatal(err)
	}
	e2 := f2.Things[thing.CategoryItem][2]
	if !e2.Props.Pickupable || e2.Props.Market.Name != "big bag" || e2.Props.LightLevel != 3 || e2.Name != "Bag" || e2.Description != "maître" {
		t.Fatalf("props %+v", e2.Props)
	}
	m, _ := parse(out)
	am, _ := parse(m[0].b)
	fl := am.sub(3)
	if d, _ := am.get(fieldDescription); string(d.b) != "ma\xeetre" {
		t.Fatalf("description must be Latin-1: %q", d.b)
	}
	if n, _ := am.get(4); string(n.b) != "Bag" || fl.uint(45) != 1 || fl.sub(fMarket).uint(5) != 2 {
		t.Fatal("unknown fields lost")
	}
	if len(m) != 2 || m[1].num != 5 {
		t.Fatal("special meaning ids lost")
	}
	si2 := am.sub(2).sub(3)
	if si2.uint(7) != 64 || si2.uint(8) != 1 {
		t.Fatal("bounding square or opacity lost with unchanged sprites")
	}
}

// TestOfficialAssets round trips the files of a Tibia installation when
// OTS_ASSETS points to its assets folder.
func TestOfficialAssets(t *testing.T) {
	dir := os.Getenv("OTS_ASSETS")
	if dir == "" {
		t.Skip("OTS_ASSETS not set")
	}
	c, err := ReadCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, c.Appearances))
	if err != nil {
		t.Fatal(err)
	}
	m := NewPieceMap(c)
	f, err := Decode(data, m)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d items, %d pieces", len(f.Things[thing.CategoryItem]), m.Count())
	resolve := func(pieces []uint32, w, h int) (uint32, error) {
		id, _ := m.Sprite(pieces, w, h)
		return id, nil
	}
	out, err := Encode(f, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs: %d vs %d bytes", len(out), len(data))
	}
	// Regenerating every thing must keep the same model.
	for _, c := range thing.Categories {
		for i, th := range f.Things[c] {
			f.Things[c][i] = th.Clone()
		}
	}
	out, err = Encode(f, resolve)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := Decode(out, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range thing.Categories {
		a, b := f.Things[c], f2.Things[c]
		if len(a) != len(b) {
			t.Fatalf("%s count %d vs %d", c, len(a), len(b))
		}
		for i := range a {
			if a[i].Props != b[i].Props || a[i].Name != b[i].Name || a[i].Description != b[i].Description || len(a[i].FrameGroups) != len(b[i].FrameGroups) {
				t.Fatalf("%s %d differs", c, a[i].ID)
			}
			for gi, g := range a[i].FrameGroups {
				h := b[i].FrameGroups[gi]
				if !reflect.DeepEqual(g, h) {
					t.Fatalf("%s %d group %d differs: %+v vs %+v", c, a[i].ID, gi, *g, *h)
				}
			}
		}
	}
	// One sheet decodes.
	raw, err := os.ReadFile(filepath.Join(dir, c.Sheets[0].File))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSheet(raw); err != nil {
		t.Fatal(err)
	}
}

func TestNewerFlagsRoundTrip(t *testing.T) {
	npc := putBytes(putBytes(nil, npcName, []byte("Rashid")), npcLocation, []byte("Svargrond"))
	npc = putUint(putUint(npc, npcSalePrice, 0), npcBuyPrice, 150)
	flags := putBytes(nil, fNpcSaleData, npc)
	flags = putBytes(flags, fNpcSaleData, putBytes(putUint(nil, npcSalePrice, 20), npcQuestFlag, []byte("gold token")))
	flags = putBytes(flags, fChangedExpire, putUint(nil, 1, 3051))
	flags = putBool(flags, 42, true) // corpse
	flags = putBytes(flags, fCyclopedia, putUint(nil, 1, 3))
	flags = putBytes(flags, fUpgrade, putUint(nil, 1, 4))
	flags = putBool(flags, 57, true) // wrapkit
	flags = putBytes(flags, fMarket, putUint(putUint(nil, marketVocation, 10), marketVocation, 1)) // promoted + knight
	app := putBytes(putUint(nil, 1, 100), 3, flags)

	m, _ := parse(app)
	var p thing.Properties
	decodeFlags(m.sub(3), &p)
	sales := decodeNpcSales(m.sub(3))
	if len(sales) != 2 || sales[0].Name != "Rashid" || sales[0].BuyPrice != 150 || sales[1].CurrencyQuestFlag != "gold token" {
		t.Fatalf("npc %+v", sales)
	}
	if !p.ChangedToExpire || p.FormerObjectID != 3051 || !p.Corpse || p.CyclopediaType != 3 || p.UpgradeClassification != 4 || !p.WrapKit {
		t.Fatalf("props %+v", p)
	}
	if p.Market.RestrictProfession != 1 {
		t.Fatalf("vocations %d", p.Market.RestrictProfession)
	}
	if got := p.AssetOnly(); len(got) != 5 {
		t.Fatalf("asset only %v", got)
	}

	// Unchanged vocations keep "promoted"; edited ones are rewritten.
	out, _ := parse(encodeFlags(&p, sales, m.sub(3)))
	if v := out.sub(fMarket).uints(marketVocation); len(v) != 2 {
		t.Fatalf("vocations %v", v)
	}
	p.Market.RestrictProfession = 1 | 8 // knight, druid
	p.UpgradeClassification = 2
	sales[0].SalePrice = 99
	out, _ = parse(encodeFlags(&p, sales, m.sub(3)))
	var p2 thing.Properties
	decodeFlags(out, &p2)
	if v := out.sub(fMarket).uints(marketVocation); len(v) != 2 || v[0] != 1 || v[1] != 4 {
		t.Fatalf("vocations %v", v)
	}
	if p2.UpgradeClassification != 2 || decodeNpcSales(out)[0].SalePrice != 99 || len(out.all(fNpcSaleData)) != 2 {
		t.Fatalf("props %+v", p2)
	}
}
