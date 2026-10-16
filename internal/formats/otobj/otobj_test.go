package otobj

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
)

// sprite returns a 32x32 sprite with one pixel of color c at (i%32, i/32);
// with noisy every pixel gets its own color (more than 256 in a sheet).
func sprite(i int, base [4]byte, noisy bool) []byte {
	pixels := make([]byte, 32*32*4)
	if noisy {
		for pixel := 0; pixel < 32*32; pixel++ {
			pixels[pixel*4], pixels[pixel*4+1], pixels[pixel*4+2], pixels[pixel*4+3] = byte(pixel), byte(pixel>>8), byte(i), 255
		}
		return pixels
	}
	copy(pixels[(i%(32*32))*4:], base[:])
	return pixels
}

func outfit(noisy bool) *obd.Data {
	object := thing.New(128, thing.CategoryOutfit)
	object.Name = "citizen"
	object.Props.HasLight, object.Props.LightLevel, object.Props.LightColor = true, 3, 215
	object.Props.IsMarket, object.Props.Market = true, thing.Market{Name: "x", Category: 1, RestrictProfession: 9}
	object.NpcSales = []thing.NpcSale{{Name: "Rashid", BuyPrice: 150}}
	idle := object.FrameGroups[0]
	idle.Layers = 2
	idle.Sprites = make([]uint32, idle.TotalSprites())
	walk := idle.Clone()
	walk.Type = thing.FrameGroupWalking
	walk.Resize(1, 1, 2, 4, 1, 1, 3, 100)
	walk.Mode, walk.LoopCount, walk.StartFrame = thing.AnimationSync, -1, 1
	walk.Durations[2] = thing.FrameDuration{Min: 50, Max: 70}
	object.FrameGroups = append(object.FrameGroups, walk)
	data := &obd.Data{Version: obd.Version3, ClientVersion: 1098, Thing: object, SpriteSize: 32}
	spriteIndex := 0
	for _, group := range object.FrameGroups {
		var list []obd.Sprite
		for range group.Sprites {
			list = append(list, obd.Sprite{Pixels: sprite(spriteIndex, [4]byte{255, 0, 255, 255}, noisy)}) // magenta stays a color
			spriteIndex++
		}
		data.Sprites = append(data.Sprites, list)
	}
	return data
}

func TestRoundTrip(t *testing.T) {
	for _, noisy := range []bool{false, true} {
		want := outfit(noisy)
		file, err := Encode(want, Options{Generator: "test"})
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(file)
		if err != nil {
			t.Fatal(err)
		}
		gotThing, wantThing := got.Thing, want.Thing
		if gotThing.ID != 128 || got.ClientVersion != 1098 || gotThing.Name != "citizen" || gotThing.Props != wantThing.Props || !reflect.DeepEqual(gotThing.NpcSales, wantThing.NpcSales) {
			t.Fatalf("thing %+v", gotThing)
		}
		for groupIndex, wantGroup := range wantThing.FrameGroups {
			gotLayout, wantLayout := *gotThing.FrameGroups[groupIndex], *wantGroup
			gotLayout.Sprites, wantLayout.Sprites = nil, nil
			if !reflect.DeepEqual(gotLayout, wantLayout) {
				t.Fatalf("group %d: %+v, want %+v", groupIndex, gotLayout, wantLayout)
			}
			for i, sprite := range want.Sprites[groupIndex] {
				if !bytes.Equal(got.Sprites[groupIndex][i].Pixels, sprite.Pixels) {
					t.Fatalf("group %d sprite %d differs (noisy %v)", groupIndex, i, noisy)
				}
			}
		}
	}
}

func TestManifestIsReadable(t *testing.T) {
	file, _ := Encode(outfit(false), Options{})
	meta := manifestOf(t, file)
	flags := map[string]any{}
	json.Unmarshal(meta.Flags, &flags)
	// Only set flags are written, with their payload.
	if len(flags) != 5 || flags["lightColor"] != 215.0 || flags["market"] == nil {
		t.Fatalf("flags %v", flags)
	}
	if meta.FrameGroups[1].Type != "walking" || meta.FrameGroups[1].Animation.Mode != "sync" || meta.FrameGroups[0].Animation != nil {
		t.Fatalf("groups %+v", meta.FrameGroups)
	}
}

func manifestOf(t *testing.T, file []byte) manifest {
	t.Helper()
	archive, _ := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	data, err := readEntry(archive, manifestName)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// rewrite returns file with the manifest changed by edit and extra entries.
func rewrite(t *testing.T, file []byte, edit func(m map[string]any), extra map[string]string) []byte {
	t.Helper()
	archive, _ := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range archive.File {
		reader, _ := entry.Open()
		data, _ := io.ReadAll(reader)
		reader.Close()
		if entry.Name == manifestName {
			var fields map[string]any
			json.Unmarshal(data, &fields)
			edit(fields)
			data, _ = json.Marshal(fields)
		}
		target, _ := writer.Create(entry.Name)
		target.Write(data)
	}
	for name, content := range extra {
		e, _ := writer.Create(name)
		e.Write([]byte(content))
	}
	writer.Close()
	return buf.Bytes()
}

func TestForwardCompatible(t *testing.T) {
	file, _ := Encode(outfit(false), Options{})
	file = rewrite(t, file, func(fields map[string]any) {
		fields["futureKey"] = 1
		fields["flags"].(map[string]any)["futureFlag"] = true
	}, map[string]string{"preview.png": "x"})
	if _, err := Decode(file); err != nil {
		t.Fatalf("unknown keys and entries must be ignored: %v", err)
	}
}

func TestBadFiles(t *testing.T) {
	good, _ := Encode(outfit(false), Options{})
	for name, file := range map[string][]byte{
		"not a zip":     []byte("hello"),
		"other format":  rewrite(t, good, func(fields map[string]any) { fields["format"] = "obd" }, nil),
		"newer version": rewrite(t, good, func(fields map[string]any) { fields["version"] = 2 }, nil),
		"bad category":  rewrite(t, good, func(fields map[string]any) { fields["category"] = "npc" }, nil),
		"sprite size":   rewrite(t, good, func(fields map[string]any) { fields["spriteSize"] = 16 }, nil),
		"missing sheet": rewrite(t, good, func(fields map[string]any) {
			fields["frameGroups"].([]any)[0].(map[string]any)["sheet"] = "sprites/none.png"
		}, nil),
		"sheet escape": rewrite(t, good, func(fields map[string]any) { fields["frameGroups"].([]any)[0].(map[string]any)["sheet"] = "../x.png" }, nil),
		"wrong size":   rewrite(t, good, func(fields map[string]any) { fields["frameGroups"].([]any)[0].(map[string]any)["width"] = 2 }, nil),
		"too many":     rewrite(t, good, func(fields map[string]any) { fields["frameGroups"].([]any)[0].(map[string]any)["frames"] = 255 }, nil),
	} {
		_, err := Decode(file)
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
		if name == "not a zip" && !errors.Is(err, ErrNotOTOBJ) {
			t.Errorf("not a zip: %v", err)
		}
		if name == "newer version" && !strings.Contains(err.Error(), "update") {
			t.Errorf("newer version: %v", err)
		}
	}
}

func TestPalette(t *testing.T) {
	for _, noisy := range []bool{false, true} {
		file, _ := Encode(outfit(noisy), Options{})
		archive, _ := zip.NewReader(bytes.NewReader(file), int64(len(file)))
		sheet, _ := archive.Open("sprites/idle.png")
		data, _ := io.ReadAll(sheet)
		sheet.Close()
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		// Up to 256 colors: a palette; more: RGBA.
		if _, paletted := img.(*image.Paletted); paletted == noisy {
			t.Fatalf("noisy %v: sheet type %T", noisy, img)
		}
	}
}
