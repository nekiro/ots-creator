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

	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/thing"
)

// sprite returns a 32x32 sprite with one pixel of color c at (i%32, i/32);
// with noisy every pixel gets its own color (more than 256 in a sheet).
func sprite(i int, c [4]byte, noisy bool) []byte {
	px := make([]byte, 32*32*4)
	if noisy {
		for k := 0; k < 32*32; k++ {
			px[k*4], px[k*4+1], px[k*4+2], px[k*4+3] = byte(k), byte(k>>8), byte(i), 255
		}
		return px
	}
	copy(px[(i%(32*32))*4:], c[:])
	return px
}

func outfit(noisy bool) *obd.Data {
	t := thing.New(128, thing.CategoryOutfit)
	t.Name = "citizen"
	t.Props.HasLight, t.Props.LightLevel, t.Props.LightColor = true, 3, 215
	t.Props.IsMarket, t.Props.Market = true, thing.Market{Name: "x", Category: 1, RestrictProfession: 9}
	t.NpcSales = []thing.NpcSale{{Name: "Rashid", BuyPrice: 150}}
	idle := t.FrameGroups[0]
	idle.Layers = 2
	idle.Sprites = make([]uint32, idle.TotalSprites())
	walk := idle.Clone()
	walk.Type = thing.FrameGroupWalking
	walk.Resize(1, 1, 2, 4, 1, 1, 3, 100)
	walk.Mode, walk.LoopCount, walk.StartFrame = thing.AnimationSync, -1, 1
	walk.Durations[2] = thing.FrameDuration{Min: 50, Max: 70}
	t.FrameGroups = append(t.FrameGroups, walk)
	d := &obd.Data{Version: obd.Version3, ClientVersion: 1098, Thing: t, SpriteSize: 32}
	n := 0
	for _, g := range t.FrameGroups {
		var list []obd.Sprite
		for range g.Sprites {
			list = append(list, obd.Sprite{Pixels: sprite(n, [4]byte{255, 0, 255, 255}, noisy)}) // magenta stays a color
			n++
		}
		d.Sprites = append(d.Sprites, list)
	}
	return d
}

func TestRoundTrip(t *testing.T) {
	for _, noisy := range []bool{false, true} {
		d := outfit(noisy)
		file, err := Encode(d, Options{Generator: "test"})
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(file)
		if err != nil {
			t.Fatal(err)
		}
		gt, wt := got.Thing, d.Thing
		if gt.ID != 128 || got.ClientVersion != 1098 || gt.Name != "citizen" || gt.Props != wt.Props || !reflect.DeepEqual(gt.NpcSales, wt.NpcSales) {
			t.Fatalf("thing %+v", gt)
		}
		for gi, g := range wt.FrameGroups {
			x, y := *gt.FrameGroups[gi], *g
			x.Sprites, y.Sprites = nil, nil
			if !reflect.DeepEqual(x, y) {
				t.Fatalf("group %d: %+v, want %+v", gi, x, y)
			}
			for i, s := range d.Sprites[gi] {
				if !bytes.Equal(got.Sprites[gi][i].Pixels, s.Pixels) {
					t.Fatalf("group %d sprite %d differs (noisy %v)", gi, i, noisy)
				}
			}
		}
	}
}

func TestManifestIsReadable(t *testing.T) {
	file, _ := Encode(outfit(false), Options{})
	m := manifestOf(t, file)
	flags := map[string]any{}
	json.Unmarshal(m.Flags, &flags)
	// Only set flags are written, with their payload.
	if len(flags) != 5 || flags["lightColor"] != 215.0 || flags["market"] == nil {
		t.Fatalf("flags %v", flags)
	}
	if m.FrameGroups[1].Type != "walking" || m.FrameGroups[1].Animation.Mode != "sync" || m.FrameGroups[0].Animation != nil {
		t.Fatalf("groups %+v", m.FrameGroups)
	}
}

func manifestOf(t *testing.T, file []byte) manifest {
	t.Helper()
	z, _ := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	data, err := readEntry(z, manifestName)
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
	z, _ := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range z.File {
		r, _ := f.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		if f.Name == manifestName {
			var m map[string]any
			json.Unmarshal(data, &m)
			edit(m)
			data, _ = json.Marshal(m)
		}
		e, _ := w.Create(f.Name)
		e.Write(data)
	}
	for name, content := range extra {
		e, _ := w.Create(name)
		e.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestForwardCompatible(t *testing.T) {
	file, _ := Encode(outfit(false), Options{})
	file = rewrite(t, file, func(m map[string]any) {
		m["futureKey"] = 1
		m["flags"].(map[string]any)["futureFlag"] = true
	}, map[string]string{"preview.png": "x"})
	if _, err := Decode(file); err != nil {
		t.Fatalf("unknown keys and entries must be ignored: %v", err)
	}
}

func TestBadFiles(t *testing.T) {
	good, _ := Encode(outfit(false), Options{})
	for name, file := range map[string][]byte{
		"not a zip":     []byte("hello"),
		"other format":  rewrite(t, good, func(m map[string]any) { m["format"] = "obd" }, nil),
		"newer version": rewrite(t, good, func(m map[string]any) { m["version"] = 2 }, nil),
		"bad category":  rewrite(t, good, func(m map[string]any) { m["category"] = "npc" }, nil),
		"sprite size":   rewrite(t, good, func(m map[string]any) { m["spriteSize"] = 16 }, nil),
		"missing sheet": rewrite(t, good, func(m map[string]any) { m["frameGroups"].([]any)[0].(map[string]any)["sheet"] = "sprites/none.png" }, nil),
		"sheet escape":  rewrite(t, good, func(m map[string]any) { m["frameGroups"].([]any)[0].(map[string]any)["sheet"] = "../x.png" }, nil),
		"wrong size":    rewrite(t, good, func(m map[string]any) { m["frameGroups"].([]any)[0].(map[string]any)["width"] = 2 }, nil),
		"too many":      rewrite(t, good, func(m map[string]any) { m["frameGroups"].([]any)[0].(map[string]any)["frames"] = 255 }, nil),
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
		z, _ := zip.NewReader(bytes.NewReader(file), int64(len(file)))
		f, _ := z.Open("sprites/idle.png")
		data, _ := io.ReadAll(f)
		f.Close()
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
