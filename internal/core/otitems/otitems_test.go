package otitems

import (
	"bytes"
	"errors"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/itemsotb"
	"github.com/nekiro/ots-creator/internal/formats/otbnode"
)

// writeOTB builds a minimal items.otb with the given items.
func writeOTB(t *testing.T, items []struct {
	group          Group
	flags          uint32
	server, client uint16
	top            uint8
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := otbnode.NewWriter(&buf, [4]byte{})
	writer.Start(0)
	writer.U32(0)
	writer.U8(0x01)
	writer.U16(140)
	writer.U32(3)
	writer.U32(57)
	writer.U32(9)
	writer.Bytes(make([]byte, 128))
	for _, item := range items {
		writer.Start(byte(item.group))
		writer.U32(item.flags)
		writer.U8(itemsotb.AttrServerID)
		writer.U16(2)
		writer.U16(item.server)
		writer.U8(itemsotb.AttrClientID)
		writer.U16(2)
		writer.U16(item.client)
		if item.top != 0 {
			writer.U8(itemsotb.AttrStackOrder)
			writer.U16(1)
			writer.U8(item.top)
		}
		writer.U8(0x99) // unknown attribute is skipped
		writer.U16(3)
		writer.Bytes([]byte{1, 2, 3})
		writer.End()
	}
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseOTB(t *testing.T) {
	data := writeOTB(t, []struct {
		group          Group
		flags          uint32
		server, client uint16
		top            uint8
	}{
		{GroupGround, 0, 100, 100, 0},
		{GroupNone, itemsotb.FlagStackOrder | itemsotb.FlagUnpassable, 1050, 1060, 2},
		{GroupContainer, itemsotb.FlagPickupable | itemsotb.FlagMovable, 1987, 1987, 0},
		{GroupNone, itemsotb.FlagStackable | itemsotb.FlagPickupable, 2148, 3031, 0},
		{GroupNone, itemsotb.FlagHookSouth, 3000, 3000, 0},
		{GroupNone, itemsotb.FlagHookEast, 3001, 3001, 0},
		{GroupDeprecated, 0, 3002, 0, 0},
	})
	db, err := ParseOTB(data)
	if err != nil {
		t.Fatal(err)
	}
	if db.MajorVersion != 3 || db.MinorVersion != 57 || db.Mode != ModeOTB {
		t.Fatalf("header %d.%d mode %s", db.MajorVersion, db.MinorVersion, db.Mode)
	}
	if ground := db.Get(100); ground == nil || !ground.IsGround() {
		t.Fatal("ground missing")
	}
	wall := db.Get(1050)
	if wall == nil || wall.ClientID != 1060 || wall.TopOrder != 2 || !wall.Flags.Has(FlagUnpassable) {
		t.Fatalf("wall = %+v", wall)
	}
	if !db.Get(1987).IsContainer() || !db.Get(2148).HasSubtype() {
		t.Fatal("container or stackable flags lost")
	}
	if south, east := db.Get(3000).Flags, db.Get(3001).Flags; !south.Has(FlagHookSouth) || south.Has(FlagHookEast) ||
		!east.Has(FlagHookEast) || east.Has(FlagHookSouth) {
		t.Fatalf("hook flags south %b east %b", south, east)
	}
	if db.Get(3002) != nil {
		t.Fatal("deprecated item kept")
	}
	if id, ok := db.FromClient(3031); !ok || id != 2148 {
		t.Fatalf("FromClient = %d %v", id, ok)
	}

	xmlData := []byte(`<?xml version="1.0"?><items>
		<item id="100" name="void"/>
		<item fromid="1050" toid="1051" article="a" name="stone wall"><attribute key="rotateTo" value="1051"/></item>
		<item id="1987" name="bag"><attribute key="type" value="container"/><attribute key="containerSize" value="8"/></item>
	</items>`)
	if err := db.ApplyXML(xmlData); err != nil {
		t.Fatal(err)
	}
	if db.Get(100).Name != "void" || db.Get(1050).Name != "stone wall" || db.Get(1050).RotateTo != 1051 {
		t.Fatal("xml names not applied")
	}
	if bag := db.Get(1987); bag.Kind != KindContainer || bag.MaxItems != 8 {
		t.Fatalf("bag = %+v", bag)
	}
	if got := db.Search("wall", 10); len(got) != 1 || got[0] != 1050 {
		t.Fatalf("search = %v", got)
	}
	if err := db.ApplyXML([]byte(`<other/>`)); !errors.Is(err, ErrNoItems) {
		t.Fatalf("err = %v", err)
	}
}

type fakeSource map[uint32]*thing.Thing

func (source fakeSource) Thing(category thing.Category, id uint32) (*thing.Thing, error) {
	if found, ok := source[id]; ok {
		return found, nil
	}
	return nil, errors.New("missing")
}

func TestFromClient(t *testing.T) {
	ground := thing.New(100, thing.CategoryItem)
	ground.Props.Ground = true
	ground.Props.GroundSpeed = 150
	ground.Props.MiniMap = true
	ground.Props.MiniMapColor = 24
	border := thing.New(101, thing.CategoryItem)
	border.Props.GroundBorder = true
	top := thing.New(102, thing.CategoryItem)
	top.Props.OnTop = true
	top.Name = "arch"
	db := FromClient(fakeSource{100: ground, 101: border, 102: top}, 103, map[uint32]string{100: "grass"})
	if db.Mode != ModeClient {
		t.Fatal("mode")
	}
	grass := db.Get(100)
	if !grass.IsGround() || grass.Speed != 150 || grass.MinimapColor != 24 || grass.Name != "grass" || grass.ClientID != 100 {
		t.Fatalf("ground = %+v", grass)
	}
	if !db.Get(101).IsBorder() || db.Get(102).TopOrder != 3 || db.Get(102).Name != "arch" {
		t.Fatal("top orders or names wrong")
	}
	if db.Get(103) != nil {
		t.Fatal("missing client item created")
	}
}

func TestKindName(t *testing.T) {
	cases := []struct {
		itemType Type
		want     string
	}{
		{Type{Kind: KindDepot, Flags: FlagContainer}, "depot"},
		{Type{Flags: FlagContainer}, "container"},
		{Type{Group: GroupDoor}, "door"},
		{Type{Group: GroupTeleport}, "teleport"},
		{Type{Flags: FlagSplash}, "fluid"},
		{Type{Flags: FlagStackable}, "stackable"},
		{Type{Flags: FlagWritable}, "writeable"},
		{Type{Flags: FlagCharges}, "charges"},
		{Type{}, ""},
	}
	for _, test := range cases {
		if got := test.itemType.KindName(); got != test.want {
			t.Errorf("KindName(%+v) = %q, want %q", test.itemType, got, test.want)
		}
	}
	if FlagNames()["hookEast"] != FlagHookEast || len(FlagNames()) != 23 {
		t.Fatal("FlagNames")
	}
}

func TestApplyClient(t *testing.T) {
	wall := &Type{ID: 1050, ClientID: 1060, Flags: FlagUnpassable, Name: "wall"}
	unnamed := &Type{ID: 1051, ClientID: 1061, LightLevel: 2, LightColor: 9, MinimapColor: 5}
	noClient := &Type{ID: 1052}
	db := NewDB(ModeOTB, []*Type{wall, unnamed, noClient})

	wallThing := thing.New(1060, thing.CategoryItem)
	wallThing.Name = "client wall"
	wallThing.Props.MiniMap, wallThing.Props.MiniMapColor = true, 86
	wallThing.Props.FullGround = true
	wallThing.Props.HasLight, wallThing.Props.LightLevel, wallThing.Props.LightColor = true, 4, 215
	wallThing.Props.HookSouth, wallThing.Props.Hangable = true, true
	// Server data is not taken from the client for otb databases.
	wallThing.Props.Pickupable, wallThing.Props.Ground = true, true
	unnamedThing := thing.New(1061, thing.CategoryItem)
	unnamedThing.Name = "torch"
	unnamedThing.Props.MiniMap, unnamedThing.Props.MiniMapColor = true, 86
	unnamedThing.Props.HasLight, unnamedThing.Props.LightLevel = true, 7
	unnamedThing.Props.HookEast, unnamedThing.Props.Corpse = true, true

	db.ApplyClient(fakeSource{1060: wallThing, 1061: unnamedThing})
	wantWall := FlagUnpassable | FlagFullGround | FlagLight | FlagHookSouth | FlagHangable
	if wall.Flags != wantWall || wall.MinimapColor != 86 || wall.LightLevel != 4 || wall.LightColor != 215 ||
		wall.Name != "wall" || wall.Group != GroupNone || wall.Speed != 0 {
		t.Fatalf("wall = %+v, want flags %b", wall, wantWall)
	}
	if unnamed.Flags != FlagLight|FlagHookEast|FlagCorpse || unnamed.MinimapColor != 5 || unnamed.LightLevel != 2 ||
		unnamed.LightColor != 9 || unnamed.Name != "torch" {
		t.Fatalf("unnamed = %+v", unnamed)
	}
	if noClient.Flags != 0 || noClient.Name != "" {
		t.Fatalf("item without client id changed: %+v", noClient)
	}
}

func TestApplyThingAll(t *testing.T) {
	cases := []struct {
		name  string
		setup func(props *thing.Properties)
		check func(itemType *Type) bool
	}{
		{"container", func(props *thing.Properties) { props.Container = true }, func(itemType *Type) bool {
			return itemType.Group == GroupContainer && itemType.Kind == KindContainer && itemType.Flags.Has(FlagContainer|FlagMoveable)
		}},
		{"splash", func(props *thing.Properties) { props.Fluid = true }, func(itemType *Type) bool {
			return itemType.Group == GroupSplash && itemType.Flags.Has(FlagSplash)
		}},
		{"fluid container", func(props *thing.Properties) { props.FluidContainer = true }, func(itemType *Type) bool {
			return itemType.Group == GroupFluid && itemType.Flags.Has(FlagFluidContainer)
		}},
		{"writable", func(props *thing.Properties) { props.Writable = true }, func(itemType *Type) bool {
			return itemType.Group == GroupWritable && itemType.Flags.Has(FlagWritable|FlagReadable)
		}},
		{"write once", func(props *thing.Properties) { props.WritableOnce = true }, func(itemType *Type) bool {
			return itemType.Group == GroupWritable && itemType.Flags.Has(FlagReadable) && !itemType.Flags.Has(FlagWritable)
		}},
		{"bottom", func(props *thing.Properties) { props.OnBottom, props.Unmoveable = true, true }, func(itemType *Type) bool {
			return itemType.TopOrder == 2 && !itemType.Flags.Has(FlagMoveable)
		}},
		{"blocking", func(props *thing.Properties) {
			props.Unpassable, props.BlockMissile, props.BlockPathfind = true, true, true
			props.FloorChange, props.HasElevation, props.HasCharges = true, true, true
			props.Rotatable, props.Stackable, props.IgnoreLook = true, true, true
		}, func(itemType *Type) bool {
			return itemType.Flags.Has(FlagUnpassable|FlagBlockMissile|FlagBlockPath|FlagFloorChange|FlagElevation|
				FlagCharges|FlagRotatable|FlagStackable|FlagIgnoreLook) && itemType.Group == GroupNone
		}},
	}
	for _, test := range cases {
		clientThing := thing.New(100, thing.CategoryItem)
		test.setup(&clientThing.Props)
		itemType := &Type{ID: 100, ClientID: 100}
		applyThing(itemType, clientThing, true)
		if !test.check(itemType) {
			t.Errorf("%s: %+v", test.name, itemType)
		}
	}
}

func TestDB(t *testing.T) {
	var nilDB *DB
	if nilDB.Get(1) != nil {
		t.Fatal("nil database returned a type")
	}
	db := NewDB(ModeClient, []*Type{{ID: 5, ClientID: 7}, {ID: 2, ClientID: 7}, {ID: 3}})
	if db.MaxID() != 5 || db.Count() != 3 || db.Get(4) != nil || db.Get(6) != nil {
		t.Fatalf("max %d count %d", db.MaxID(), db.Count())
	}
	if id, ok := db.FromClient(7); !ok || id != 2 {
		t.Fatalf("FromClient(7) = %d %v, want the lowest server id", id, ok)
	}
	if _, ok := db.FromClient(0); ok {
		t.Fatal("client id 0 indexed")
	}
	var ids []uint16
	db.Each(func(itemType *Type) { ids = append(ids, itemType.ID) })
	if len(ids) != 3 || ids[0] != 2 || ids[2] != 5 {
		t.Fatalf("Each order %v", ids)
	}
	if (&DB{}).MaxID() != 0 {
		t.Fatal("empty MaxID")
	}

	db.MarkMeta([]uint16{3, 9})
	if !db.Get(3).Flags.Has(FlagMeta) || db.Get(3).Name != "" {
		t.Fatalf("existing meta item %+v", db.Get(3))
	}
	if created := db.Get(9); created == nil || !created.Flags.Has(FlagMeta) || created.Name != "editor item" {
		t.Fatalf("created meta item %+v", created)
	}
	if got := db.Search("  ", 10); len(got) != 0 {
		t.Fatalf("empty search %v", got)
	}
	if got := db.Search("9", 10); len(got) != 1 || got[0] != 9 {
		t.Fatalf("id search %v", got)
	}
	if got := db.Search("editor", 0); len(got) != 0 {
		t.Fatalf("search past limit %v", got)
	}
}

func TestTypePredicates(t *testing.T) {
	if name := (&Type{ID: 12}).DisplayName(); name != "item 12" {
		t.Fatalf("placeholder %q", name)
	}
	if name := (&Type{ID: 12, Name: "sword"}).DisplayName(); name != "sword" {
		t.Fatalf("name %q", name)
	}
	if !(&Type{Kind: KindDoor}).IsDoor() || !(&Type{Group: GroupDoor}).IsDoor() || (&Type{}).IsDoor() {
		t.Fatal("IsDoor")
	}
	if !(&Type{Kind: KindTeleport}).IsTeleport() || !(&Type{Group: GroupTeleport}).IsTeleport() || (&Type{}).IsTeleport() {
		t.Fatal("IsTeleport")
	}
	if !(&Type{Kind: KindDepot}).IsDepot() || (&Type{Group: GroupContainer}).IsDepot() {
		t.Fatal("IsDepot")
	}
	for name, kind := range kindNames {
		if kind.String() != name {
			t.Errorf("%d.String() = %q, want %q", kind, kind.String(), name)
		}
	}
	if KindNone.String() != "" {
		t.Fatalf("KindNone.String() = %q", KindNone.String())
	}
}

func TestApplyXMLTypes(t *testing.T) {
	db := NewDB(ModeClient, []*Type{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}})
	xmlData := []byte(`<items>
		<item id="1" plural="coins" editorsuffix="(x)"><attribute key="type" value="depot"/></item>
		<item id="2"><attribute key="charges" value="5"/><attribute key="corpseType" value="blood"/></item>
		<item id="3"><attribute key="writeable" value="1"/></item>
		<item id="4"><attribute key="readable" value="1"/><attribute key="type" value="unknown"/></item>
		<item fromid="6" toid="5" name="reversed"/>
		<item fromid="7" toid="8" name="missing"/>
	</items>`)
	if err := db.ApplyXML(xmlData); err != nil {
		t.Fatal(err)
	}
	if depot := db.Get(1); depot.Kind != KindDepot || !depot.IsContainer() || depot.Plural != "coins" || depot.EditorSuffix != "(x)" {
		t.Fatalf("depot %+v", depot)
	}
	if !db.Get(2).Flags.Has(FlagCharges|FlagCorpse) || !db.Get(3).Flags.Has(FlagWritable|FlagReadable) {
		t.Fatal("charges, corpse or writable flags missing")
	}
	if four := db.Get(4); four.Flags != FlagReadable || four.Kind != KindNone {
		t.Fatalf("readable %+v", four)
	}
	if db.Get(6).Name != "reversed" || db.Get(5).Name != "" {
		t.Fatalf("reversed range applied to %q %q", db.Get(5).Name, db.Get(6).Name)
	}
	if err := db.ApplyXML([]byte(`<items><item id="1"`)); err == nil {
		t.Fatal("broken xml parsed")
	}
}
