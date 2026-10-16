package market

import (
	"image"
	"image/color"
	"testing"
)

func TestHSIToRGB(t *testing.T) {
	cases := map[int][3]uint8{
		0:   {255, 255, 255}, // white column
		-1:  {255, 255, 255}, // out of range is color 0
		133: {255, 255, 255},
		114: {36, 36, 36}, // the darkest gray
	}
	for index, want := range cases {
		if got := hsiToRGB(index); got != want {
			t.Errorf("hsiToRGB(%d) = %v, want %v", index, got, want)
		}
	}
	// Every palette entry is valid and colored ones are not gray.
	for index := 0; index < 19*7; index++ {
		rgb := hsiToRGB(index)
		if index%19 != 0 && rgb[0] == rgb[1] && rgb[1] == rgb[2] {
			t.Errorf("color %d is gray: %v", index, rgb)
		}
	}
}

func TestWebPalette(t *testing.T) {
	colors := webPalette()
	if len(colors) != 256 || colors[0] != transparent {
		t.Fatalf("palette of %d colors", len(colors))
	}
	if colors[216] != (color.NRGBA{255, 255, 255, 255}) || colors[217] != (color.NRGBA{0, 0, 0, 255}) {
		t.Fatalf("palette edges %v %v", colors[216], colors[217])
	}
}

func TestPaletteFallsBackToWeb(t *testing.T) {
	frame := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < len(frame.Pix); i += 4 {
		frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2], frame.Pix[i+3] = byte(i/4), byte(i/1024), 0, 255
	}
	colors, exact := palette([]*image.NRGBA{frame}, frame.Rect)
	if exact || len(colors) != 256 {
		t.Fatalf("exact %v, %d colors", exact, len(colors))
	}
	if opaque(color.NRGBA{1, 2, 3, 100}) != transparent || opaque(color.NRGBA{1, 2, 3, 200}).A != 255 {
		t.Fatal("opaque")
	}
}

func TestPackImageRoundTrip(t *testing.T) {
	sprites := make([][]byte, 10)
	for i := range sprites {
		sprites[i] = pixels(byte(i*20), 1)
	}
	img, err := PackImage(sprites, 32)
	if err != nil {
		t.Fatal(err)
	}
	if img.Rect.Dx() != packColumns*32 || img.Rect.Dy() != 2*32 {
		t.Fatalf("image %v", img.Rect)
	}
	if _, err := PackImage(nil, 32); err == nil {
		t.Fatal("no sprites must fail")
	}
	if _, err := PackImage(make([][]byte, MaxPackSprites+1), 32); err == nil {
		t.Fatal("too many sprites must fail")
	}
}
