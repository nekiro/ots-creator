package itemsotb

import (
	"bytes"
	"crypto/md5"
	"testing"

	"github.com/nekiro/ots-creator/internal/formats/otbnode"
	"github.com/nekiro/ots-creator/internal/core/thing"
)

// sample builds an items.otb by hand, with an unknown attribute, a
// deprecated item and server-only flags.
func sample(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := otbnode.NewWriter(&buf, [4]byte{})
	writer.Start(0)
	writer.U32(0)
	writer.U8(rootAttrVersion)
	writer.U16(140)
	writer.U32(3)
	writer.U32(57)
	writer.U32(62)
	writer.Bytes(make([]byte, 128))
	item := func(group Group, flags uint32, server, client uint16, extra ...byte) {
		writer.Start(byte(group))
		writer.U32(flags)
		writer.U8(AttrServerID)
		writer.U16(2)
		writer.U16(server)
		if client != 0 {
			writer.U8(AttrClientID)
			writer.U16(2)
			writer.U16(client)
		}
		writer.Bytes(extra)
		writer.End()
	}
	item(GroupGround, FlagMovable, 100, 100, AttrGroundSpeed, 2, 0, 150, 0)
	item(GroupDeprecated, 0, 101, 0)
	// 13 = door; 1<<8 floor change down; 0x99 unknown; 0xFE needs escaping.
	item(13, FlagUnpassable|1<<8, 102, 254, 0x99, 1, 0, 0xFE)
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoundTrip(t *testing.T) {
	data := sample(t)
	file, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if major, minor, build := file.Version(); major != 3 || minor != 57 || build != 62 {
		t.Fatalf("version %d.%d.%d", major, minor, build)
	}
	if len(file.Items) != 3 || file.Items[2].ServerID() != 102 || file.Items[2].ClientID() != 254 || !file.Items[1].Deprecated() {
		t.Fatalf("items %+v", file.Items)
	}
	out, err := file.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatal("unchanged file does not encode to the same bytes")
	}
}

func TestReloadKeepsServerData(t *testing.T) {
	file, err := Parse(sample(t))
	if err != nil {
		t.Fatal(err)
	}
	item := file.Items[2]
	clientThing := thing.New(254, thing.CategoryItem)
	clientThing.Props.Pickupable = true
	clientThing.Props.OnTop = true
	clientThing.Props.HasLight, clientThing.Props.LightLevel, clientThing.Props.LightColor = true, 3, 215
	clientThing.FrameGroups[0].Frames = 2
	hash := [16]byte{1, 2, 3}
	item.Reload(clientThing, &hash)
	want := uint32(FlagPickupable | FlagMovable | FlagStackOrder | FlagIsAnimation | 1<<8)
	if item.Flags != want {
		t.Fatalf("flags %b, want %b", item.Flags, want)
	}
	if item.Group != 13 {
		t.Fatalf("server group changed to %d", item.Group)
	}
	if data := item.Attr(AttrStackOrder); len(data) != 1 || data[0] != 3 {
		t.Fatalf("stack order %v", data)
	}
	if data := item.Attr(AttrLight); !bytes.Equal(data, []byte{3, 0, 215, 0}) {
		t.Fatalf("light %v", data)
	}
	if data := item.Attr(0x99); !bytes.Equal(data, []byte{0xFE}) {
		t.Fatalf("unknown attribute %v", data)
	}
	if data := item.Attr(AttrSpriteHash); !bytes.Equal(data, hash[:]) {
		t.Fatalf("hash %v", data)
	}
	if item.ServerID() != 102 || item.ClientID() != 254 {
		t.Fatal("ids changed")
	}

	// A ground loses its group and speed when the client item is no ground.
	ground := file.Items[0]
	ground.Reload(thing.New(100, thing.CategoryItem), nil)
	if ground.Group != GroupNone || ground.Attr(AttrGroundSpeed) != nil {
		t.Fatalf("ground kept %d %v", ground.Group, ground.Attr(AttrGroundSpeed))
	}
}

func TestSpriteHashEmpty(t *testing.T) {
	group := thing.NewFrameGroup()
	group.Sprites = []uint32{0}
	got, err := SpriteHash(group, false, func(int) ([]byte, error) { t.Fatal("empty sprite read"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if want := md5.Sum(bytes.Repeat([]byte{0x11, 0x11, 0x11, 0}, 32*32)); got != want {
		t.Fatalf("hash %x, want %x", got, want)
	}
}

func TestSpriteHashPixels(t *testing.T) {
	group := thing.NewFrameGroup()
	group.Sprites = []uint32{7}
	// One red pixel at (0, 0), the rest transparent.
	data := []byte{0, 0, 1, 0, 0xFF, 0, 0}
	got, err := SpriteHash(group, false, func(int) ([]byte, error) { return data, nil })
	if err != nil {
		t.Fatal(err)
	}
	pixels := bytes.Repeat([]byte{0x11, 0x11, 0x11, 0}, 32*32)
	// Rows are stored bottom first and pixels as BGR0.
	copy(pixels[31*32*4:], []byte{0, 0, 0xFF, 0})
	if want := md5.Sum(pixels); got != want {
		t.Fatalf("hash %x, want %x", got, want)
	}
}

func TestNewVersion(t *testing.T) {
	file := New(3, 60, 1)
	if major, minor, build := file.Version(); major != 3 || minor != 60 || build != 1 {
		t.Fatalf("version %d.%d.%d", major, minor, build)
	}
	data, err := file.Encode()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, minor, _ := parsed.Version(); minor != 60 || len(parsed.Items) != 0 {
		t.Fatalf("parsed minor %d, %d items", minor, len(parsed.Items))
	}
	description := parsed.rootAttrs[0].Data[12:]
	if len(description) != versionDescriptionSize || !bytes.HasPrefix(description, []byte("OTB 3.60.1\x00")) {
		t.Fatalf("description %q", description)
	}
}

func TestNewItemCloneSetClientID(t *testing.T) {
	item := NewItem(100, 200)
	if item.ServerID() != 100 || item.ClientID() != 200 || item.Flags != FlagMovable {
		t.Fatalf("new item %+v", item)
	}
	clone := item.Clone()
	if !clone.Equal(item) {
		t.Fatal("clone differs")
	}
	clone.SetClientID(300)
	if item.ClientID() != 200 || clone.ClientID() != 300 || clone.Equal(item) {
		t.Fatalf("clone shares data: %d %d", item.ClientID(), clone.ClientID())
	}
	clone.Attrs[0].Data[0] = 0
	if item.ServerID() != 100 {
		t.Fatal("clone shares attribute bytes")
	}

	// SetClientID adds the attribute when it is missing.
	deprecated := &Item{Group: GroupDeprecated}
	deprecated.SetClientID(5)
	if deprecated.ClientID() != 5 || len(deprecated.Attrs) != 1 {
		t.Fatalf("attrs %+v", deprecated.Attrs)
	}
}

func TestReloadGroupsAndAttrs(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(props *thing.Properties)
		group Group
	}{
		{"ground", func(props *thing.Properties) { props.Ground = true }, GroupGround},
		{"container", func(props *thing.Properties) { props.Container = true }, GroupContainer},
		{"fluid container", func(props *thing.Properties) { props.FluidContainer = true }, GroupFluid},
		{"splash", func(props *thing.Properties) { props.Fluid = true }, GroupSplash},
	} {
		item := NewItem(1, 1)
		client := thing.New(1, thing.CategoryItem)
		test.setup(&client.Props)
		item.Reload(client, nil)
		if item.Group != test.group {
			t.Errorf("%s: group %d, want %d", test.name, item.Group, test.group)
		}
	}

	item := NewItem(1, 1)
	item.Attrs = append(item.Attrs, Attr{0x99, []byte{1}})
	client := thing.New(1, thing.CategoryItem)
	client.Name = "book"
	client.Props.Unmoveable = true
	client.Props.LensHelp, client.Props.LensHelpValue = true, readableLensHelp
	client.Props.GroundBorder = true
	client.Props.MiniMap, client.Props.MiniMapColor = true, 7
	client.Props.Writable, client.Props.MaxTextLength = true, 100
	client.Props.IsMarket, client.Props.Market.TradeAs = true, 9
	item.Reload(client, nil)
	if item.Flags != FlagReadable|FlagStackOrder {
		t.Fatalf("flags %b", item.Flags)
	}
	var order []byte
	for _, attr := range item.Attrs {
		order = append(order, attr.Type)
	}
	want := []byte{AttrServerID, AttrClientID, AttrMinimapColor, AttrMaxReadWriteChars, AttrStackOrder, AttrTradeAs, AttrName, 0x99}
	if !bytes.Equal(order, want) {
		t.Fatalf("attribute order %x, want %x", order, want)
	}
	if data := item.Attr(AttrStackOrder); data[0] != 1 {
		t.Fatalf("stack order %v", data)
	}
	if string(item.Attr(AttrName)) != "book" {
		t.Fatalf("name %q", item.Attr(AttrName))
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte{1, 2}); err == nil {
		t.Fatal("tiny file parsed")
	}
	build := func(writeItem func(writer *otbnode.Writer)) []byte {
		var buf bytes.Buffer
		writer := otbnode.NewWriter(&buf, [4]byte{})
		writer.Start(0)
		writer.U32(0)
		writer.Start(1)
		writeItem(writer)
		writer.End()
		writer.End()
		if err := writer.Flush(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	withoutFlags := build(func(writer *otbnode.Writer) { writer.U16(0) })
	withChild := build(func(writer *otbnode.Writer) {
		writer.U32(0)
		writer.Start(2)
		writer.End()
	})
	for name, data := range map[string][]byte{"without flags": withoutFlags, "with child": withChild} {
		if _, err := Parse(data); err == nil {
			t.Errorf("item %s parsed", name)
		}
	}
}

func TestTail(t *testing.T) {
	attrs, tail := readAttrs([]byte{AttrServerID, 2, 0, 5, 0, AttrName, 9})
	if len(attrs) != 1 || !bytes.Equal(tail, []byte{AttrName, 9}) {
		t.Fatalf("attrs %+v tail %v", attrs, tail)
	}
}
