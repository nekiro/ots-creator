package itemsotb

import (
	"bytes"
	"crypto/md5"
	"testing"

	"github.com/nekiro/ots-creator/internal/otbnode"
	"github.com/nekiro/ots-creator/internal/thing"
)

// sample builds an items.otb by hand, with an unknown attribute, a
// deprecated item and server-only flags.
func sample(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := otbnode.NewWriter(&buf, [4]byte{})
	w.Start(0)
	w.U32(0)
	w.U8(rootAttrVersion)
	w.U16(140)
	w.U32(3)
	w.U32(57)
	w.U32(62)
	w.Bytes(make([]byte, 128))
	item := func(group Group, flags uint32, server, client uint16, extra ...byte) {
		w.Start(byte(group))
		w.U32(flags)
		w.U8(AttrServerID)
		w.U16(2)
		w.U16(server)
		if client != 0 {
			w.U8(AttrClientID)
			w.U16(2)
			w.U16(client)
		}
		w.Bytes(extra)
		w.End()
	}
	item(GroupGround, FlagMovable, 100, 100, AttrGroundSpeed, 2, 0, 150, 0)
	item(GroupDeprecated, 0, 101, 0)
	// 13 = door; 1<<8 floor change down; 0x99 unknown; 0xFE needs escaping.
	item(13, FlagUnpassable|1<<8, 102, 254, 0x99, 1, 0, 0xFE)
	w.End()
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoundTrip(t *testing.T) {
	data := sample(t)
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if major, minor, build := f.Version(); major != 3 || minor != 57 || build != 62 {
		t.Fatalf("version %d.%d.%d", major, minor, build)
	}
	if len(f.Items) != 3 || f.Items[2].ServerID() != 102 || f.Items[2].ClientID() != 254 || !f.Items[1].Deprecated() {
		t.Fatalf("items %+v", f.Items)
	}
	out, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatal("unchanged file does not encode to the same bytes")
	}
}

func TestReloadKeepsServerData(t *testing.T) {
	f, err := Parse(sample(t))
	if err != nil {
		t.Fatal(err)
	}
	it := f.Items[2]
	th := thing.New(254, thing.CategoryItem)
	th.Props.Pickupable = true
	th.Props.OnTop = true
	th.Props.HasLight, th.Props.LightLevel, th.Props.LightColor = true, 3, 215
	th.FrameGroups[0].Frames = 2
	h := [16]byte{1, 2, 3}
	it.Reload(th, &h)
	want := uint32(FlagPickupable | FlagMovable | FlagStackOrder | FlagIsAnimation | 1<<8)
	if it.Flags != want {
		t.Fatalf("flags %b, want %b", it.Flags, want)
	}
	if it.Group != 13 {
		t.Fatalf("server group changed to %d", it.Group)
	}
	if d := it.Attr(AttrStackOrder); len(d) != 1 || d[0] != 3 {
		t.Fatalf("stack order %v", d)
	}
	if d := it.Attr(AttrLight); !bytes.Equal(d, []byte{3, 0, 215, 0}) {
		t.Fatalf("light %v", d)
	}
	if d := it.Attr(0x99); !bytes.Equal(d, []byte{0xFE}) {
		t.Fatalf("unknown attribute %v", d)
	}
	if d := it.Attr(AttrSpriteHash); !bytes.Equal(d, h[:]) {
		t.Fatalf("hash %v", d)
	}
	if it.ServerID() != 102 || it.ClientID() != 254 {
		t.Fatal("ids changed")
	}

	// A ground loses its group and speed when the client item is no ground.
	g := f.Items[0]
	g.Reload(thing.New(100, thing.CategoryItem), nil)
	if g.Group != GroupNone || g.Attr(AttrGroundSpeed) != nil {
		t.Fatalf("ground kept %d %v", g.Group, g.Attr(AttrGroundSpeed))
	}
}

func TestSpriteHashEmpty(t *testing.T) {
	g := thing.NewFrameGroup()
	g.Sprites = []uint32{0}
	got, err := SpriteHash(g, false, func(int) ([]byte, error) { t.Fatal("empty sprite read"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if want := md5.Sum(bytes.Repeat([]byte{0x11, 0x11, 0x11, 0}, 32*32)); got != want {
		t.Fatalf("hash %x, want %x", got, want)
	}
}

func TestSpriteHashPixels(t *testing.T) {
	g := thing.NewFrameGroup()
	g.Sprites = []uint32{7}
	// One red pixel at (0, 0), the rest transparent.
	data := []byte{0, 0, 1, 0, 0xFF, 0, 0}
	got, err := SpriteHash(g, false, func(int) ([]byte, error) { return data, nil })
	if err != nil {
		t.Fatal(err)
	}
	px := bytes.Repeat([]byte{0x11, 0x11, 0x11, 0}, 32*32)
	// Rows are stored bottom first and pixels as BGR0.
	copy(px[31*32*4:], []byte{0, 0, 0xFF, 0})
	if want := md5.Sum(px); got != want {
		t.Fatalf("hash %x, want %x", got, want)
	}
}
