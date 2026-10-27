package obd

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/thing"
)

func pixels(seed byte) []byte {
	pixelData := make([]byte, 32*32*4)
	for i := 0; i < len(pixelData); i += 4 {
		pixelData[i], pixelData[i+1], pixelData[i+2], pixelData[i+3] = seed, byte(i), byte(i>>8), 255
	}
	return pixelData
}

func sampleOutfit() *Data {
	outfit := thing.New(0, thing.CategoryOutfit)
	outfit.Props.HasLight, outfit.Props.LightLevel, outfit.Props.LightColor = true, 3, 200
	outfit.Props.AnimateAlways = true
	idle := thing.NewFrameGroup()
	idle.PatternX = 4
	idle.Sprites = []uint32{1, 2, 3, 4}
	walk := thing.NewFrameGroup()
	walk.Type = 1
	walk.PatternX, walk.Frames = 4, 2
	walk.Sprites = []uint32{5, 6, 7, 8, 9, 10, 11, 12}
	walk.Durations = []thing.FrameDuration{{Min: 100, Max: 150}, {Min: 200, Max: 250}}
	walk.LoopCount, walk.StartFrame, walk.Mode = 0, 0, thing.AnimationSync
	outfit.FrameGroups = []*thing.FrameGroup{idle, walk}
	data := &Data{Version: Version3, ClientVersion: 1098, Thing: outfit, SpriteSize: 32}
	for _, group := range outfit.FrameGroups {
		var sprites []Sprite
		for _, id := range group.Sprites {
			sprites = append(sprites, Sprite{ID: id, Pixels: pixels(byte(id))})
		}
		data.Sprites = append(data.Sprites, sprites)
	}
	return data
}

func TestV3RoundTripOutfitGroups(t *testing.T) {
	src := sampleOutfit()
	file, err := Encode(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version3 || got.ClientVersion != 1098 || got.SpriteSize != 32 {
		t.Fatalf("header %+v", got)
	}
	src.Thing.FrameGroups[0].Type = 0 // written as index 0
	if !reflect.DeepEqual(got.Thing, src.Thing) {
		t.Fatalf("thing mismatch\n got %+v\nwant %+v", got.Thing.FrameGroups[1], src.Thing.FrameGroups[1])
	}
	if !reflect.DeepEqual(got.Sprites, src.Sprites) {
		t.Fatal("sprites mismatch")
	}
}

func TestV2RoundTripKeepsFirstGroup(t *testing.T) {
	src := sampleOutfit()
	src.Version = Version2
	file, err := Encode(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Thing.FrameGroups) != 1 || len(got.Sprites) != 1 || len(got.Sprites[0]) != 4 {
		t.Fatalf("groups = %d", len(got.Thing.FrameGroups))
	}
	if !reflect.DeepEqual(got.Sprites[0], src.Sprites[0]) {
		t.Fatal("pixels differ")
	}
	if !got.Thing.Props.HasLight || got.Thing.Props.LightColor != 200 {
		t.Fatal("props lost")
	}
}

func TestV1RoundTrip(t *testing.T) {
	item := thing.New(0, thing.CategoryItem)
	item.Props.Pickupable, item.Props.Stackable = true, true
	group := item.FrameGroups[0]
	group.Frames = 3
	group.Sprites = []uint32{10, 11, 12}
	group.EnsureDurations(500)
	data := &Data{Version: Version1, ClientVersion: 860, Thing: item, Sprites: [][]Sprite{{{10, pixels(1)}, {11, pixels(2)}, {12, pixels(3)}}}}
	file, err := Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version1 || got.ClientVersion != 860 || got.Thing.Category != thing.CategoryItem {
		t.Fatalf("header %+v", got)
	}
	if !reflect.DeepEqual(got.Thing.FrameGroups[0], group) {
		t.Fatalf("group %+v", got.Thing.FrameGroups[0])
	}
	if !reflect.DeepEqual(got.Sprites, data.Sprites) {
		t.Fatal("pixels differ")
	}
}

func TestRawLayoutV2(t *testing.T) {
	effect := thing.New(0, thing.CategoryEffect)
	data := &Data{Version: Version2, ClientVersion: 1098, Thing: effect, Sprites: [][]Sprite{{{0, pixels(9)}}}}
	raw, err := EncodeRaw(data)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(raw) != 200 || binary.LittleEndian.Uint16(raw[2:]) != 1098 || raw[4] != 3 {
		t.Fatalf("header %v", raw[:5])
	}
	texturePos := binary.LittleEndian.Uint32(raw[5:])
	if texturePos != 10 || raw[9] != 0xFF {
		t.Fatalf("texture position %d, flags end %x", texturePos, raw[9])
	}
	// w h layers px py pz frames, id, 4096 bytes of ARGB
	if len(raw) != 10+7+4+4096 {
		t.Fatalf("len %d", len(raw))
	}
	argb := raw[10+7+4:]
	if argb[0] != 255 || argb[1] != 9 {
		t.Fatalf("pixel not ARGB: %v", argb[:4])
	}
}

func TestLZMAHeaderIsLZMAAlone(t *testing.T) {
	file, err := Encode(sampleOutfit())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := EncodeRaw(sampleOutfit())
	if file[0] != 0x5D {
		t.Fatalf("properties byte %x", file[0])
	}
	if size := binary.LittleEndian.Uint64(file[5:13]); size != uint64(len(raw)) {
		t.Fatalf("size in header %d, want %d", size, len(raw))
	}
}

func TestDecodeErrors(t *testing.T) {
	if _, err := Decode([]byte("not lzma")); err == nil {
		t.Fatal("garbage must fail")
	}
	if _, err := DecodeRaw([]byte{5, 0}); err == nil || !strings.Contains(err.Error(), "unknown version") {
		t.Fatalf("err = %v", err)
	}
	raw, _ := EncodeRaw(sampleOutfit())
	for _, cut := range []int{6, 20, len(raw) - 10} {
		if _, err := DecodeRaw(raw[:cut]); err == nil {
			t.Fatalf("truncated at %d must fail", cut)
		}
	}
	bad := bytes.Clone(raw)
	bad[4] = 9
	if _, err := DecodeRaw(bad); err == nil {
		t.Fatal("bad category must fail")
	}
}

func TestEncodeValidates(t *testing.T) {
	data := sampleOutfit()
	data.Sprites[1] = data.Sprites[1][:3]
	if _, err := Encode(data); err == nil {
		t.Fatal("sprite count mismatch must fail")
	}
	data = sampleOutfit()
	data.Sprites[0][0].Pixels = data.Sprites[0][0].Pixels[:10]
	if _, err := Encode(data); err == nil {
		t.Fatal("pixel size mismatch must fail")
	}
}
