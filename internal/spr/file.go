package spr

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// Options select the encoding of a spr file.
type Options struct {
	Extended    bool // u32 sprite count instead of u16
	Transparent bool // pixels carry alpha
	SpriteSize  int  // edge length in pixels, 32 for official clients
}

func (o Options) headerSize() int {
	if o.Extended {
		return 8
	}
	return 6
}

func (o Options) size() int {
	if o.SpriteSize <= 0 {
		return 32
	}
	return o.SpriteSize
}

// File gives random access to the sprites of a loaded spr file.
// Sprite ids start at 1.
type File struct {
	Signature uint32
	Count     uint32
	opts      Options
	data      []byte
}

// Open parses the header of a spr file held in memory. data is not copied
// and must not change while the File is used.
func Open(data []byte, opts Options) (*File, error) {
	if len(data) < opts.headerSize() {
		return nil, fmt.Errorf("spr: file too small (%d bytes)", len(data))
	}
	f := &File{Signature: binary.LittleEndian.Uint32(data), opts: opts, data: data}
	if opts.Extended {
		f.Count = binary.LittleEndian.Uint32(data[4:])
	} else {
		f.Count = uint32(binary.LittleEndian.Uint16(data[4:]))
	}
	tableEnd := uint64(opts.headerSize()) + uint64(f.Count)*4
	if tableEnd > uint64(len(data)) {
		return nil, fmt.Errorf("spr: address table for %d sprites exceeds file size %d (wrong extended setting?)", f.Count, len(data))
	}
	return f, nil
}

// Options returns the options used to open the file.
func (f *File) Options() Options { return f.opts }

// Compressed returns the RLE data of a sprite. Empty sprites return nil.
// The slice aliases the file buffer.
func (f *File) Compressed(id uint32) ([]byte, error) {
	if id == 0 || id > f.Count {
		return nil, fmt.Errorf("spr: sprite id %d out of range [1,%d]", id, f.Count)
	}
	at := f.opts.headerSize() + int(id-1)*4
	addr := uint64(binary.LittleEndian.Uint32(f.data[at:]))
	if addr == 0 {
		return nil, nil
	}
	if addr+5 > uint64(len(f.data)) {
		return nil, fmt.Errorf("spr: sprite %d address %d out of file", id, addr)
	}
	// 3 bytes of color key (ignored), then u16 length.
	n := uint64(binary.LittleEndian.Uint16(f.data[addr+3:]))
	start := addr + 5
	if start+n > uint64(len(f.data)) {
		return nil, fmt.Errorf("spr: sprite %d data exceeds file", id)
	}
	if n == 0 {
		return nil, nil
	}
	return f.data[start : start+n], nil
}

// Pixels returns the decoded RGBA pixels of a sprite.
func (f *File) Pixels(id uint32) ([]byte, error) {
	c, err := f.Compressed(id)
	if err != nil {
		return nil, err
	}
	return Decompress(c, f.opts.size(), f.opts.Transparent)
}

// Source supplies compressed sprites for Encode.
type Source interface {
	// Count returns the number of sprites (highest id).
	Count() uint32
	// Compressed returns sprite data already encoded with the target
	// transparency. Empty sprites return nil.
	Compressed(id uint32) ([]byte, error)
}

// Encode writes a spr file. It calls src.Compressed twice for every sprite
// (once to lay out addresses, once to write), so the source should be cheap.
func Encode(out io.Writer, signature uint32, src Source, opts Options) error {
	count := src.Count()
	if !opts.Extended && count > 0xFFFF {
		return fmt.Errorf("spr: %d sprites need the extended feature", count)
	}
	bw := bufio.NewWriterSize(out, 1<<20)
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[:], signature)
	if opts.Extended {
		binary.LittleEndian.PutUint32(hdr[4:], count)
	} else {
		binary.LittleEndian.PutUint16(hdr[4:], uint16(count))
	}
	if _, err := bw.Write(hdr[:opts.headerSize()]); err != nil {
		return err
	}

	offset := uint64(opts.headerSize()) + uint64(count)*4
	var buf [4]byte
	for id := uint32(1); id <= count; id++ {
		c, err := src.Compressed(id)
		if err != nil {
			return fmt.Errorf("sprite %d: %w", id, err)
		}
		if len(c) > 0xFFFF {
			return fmt.Errorf("sprite %d: compressed size %d exceeds 65535", id, len(c))
		}
		addr := uint32(0)
		if len(c) > 0 {
			if offset > 0xFFFFFFFF {
				return fmt.Errorf("spr: file exceeds 4 GiB")
			}
			addr = uint32(offset)
			offset += 5 + uint64(len(c))
		}
		binary.LittleEndian.PutUint32(buf[:], addr)
		if _, err := bw.Write(buf[:]); err != nil {
			return err
		}
	}
	for id := uint32(1); id <= count; id++ {
		c, err := src.Compressed(id)
		if err != nil {
			return fmt.Errorf("sprite %d: %w", id, err)
		}
		if len(c) == 0 {
			continue
		}
		// Magenta color key, kept for compatibility with old tools.
		if _, err := bw.Write([]byte{0xFF, 0x00, 0xFF, byte(len(c)), byte(len(c) >> 8)}); err != nil {
			return err
		}
		if _, err := bw.Write(c); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// SliceSource is a Source backed by a slice where index 0 is sprite 1.
type SliceSource [][]byte

func (s SliceSource) Count() uint32 { return uint32(len(s)) }

func (s SliceSource) Compressed(id uint32) ([]byte, error) {
	if id == 0 || int(id) > len(s) {
		return nil, fmt.Errorf("sprite id %d out of range", id)
	}
	return s[id-1], nil
}
