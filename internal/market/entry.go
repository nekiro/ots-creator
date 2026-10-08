package market

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/png"
	"slices"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Info is what the app reads from an entry's file when sharing it; the
// market shows it without decoding the file.
type Info struct {
	ClientVersion uint16 `json:"clientVersion,omitempty"`
	// Width and Height are in tiles (objects) or sprites (packs).
	Width   int `json:"width"`
	Height  int `json:"height"`
	Frames  int `json:"frames,omitempty"`
	Sprites int `json:"sprites"`
}

// ReadObject decodes an object entry file and checks it against meta.
func ReadObject(m *Meta, data []byte) (*obd.Data, Info, error) {
	if len(data) > MaxObjectBytes {
		return nil, Info{}, fmt.Errorf("%s is larger than %d bytes", ObjectFile, MaxObjectBytes)
	}
	d, err := obd.Decode(data)
	if err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", ObjectFile, err)
	}
	if err := d.Thing.Validate(); err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", ObjectFile, err)
	}
	if d.SpriteSize != m.SpriteSize {
		return nil, Info{}, fmt.Errorf("%s uses %d px sprites, meta says %d", ObjectFile, d.SpriteSize, m.SpriteSize)
	}
	if d.Thing.Category.String() != m.Category {
		return nil, Info{}, fmt.Errorf("%s is a %s, meta says %s", ObjectFile, d.Thing.Category, m.Category)
	}
	g := d.Thing.FrameGroups[0]
	info := Info{ClientVersion: d.ClientVersion, Width: int(g.Width), Height: int(g.Height)}
	for _, g := range d.Thing.FrameGroups {
		info.Frames += int(g.Frames)
		info.Sprites += len(g.Sprites)
	}
	return d, info, nil
}

// ReadPack decodes a sprite pack and returns its non-empty sprites in grid
// order (left to right, top to bottom).
func ReadPack(m *Meta, data []byte) ([][]byte, Info, error) {
	if len(data) > MaxPackBytes {
		return nil, Info{}, fmt.Errorf("%s is larger than %d bytes", SpritesFile, MaxPackBytes)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", SpritesFile, err)
	}
	size := m.SpriteSize
	if size == 0 || cfg.Width%size != 0 || cfg.Height%size != 0 || cfg.Width == 0 || cfg.Height == 0 {
		return nil, Info{}, fmt.Errorf("%s is %dx%d, not a grid of %d px sprites", SpritesFile, cfg.Width, cfg.Height, size)
	}
	if (cfg.Width/size)*(cfg.Height/size) > MaxPackSprites {
		return nil, Info{}, fmt.Errorf("%s holds more than %d sprites", SpritesFile, MaxPackSprites)
	}
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", SpritesFile, err)
	}
	imaging.RemoveMagenta(img)
	imaging.Normalize(img)
	var sprites [][]byte
	for _, px := range imaging.SliceTiles(img, size) {
		if slices.ContainsFunc(alphas(px), func(a byte) bool { return a != 0 }) {
			sprites = append(sprites, px)
		}
	}
	if len(sprites) == 0 {
		return nil, Info{}, fmt.Errorf("%s has no sprites", SpritesFile)
	}
	return sprites, Info{Width: cfg.Width / size, Height: cfg.Height / size, Sprites: len(sprites)}, nil
}

func alphas(px []byte) []byte {
	out := make([]byte, 0, len(px)/4)
	for i := 3; i < len(px); i += 4 {
		out = append(out, px[i])
	}
	return out
}

// PackImage lays sprites out in a grid of up to 8 columns.
func PackImage(sprites [][]byte, size int) (*image.NRGBA, error) {
	if len(sprites) == 0 {
		return nil, errors.New("no sprites")
	}
	if len(sprites) > MaxPackSprites {
		return nil, fmt.Errorf("a pack holds at most %d sprites", MaxPackSprites)
	}
	cols := min(len(sprites), 8)
	rows := (len(sprites) + cols - 1) / cols
	img := image.NewNRGBA(image.Rect(0, 0, cols*size, rows*size))
	for i, px := range sprites {
		imaging.PutTile(img, (i%cols)*size, (i/cols)*size, size, px)
	}
	return img, nil
}

// walkFrameMs is the preview speed of walking frames (the client moves
// them with the creature speed; stored durations are placeholders).
const walkFrameMs = 100

// Preview renders an animated GIF of an object: outfits walk south, other
// things show their first pattern. Outfits with a color mask layer are
// painted with the default outfit colors.
func Preview(d *obd.Data) ([]byte, error) {
	t := d.Thing
	gi := 0
	walking := false
	if t.Category == thing.CategoryOutfit {
		for i, g := range t.FrameGroups {
			if g.Type == thing.FrameGroupWalking && g.Frames > 1 {
				gi, walking = i, true
			}
		}
	}
	g := t.FrameGroups[gi]
	px := 0
	if t.Category == thing.CategoryOutfit && g.PatternX > uint8(thing.South) {
		px = int(thing.South)
	}
	size := d.SpriteSize
	w, h := int(g.Width)*size, int(g.Height)*size
	frames := make([]*image.NRGBA, 0, g.Frames)
	for f := 0; f < int(g.Frames) && f < 64; f++ {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for th := 0; th < int(g.Height); th++ {
			for tw := 0; tw < int(g.Width); tw++ {
				s := d.Sprites[gi][g.SpriteIndex(tw, th, 0, px, 0, 0, f)].Pixels
				if t.Category == thing.CategoryOutfit && g.Layers > 1 {
					s = slices.Clone(s)
					colorize(s, d.Sprites[gi][g.SpriteIndex(tw, th, 1, px, 0, 0, f)].Pixels)
				}
				imaging.PutTile(img, (int(g.Width)-1-tw)*size, (int(g.Height)-1-th)*size, size, s)
			}
		}
		frames = append(frames, img)
	}
	// Crop every frame to the union of visible pixels.
	var box image.Rectangle
	for _, f := range frames {
		box = box.Union(imaging.ContentBounds(f))
	}
	if box.Empty() {
		box = image.Rect(0, 0, min(w, size), min(h, size))
	}
	for _, f := range frames {
		for i := 0; i < len(f.Pix); i += 4 {
			c := opaque(color.NRGBA{f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3]})
			f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
	pal, exact := palette(frames, box)
	out := &gif.GIF{LoopCount: 0}
	for i, f := range frames {
		p := image.NewPaletted(image.Rect(0, 0, box.Dx(), box.Dy()), pal)
		if exact {
			for y := 0; y < box.Dy(); y++ {
				for x := 0; x < box.Dx(); x++ {
					p.SetColorIndex(x, y, uint8(pal.Index(f.NRGBAAt(box.Min.X+x, box.Min.Y+y))))
				}
			}
		} else {
			draw.FloydSteinberg.Draw(p, p.Rect, f, box.Min)
		}
		delay := walkFrameMs
		if !walking && i < len(g.Durations) {
			delay = int(g.Durations[i].Min+g.Durations[i].Max) / 2
		} else if !walking {
			delay = int(t.Category.DefaultDuration())
		}
		out.Image = append(out.Image, p)
		out.Delay = append(out.Delay, max(delay/10, 2))
		out.Disposal = append(out.Disposal, gif.DisposalBackground)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var transparent = color.NRGBA{}

// opaque maps half transparent pixels to the GIF's one bit alpha.
func opaque(c color.NRGBA) color.NRGBA {
	if c.A < 128 {
		return transparent
	}
	c.A = 255
	return c
}

// palette returns the exact colors of the frames when they fit in a GIF
// palette, else a web palette for dithering. Index 0 is transparent.
func palette(frames []*image.NRGBA, box image.Rectangle) (color.Palette, bool) {
	pal := color.Palette{transparent}
	seen := map[color.NRGBA]bool{transparent: true}
	for _, f := range frames {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				c := f.NRGBAAt(x, y)
				if !seen[c] {
					seen[c] = true
					pal = append(pal, c)
					if len(pal) > 256 {
						return webPalette(), false
					}
				}
			}
		}
	}
	return pal, true
}

func webPalette() color.Palette {
	pal := color.Palette{transparent}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				pal = append(pal, color.NRGBA{uint8(r * 51), uint8(g * 51), uint8(b * 51), 255})
			}
		}
	}
	for i := len(pal); i < 256; i++ {
		v := uint8((i - 217) * 6)
		pal = append(pal, color.NRGBA{v, v, v, 255})
	}
	return pal
}
