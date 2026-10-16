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

	"github.com/nekiro/ots-creator/internal/core/imaging"
	"github.com/nekiro/ots-creator/internal/core/thing"
	"github.com/nekiro/ots-creator/internal/formats/obd"
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
func ReadObject(meta *Meta, data []byte) (*obd.Data, Info, error) {
	if len(data) > MaxObjectBytes {
		return nil, Info{}, fmt.Errorf("%s is larger than %d bytes", ObjectFile, MaxObjectBytes)
	}
	decoded, err := obd.Decode(data)
	if err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", ObjectFile, err)
	}
	if err := decoded.Thing.Validate(); err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", ObjectFile, err)
	}
	if decoded.SpriteSize != meta.SpriteSize {
		return nil, Info{}, fmt.Errorf("%s uses %d px sprites, meta says %d", ObjectFile, decoded.SpriteSize, meta.SpriteSize)
	}
	if decoded.Thing.Category.String() != meta.Category {
		return nil, Info{}, fmt.Errorf("%s is a %s, meta says %s", ObjectFile, decoded.Thing.Category, meta.Category)
	}
	first := decoded.Thing.FrameGroups[0]
	info := Info{ClientVersion: decoded.ClientVersion, Width: int(first.Width), Height: int(first.Height)}
	for _, group := range decoded.Thing.FrameGroups {
		info.Frames += int(group.Frames)
		info.Sprites += len(group.Sprites)
	}
	return decoded, info, nil
}

// ReadPack decodes a sprite pack and returns its non-empty sprites in grid
// order (left to right, top to bottom).
func ReadPack(meta *Meta, data []byte) ([][]byte, Info, error) {
	if len(data) > MaxPackBytes {
		return nil, Info{}, fmt.Errorf("%s is larger than %d bytes", SpritesFile, MaxPackBytes)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", SpritesFile, err)
	}
	size := meta.SpriteSize
	if size == 0 || config.Width%size != 0 || config.Height%size != 0 || config.Width == 0 || config.Height == 0 {
		return nil, Info{}, fmt.Errorf("%s is %dx%d, not a grid of %d px sprites", SpritesFile, config.Width, config.Height, size)
	}
	if (config.Width/size)*(config.Height/size) > MaxPackSprites {
		return nil, Info{}, fmt.Errorf("%s holds more than %d sprites", SpritesFile, MaxPackSprites)
	}
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, Info{}, fmt.Errorf("%s: %w", SpritesFile, err)
	}
	imaging.RemoveMagenta(img)
	imaging.Normalize(img)
	var sprites [][]byte
	for _, pixels := range imaging.SliceTiles(img, size) {
		if slices.ContainsFunc(alphas(pixels), func(alpha byte) bool { return alpha != 0 }) {
			sprites = append(sprites, pixels)
		}
	}
	if len(sprites) == 0 {
		return nil, Info{}, fmt.Errorf("%s has no sprites", SpritesFile)
	}
	return sprites, Info{Width: config.Width / size, Height: config.Height / size, Sprites: len(sprites)}, nil
}

func alphas(pixels []byte) []byte {
	out := make([]byte, 0, len(pixels)/4)
	for i := 3; i < len(pixels); i += 4 {
		out = append(out, pixels[i])
	}
	return out
}

// packColumns is the widest grid PackImage lays sprites out in.
const packColumns = 8

// PackImage lays sprites out in a grid of up to packColumns columns.
func PackImage(sprites [][]byte, size int) (*image.NRGBA, error) {
	if len(sprites) == 0 {
		return nil, errors.New("no sprites")
	}
	if len(sprites) > MaxPackSprites {
		return nil, fmt.Errorf("a pack holds at most %d sprites", MaxPackSprites)
	}
	columns := min(len(sprites), packColumns)
	rows := (len(sprites) + columns - 1) / columns
	img := image.NewNRGBA(image.Rect(0, 0, columns*size, rows*size))
	for i, pixels := range sprites {
		imaging.PutTile(img, (i%columns)*size, (i/columns)*size, size, pixels)
	}
	return img, nil
}

// walkFrameMs is the preview speed of walking frames (the client moves
// them with the creature speed; stored durations are placeholders).
const walkFrameMs = 100

// maxPreviewFrames bounds the frames of a preview GIF.
const maxPreviewFrames = 64

// Preview renders an animated GIF of an object: outfits walk south, other
// things show their first pattern. Outfits with a color mask layer are
// painted with the default outfit colors.
func Preview(object *obd.Data) ([]byte, error) {
	thingType := object.Thing
	groupIndex := 0
	walking := false
	if thingType.Category == thing.CategoryOutfit {
		for i, group := range thingType.FrameGroups {
			if group.Type == thing.FrameGroupWalking && group.Frames > 1 {
				groupIndex, walking = i, true
			}
		}
	}
	group := thingType.FrameGroups[groupIndex]
	patternX := 0
	if thingType.Category == thing.CategoryOutfit && group.PatternX > uint8(thing.South) {
		patternX = int(thing.South)
	}
	size := object.SpriteSize
	width, height := int(group.Width)*size, int(group.Height)*size
	frames := make([]*image.NRGBA, 0, group.Frames)
	for frameIndex := 0; frameIndex < int(group.Frames) && frameIndex < maxPreviewFrames; frameIndex++ {
		img := image.NewNRGBA(image.Rect(0, 0, width, height))
		for tileY := 0; tileY < int(group.Height); tileY++ {
			for tileX := 0; tileX < int(group.Width); tileX++ {
				pixels := object.Sprites[groupIndex][group.SpriteIndex(tileX, tileY, 0, patternX, 0, 0, frameIndex)].Pixels
				if thingType.Category == thing.CategoryOutfit && group.Layers > 1 {
					pixels = slices.Clone(pixels)
					colorize(pixels, object.Sprites[groupIndex][group.SpriteIndex(tileX, tileY, 1, patternX, 0, 0, frameIndex)].Pixels)
				}
				imaging.PutTile(img, (int(group.Width)-1-tileX)*size, (int(group.Height)-1-tileY)*size, size, pixels)
			}
		}
		frames = append(frames, img)
	}
	// Crop every frame to the union of visible pixels.
	var box image.Rectangle
	for _, frame := range frames {
		box = box.Union(imaging.ContentBounds(frame))
	}
	if box.Empty() {
		box = image.Rect(0, 0, min(width, size), min(height, size))
	}
	for _, frame := range frames {
		for i := 0; i < len(frame.Pix); i += 4 {
			pixel := opaque(color.NRGBA{frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2], frame.Pix[i+3]})
			frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2], frame.Pix[i+3] = pixel.R, pixel.G, pixel.B, pixel.A
		}
	}
	colors, exact := palette(frames, box)
	out := &gif.GIF{LoopCount: 0}
	for i, frame := range frames {
		paletted := image.NewPaletted(image.Rect(0, 0, box.Dx(), box.Dy()), colors)
		if exact {
			for y := 0; y < box.Dy(); y++ {
				for x := 0; x < box.Dx(); x++ {
					paletted.SetColorIndex(x, y, uint8(colors.Index(frame.NRGBAAt(box.Min.X+x, box.Min.Y+y))))
				}
			}
		} else {
			draw.FloydSteinberg.Draw(paletted, paletted.Rect, frame, box.Min)
		}
		delay := walkFrameMs
		if !walking && i < len(group.Durations) {
			delay = int(group.Durations[i].Min+group.Durations[i].Max) / 2
		} else if !walking {
			delay = int(thingType.Category.DefaultDuration())
		}
		out.Image = append(out.Image, paletted)
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
func opaque(pixel color.NRGBA) color.NRGBA {
	if pixel.A < 128 {
		return transparent
	}
	pixel.A = 255
	return pixel
}

// palette returns the exact colors of the frames when they fit in a GIF
// palette, else a web palette for dithering. Index 0 is transparent.
func palette(frames []*image.NRGBA, box image.Rectangle) (color.Palette, bool) {
	colors := color.Palette{transparent}
	seen := map[color.NRGBA]bool{transparent: true}
	for _, frame := range frames {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				pixel := frame.NRGBAAt(x, y)
				if !seen[pixel] {
					seen[pixel] = true
					colors = append(colors, pixel)
					if len(colors) > 256 {
						return webPalette(), false
					}
				}
			}
		}
	}
	return colors, true
}

func webPalette() color.Palette {
	colors := color.Palette{transparent}
	for red := 0; red < 6; red++ {
		for green := 0; green < 6; green++ {
			for blue := 0; blue < 6; blue++ {
				colors = append(colors, color.NRGBA{uint8(red * 51), uint8(green * 51), uint8(blue * 51), 255})
			}
		}
	}
	for i := len(colors); i < 256; i++ {
		gray := uint8((i - 217) * 6)
		colors = append(colors, color.NRGBA{gray, gray, gray, 255})
	}
	return colors
}
