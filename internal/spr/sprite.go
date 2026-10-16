// Package spr reads and writes Tibia.spr sprite files and the RLE pixel
// compression they use.
package spr

import (
	"encoding/binary"
	"fmt"
)

// Compress encodes RGBA (non-premultiplied, 4 bytes per pixel, row major)
// pixels of a size x size sprite. A pixel with alpha 0 is transparent.
// Without transparency, colored pixels are stored as opaque RGB.
// A fully transparent sprite compresses to an empty slice.
func Compress(rgba []byte, size int, transparent bool) []byte {
	pixelCount := size * size
	if len(rgba) != pixelCount*4 {
		panic(fmt.Sprintf("spr.Compress: got %d bytes, want %d", len(rgba), pixelCount*4))
	}
	channels := 3
	if transparent {
		channels = 4
	}
	out := make([]byte, 0, 64)
	i := 0
	for i < pixelCount {
		skip := 0
		for i < pixelCount && rgba[i*4+3] == 0 {
			skip++
			i++
		}
		if i >= pixelCount {
			break // trailing transparent pixels are implicit
		}
		start := len(out)
		out = binary.LittleEndian.AppendUint16(out, uint16(skip))
		out = append(out, 0, 0) // colored count, patched below
		colored := 0
		for i < pixelCount && rgba[i*4+3] != 0 {
			pixel := rgba[i*4 : i*4+4]
			out = append(out, pixel[:channels]...)
			colored++
			i++
		}
		binary.LittleEndian.PutUint16(out[start+2:], uint16(colored))
	}
	return out
}

// Decompress decodes RLE data into RGBA pixels of a size x size sprite.
func Decompress(data []byte, size int, transparent bool) ([]byte, error) {
	out := make([]byte, size*size*4)
	return out, DecompressInto(out, data, size, transparent)
}

// DecompressInto decodes RLE data into dst, which must hold size*size*4
// bytes. dst is fully overwritten.
func DecompressInto(dst, data []byte, size int, transparent bool) error {
	pixelCount := size * size
	if len(dst) != pixelCount*4 {
		return fmt.Errorf("spr: destination has %d bytes, want %d", len(dst), pixelCount*4)
	}
	clear(dst)
	channels := 3
	if transparent {
		channels = 4
	}
	pos, pixel := 0, 0
	for pos < len(data) {
		if pos+4 > len(data) {
			return fmt.Errorf("spr: truncated chunk header at %d", pos)
		}
		skip := int(binary.LittleEndian.Uint16(data[pos:]))
		colored := int(binary.LittleEndian.Uint16(data[pos+2:]))
		pos += 4
		pixel += skip
		if pixel+colored > pixelCount {
			return fmt.Errorf("spr: pixel overflow (%d > %d)", pixel+colored, pixelCount)
		}
		if pos+colored*channels > len(data) {
			return fmt.Errorf("spr: truncated pixel data at %d", pos)
		}
		for i := 0; i < colored; i++ {
			target := dst[pixel*4 : pixel*4+4]
			target[0], target[1], target[2] = data[pos], data[pos+1], data[pos+2]
			if transparent {
				target[3] = data[pos+3]
			} else {
				target[3] = 0xFF
			}
			pos += channels
			pixel++
		}
	}
	return nil
}

// Transcode converts compressed data between the opaque and transparent
// encodings. It returns data unchanged when both encodings match.
func Transcode(data []byte, size int, from, to bool) ([]byte, error) {
	if from == to || len(data) == 0 {
		return data, nil
	}
	pixels, err := Decompress(data, size, from)
	if err != nil {
		return nil, err
	}
	return Compress(pixels, size, to), nil
}
