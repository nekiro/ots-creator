package dat

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/binio"
	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/thing"
)

// fullProps sets every property with distinct payload values.
func fullProps() thing.Properties {
	return thing.Properties{
		Ground: true, GroundSpeed: 150, GroundBorder: true, OnBottom: true, OnTop: true,
		Container: true, Stackable: true, ForceUse: true, MultiUse: true, HasCharges: true,
		Writable: true, MaxTextLength: 255, WritableOnce: true, MaxReadLength: 1024,
		FluidContainer: true, Fluid: true, Unpassable: true, Unmoveable: true,
		BlockMissile: true, BlockPathfind: true, NoMoveAnimation: true, Pickupable: true,
		Hangable: true, HookSouth: true, HookEast: true, Rotatable: true,
		HasLight: true, LightLevel: 7, LightColor: 215, DontHide: true, Translucent: true,
		FloorChange: true, HasOffset: true, OffsetX: -8, OffsetY: 16,
		HasElevation: true, Elevation: 8, LyingObject: true, AnimateAlways: true,
		MiniMap: true, MiniMapColor: 129, LensHelp: true, LensHelpValue: 1112,
		FullGround: true, IgnoreLook: true, Cloth: true, ClothSlot: 5,
		IsMarket: true, Market: thing.Market{Category: 3, TradeAs: 2400, ShowAs: 2401, Name: "magic sword é", RestrictProfession: 1, RestrictLevel: 80},
		HasDefaultAction: true, DefaultAction: 2, Wrappable: true, Unwrappable: true,
		TopEffect: true, Usable: true, HasBones: true,
		Bones: [4]thing.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}},
	}
}

func TestFlagTablesRoundTrip(t *testing.T) {
	tables := []*FlagTable{table1, table2, table3, table4, table5, table6, OBDTable}
	for _, tb := range tables {
		t.Run(tb.Name(), func(t *testing.T) {
			src := fullProps()
			w := binio.NewWriter(0)
			tb.WriteProperties(w, &src)
			first := slicesClone(w.Bytes())

			var got thing.Properties
			r := binio.NewReader(first)
			if err := tb.ReadProperties(r, &got); err != nil {
				t.Fatal(err)
			}
			if r.Remaining() != 0 {
				t.Fatalf("%d bytes left", r.Remaining())
			}
			for a := range tb.byAttr {
				if !has(a, &got) {
					t.Errorf("attr %s lost", attrNames[a])
				}
			}
			if u := tb.Unsupported(&got); len(u) != 0 {
				t.Errorf("decoded props contain unsupported attrs %v", u)
			}
			if tb.byAttr[aLight] != 0 || tb == table1 { // every table has light
				if got.LightLevel != 7 || got.LightColor != 215 {
					t.Errorf("light = %d/%d", got.LightLevel, got.LightColor)
				}
			}
			if _, ok := tb.byAttr[aMarket]; ok && got.Market != src.Market {
				t.Errorf("market = %+v", got.Market)
			}
			if _, ok := tb.byAttr[aBones]; ok && got.Bones != src.Bones {
				t.Errorf("bones = %+v", got.Bones)
			}
			if tb.offsetHasPayload {
				if got.OffsetX != -8 || got.OffsetY != 16 {
					t.Errorf("offset = %d,%d", got.OffsetX, got.OffsetY)
				}
			} else if got.OffsetX != 8 || got.OffsetY != 8 {
				t.Errorf("legacy offset = %d,%d, want 8,8", got.OffsetX, got.OffsetY)
			}

			w2 := binio.NewWriter(0)
			tb.WriteProperties(w2, &got)
			if !bytes.Equal(first, w2.Bytes()) {
				t.Fatal("second encode differs")
			}
		})
	}
}

func slicesClone(b []byte) []byte { return append([]byte(nil), b...) }

func TestFlagBytesMatchObjectBuilder(t *testing.T) {
	// Spot checks against MetadataFlagsN.as.
	checks := []struct {
		tb   *FlagTable
		a    attr
		want byte
	}{
		{table1, aMiniMap, 0x16}, {table1, aLensHelp, 0x1A},
		{table2, aHangable, 0x19}, {table2, aLensHelp, 0x1D},
		{table3, aFloorChange, 0x17}, {table3, aFullGround, 0x1E},
		{table4, aHasCharges, 0x08}, {table4, aBones, 0x27}, {table4, aIgnoreLook, 0x20},
		{table5, aCloth, 0x20}, {table5, aMarket, 0x21}, {table5, aTranslucent, 0x17},
		{table6, aNoMoveAnimation, 0x10}, {table6, aMarket, 0x22}, {table6, aUsable, 0xFE},
		{OBDTable, aHasCharges, 0xFC}, {OBDTable, aFloorChange, 0xFD},
	}
	for _, c := range checks {
		if got := c.tb.byAttr[c.a]; got != c.want {
			t.Errorf("%s %s = 0x%02X, want 0x%02X", c.tb.Name(), attrNames[c.a], got, c.want)
		}
	}
	if _, ok := OBDTable.byAttr[aBones]; ok {
		t.Error("OBD must not store bones")
	}
}

func TestUnknownFlag(t *testing.T) {
	var p thing.Properties
	err := table3.ReadProperties(binio.NewReader([]byte{0x04, 0x16, 0xFF}), &p)
	if err == nil || !strings.Contains(err.Error(), "0x16") {
		t.Fatalf("err = %v", err)
	}
}

func TestTruncatedProperties(t *testing.T) {
	var p thing.Properties
	if err := table6.ReadProperties(binio.NewReader([]byte{0x00, 0x01}), &p); err == nil {
		t.Fatal("expected EOF")
	}
}

func animatedGroup(typ thing.FrameGroupType, frames uint8, base uint32) *thing.FrameGroup {
	g := &thing.FrameGroup{Type: typ, Width: 2, Height: 2, ExactSize: 64, Layers: 2, PatternX: 4, PatternY: 1, PatternZ: 2, Frames: frames, Mode: thing.AnimationSync, LoopCount: -1, StartFrame: 1}
	g.Sprites = make([]uint32, g.TotalSprites())
	for i := range g.Sprites {
		g.Sprites[i] = base + uint32(i)
	}
	if frames > 1 {
		g.Durations = make([]thing.FrameDuration, frames)
		for i := range g.Durations {
			g.Durations[i] = thing.FrameDuration{Min: uint32(100 + i), Max: uint32(200 + i)}
		}
	}
	return g
}

func sampleFile(withGroups bool) *File {
	f := NewFile(0x42A3)
	item := thing.New(100, thing.CategoryItem)
	item.Props.Ground, item.Props.GroundSpeed = true, 150
	item2 := thing.New(101, thing.CategoryItem)
	item2.Props.Pickupable = true
	item2.FrameGroups[0] = animatedGroup(0, 3, 10)
	f.Things[thing.CategoryItem] = []*thing.Thing{item, item2}

	outfit := thing.New(1, thing.CategoryOutfit)
	outfit.FrameGroups = []*thing.FrameGroup{animatedGroup(0, 1, 500), animatedGroup(1, 4, 600)}
	outfit.FrameGroups[0].Mode, outfit.FrameGroups[0].LoopCount, outfit.FrameGroups[0].StartFrame = 0, 0, 0
	if !withGroups {
		outfit.FrameGroups = outfit.FrameGroups[1:]
		outfit.FrameGroups[0].Type = 0
	}
	f.Things[thing.CategoryOutfit] = []*thing.Thing{outfit}

	effect := thing.New(1, thing.CategoryEffect)
	effect.Props.TopEffect = true
	effect.FrameGroups[0] = animatedGroup(0, 2, 70000)
	f.Things[thing.CategoryEffect] = []*thing.Thing{effect}
	// No missiles: header must contain 0.
	return f
}

func TestFileRoundTripModern(t *testing.T) {
	opts := OptionsFor(1098, client.Features{})
	src := sampleFile(true)
	data, err := Encode(src, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxID(thing.CategoryMissile) != 0 || len(got.Things[thing.CategoryMissile]) != 0 {
		t.Fatal("missiles should be empty")
	}
	for _, c := range []thing.Category{thing.CategoryItem, thing.CategoryOutfit, thing.CategoryEffect} {
		if !reflect.DeepEqual(src.Things[c], got.Things[c]) {
			t.Errorf("%s mismatch:\nsrc %+v\ngot %+v", c, src.Things[c][0].FrameGroups[0], got.Things[c][0].FrameGroups[0])
		}
	}
	again, err := Encode(got, opts)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("re-encode differs: %v", err)
	}
	if got.Get(thing.CategoryItem, 101) == nil || got.Get(thing.CategoryItem, 99) != nil || got.Get(thing.CategoryItem, 102) != nil {
		t.Fatal("Get bounds wrong")
	}
}

func TestFileLegacyDefaultsDurations(t *testing.T) {
	opts := OptionsFor(860, client.Features{})
	if opts.Features.Extended || opts.Features.ImprovedAnimations {
		t.Fatal("8.60 must not force extended/animations")
	}
	src := sampleFile(false)
	// Effect sprite ids above 0xFFFF must be rejected without extended.
	if _, err := Encode(src, opts); err == nil || !strings.Contains(err.Error(), "extended") {
		t.Fatalf("expected extended error, got %v", err)
	}
	src.Things[thing.CategoryEffect][0].FrameGroups[0] = animatedGroup(0, 2, 7000)
	data, err := Encode(src, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data, opts)
	if err != nil {
		t.Fatal(err)
	}
	g := got.Get(thing.CategoryItem, 101).FrameGroups[0]
	if len(g.Durations) != 3 || g.Durations[0].Min != 500 || g.Durations[0].Max != 500 {
		t.Fatalf("default item durations = %+v", g.Durations)
	}
	eg := got.Get(thing.CategoryEffect, 1).FrameGroups[0]
	if eg.Durations[1].Min != 100 {
		t.Fatalf("default effect durations = %+v", eg.Durations)
	}
	if got.Get(thing.CategoryOutfit, 1).FrameGroups[0].Durations[0].Min != 300 {
		t.Fatal("default outfit duration")
	}
}

func TestFileNoPatternZ(t *testing.T) {
	opts := OptionsFor(740, client.Features{})
	f := NewFile(1)
	it := thing.New(100, thing.CategoryItem)
	it.Props.HasOffset = true
	f.Things[thing.CategoryItem] = []*thing.Thing{it}
	data, err := Encode(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	// header 12 + flags (0x14, 0xFF) + w,h,layers,px,py,frames + u16 sprite
	if len(data) != 12+2+6+2 {
		t.Fatalf("len = %d", len(data))
	}
	got, err := Decode(data, opts)
	if err != nil {
		t.Fatal(err)
	}
	p := got.Get(thing.CategoryItem, 100).Props
	if !p.HasOffset || p.OffsetX != 8 {
		t.Fatalf("offset = %+v", p)
	}
	it.FrameGroups[0].PatternZ = 2
	it.FrameGroups[0].Sprites = make([]uint32, 2)
	if _, err := Encode(f, opts); err == nil {
		t.Fatal("pattern Z > 1 must fail on 7.40")
	}
}

func TestSingleOutfitGroupWrittenAsWalking(t *testing.T) {
	opts := OptionsFor(1098, client.Features{})
	o := thing.New(1, thing.CategoryOutfit)
	w := binio.NewWriter(0)
	if err := WriteThing(w, o, opts); err != nil {
		t.Fatal(err)
	}
	b := w.Bytes()
	// flags: LastFlag, then group count 1, group type 1
	if b[0] != LastFlag || b[1] != 1 || b[2] != 1 {
		t.Fatalf("bytes = %v", b[:3])
	}
}

func TestDecodeErrors(t *testing.T) {
	opts := OptionsFor(1098, client.Features{})
	if _, err := Decode([]byte{1, 2, 3}, opts); err == nil {
		t.Fatal("short header must fail")
	}
	data, _ := Encode(sampleFile(true), opts)
	for _, cut := range []int{13, 20, len(data) - 1} {
		if _, err := Decode(data[:cut], opts); err == nil {
			t.Fatalf("truncated at %d must fail", cut)
		}
	}
	// zero width layout
	bad := []byte{0, 0, 0, 0, 100, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0, 1, 1, 1, 1, 1, 1}
	if _, err := Decode(bad, opts); err == nil {
		t.Fatal("zero layout must fail")
	}
}

func TestUnsupportedListsLostFlags(t *testing.T) {
	p := thing.Properties{IsMarket: true, Usable: true, Pickupable: true}
	u := table4.Unsupported(&p)
	if len(u) != 2 {
		t.Fatalf("unsupported = %v", u)
	}
}
