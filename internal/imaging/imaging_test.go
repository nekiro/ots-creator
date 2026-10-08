package imaging

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/nekiro/ots-creator/internal/thing"
)

func solid(v byte, size int) []byte {
	return bytes.Repeat([]byte{v, v, v, 255}, size*size)
}

func TestSheetRoundTrip(t *testing.T) {
	g := &thing.FrameGroup{Width: 2, Height: 1, Layers: 2, PatternX: 2, PatternY: 2, PatternZ: 1, Frames: 2}
	g.Sprites = make([]uint32, g.TotalSprites())
	src := make([][]byte, g.TotalSprites())
	for i := range src {
		if i%5 == 0 {
			continue // empty slot
		}
		src[i] = solid(byte(i+1), 32)
	}
	img := Sheet(g, 32, Magenta, func(slot int) []byte { return src[slot] })
	w, h := g.SheetSize(32)
	if img.Rect.Dx() != w || img.Rect.Dy() != h {
		t.Fatalf("sheet %v", img.Rect)
	}
	// Empty slot 0 is magenta in the sheet.
	if c := img.NRGBAAt(32+5, 5); c != Magenta {
		t.Fatalf("slot 0 pixel %v", c)
	}
	got, err := SliceSheet(img, g, 32)
	if err != nil {
		t.Fatal(err)
	}
	empty := make([]byte, 32*32*4)
	for i := range src {
		want := src[i]
		if want == nil {
			want = empty
		}
		if !bytes.Equal(got[i], want) {
			t.Fatalf("slot %d differs", i)
		}
	}
}

func TestSheetLayoutMatchesObjectBuilder(t *testing.T) {
	// 2x1 tiles: sprite w=0 is drawn on the right.
	g := &thing.FrameGroup{Width: 2, Height: 1, Layers: 1, PatternX: 1, PatternY: 1, PatternZ: 1, Frames: 1}
	g.Sprites = make([]uint32, 2)
	img := Sheet(g, 32, color.NRGBA{}, func(slot int) []byte { return solid(byte(10+slot), 32) })
	if img.NRGBAAt(40, 0).R != 10 || img.NRGBAAt(0, 0).R != 11 {
		t.Fatal("w=0 must be the right tile")
	}
}

func TestSliceSheetSizeMismatch(t *testing.T) {
	g := thing.NewFrameGroup()
	if _, err := SliceSheet(image.NewNRGBA(image.Rect(0, 0, 10, 10)), g, 32); err == nil {
		t.Fatal("expected error")
	}
}

func TestSliceTilesPadsEdges(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 40, 32))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	tiles := SliceTiles(img, 32)
	if len(tiles) != 2 {
		t.Fatalf("tiles %d", len(tiles))
	}
	// Second tile: first 8 columns filled, rest transparent.
	second := tiles[1]
	if second[0] != 200 || second[8*4+3] != 0 {
		t.Fatal("padding wrong")
	}
}

func TestRemoveMagentaAndNormalize(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	copy(img.Pix, []byte{255, 0, 255, 255, 9, 9, 9, 0})
	RemoveMagenta(img)
	Normalize(img)
	for _, b := range img.Pix {
		if b != 0 {
			t.Fatalf("pix %v", img.Pix)
		}
	}
}

func TestEncodeDecodePNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	img.Pix[0], img.Pix[3] = 7, 255
	data, err := EncodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(bytes.NewReader(data))
	if err != nil || !bytes.Equal(back.Pix, img.Pix) {
		t.Fatal("png round trip")
	}
}

func TestPutTile(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	PutTile(img, 32, 0, 32, solid(5, 32))
	if !bytes.Equal(Tile(img, 32, 0, 32), solid(5, 32)) {
		t.Fatal("PutTile/Tile mismatch")
	}
}

func TestEncodeFormats(t *testing.T) {
	// Left half opaque red, right half transparent; JPEG blurs the border,
	// so only the far corners are checked.
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 8 {
			copy(img.Pix[img.PixOffset(x, y):], []byte{255, 0, 0, 255})
		}
	}
	last := img.PixOffset(15, 15)
	for _, f := range Formats {
		data, err := Encode(img, f, Magenta)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		back, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s decode: %v", f, err)
		}
		if f == "png" {
			if back.Pix[last+3] != 0 {
				t.Fatal("png must keep alpha")
			}
			continue
		}
		// The transparent pixel is flattened onto magenta.
		if r, g, b := back.Pix[last], back.Pix[last+1], back.Pix[last+2]; r < 240 || g > 15 || b < 240 {
			t.Fatalf("%s background %v", f, back.Pix[last:last+4])
		}
	}
	if FormatOf("a/B.JPEG") != "jpg" || FormatOf("x.bmp") != "bmp" || FormatOf("x") != "png" {
		t.Fatal("FormatOf")
	}
}

func TestCropToContent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	img.SetNRGBA(40, 50, color.NRGBA{R: 1, A: 255})
	img.SetNRGBA(60, 62, color.NRGBA{G: 1, A: 10})
	got := CropToContent(img)
	if got.Rect.Dx() != 21 || got.Rect.Dy() != 13 || got.Pix[3] != 255 {
		t.Fatalf("crop %v", got.Rect)
	}
	empty := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	if CropToContent(empty) != empty {
		t.Fatal("empty image must stay")
	}
}
