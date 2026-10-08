package spr

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"testing"
)

func pattern(size int, seed uint64, transparent bool) []byte {
	rng := rand.New(rand.NewPCG(seed, 1))
	px := make([]byte, size*size*4)
	for i := 0; i < size*size; i++ {
		if rng.IntN(3) == 0 {
			continue // transparent run
		}
		p := px[i*4:]
		p[0], p[1], p[2] = byte(rng.IntN(256)), byte(rng.IntN(256)), byte(rng.IntN(256))
		if transparent {
			p[3] = byte(1 + rng.IntN(255))
		} else {
			p[3] = 0xFF
		}
	}
	return px
}

func TestCompressRoundTrip(t *testing.T) {
	for _, size := range []int{32, 64} {
		for _, tr := range []bool{false, true} {
			for seed := uint64(0); seed < 20; seed++ {
				src := pattern(size, seed, tr)
				c := Compress(src, size, tr)
				got, err := Decompress(c, size, tr)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(src, got) {
					t.Fatalf("size=%d transparent=%v seed=%d mismatch", size, tr, seed)
				}
				if again := Compress(got, size, tr); !bytes.Equal(again, c) {
					t.Fatal("compress is not stable")
				}
			}
		}
	}
}

func TestCompressKnownBytes(t *testing.T) {
	px := make([]byte, 32*32*4)
	// pixel 2 and 3 red, pixel 5 green; rest transparent.
	copy(px[2*4:], []byte{255, 0, 0, 255, 255, 0, 0, 255})
	copy(px[5*4:], []byte{0, 255, 0, 255})
	got := Compress(px, 32, false)
	want := []byte{
		2, 0, 2, 0, 255, 0, 0, 255, 0, 0, // skip 2, 2 colored
		1, 0, 1, 0, 0, 255, 0, // skip 1, 1 colored
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	gotT := Compress(px, 32, true)
	if len(gotT) != 4+8+4+4 {
		t.Fatalf("transparent len = %d", len(gotT))
	}
}

func TestEmptyAndFull(t *testing.T) {
	empty := make([]byte, 32*32*4)
	if c := Compress(empty, 32, true); len(c) != 0 {
		t.Fatalf("empty sprite compressed to %d bytes", len(c))
	}
	px, err := Decompress(nil, 32, false)
	if err != nil || !bytes.Equal(px, empty) {
		t.Fatal("decompress nil")
	}
	full := bytes.Repeat([]byte{1, 2, 3, 255}, 32*32)
	c := Compress(full, 32, false)
	if binary.LittleEndian.Uint16(c[0:]) != 0 || binary.LittleEndian.Uint16(c[2:]) != 1024 || len(c) != 4+1024*3 {
		t.Fatal("full sprite layout")
	}
}

func TestOpaqueDropsAlpha(t *testing.T) {
	px := make([]byte, 32*32*4)
	copy(px, []byte{10, 20, 30, 128})
	got, _ := Decompress(Compress(px, 32, false), 32, false)
	if got[3] != 255 || got[0] != 10 {
		t.Fatalf("got %v", got[:4])
	}
}

func TestDecompressErrors(t *testing.T) {
	cases := map[string][]byte{
		"short header": {1, 0, 1},
		"overflow":     {0, 4, 1, 0, 1, 2, 3}, // skip 1024 then 1 colored
		"truncated":    {0, 0, 2, 0, 1, 2, 3},
	}
	for name, data := range cases {
		if _, err := Decompress(data, 32, false); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestTranscode(t *testing.T) {
	src := pattern(32, 7, false)
	opaque := Compress(src, 32, false)
	tr, err := Transcode(opaque, 32, false, true)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Transcode(tr, 32, true, false)
	if err != nil || !bytes.Equal(back, opaque) {
		t.Fatal("transcode round trip failed")
	}
}

func TestFileRoundTrip(t *testing.T) {
	for _, opts := range []Options{{Extended: false}, {Extended: true, Transparent: true}, {Extended: true, SpriteSize: 64}} {
		size := opts.size()
		src := SliceSource{
			Compress(pattern(size, 1, opts.Transparent), size, opts.Transparent),
			nil, // empty sprite
			Compress(pattern(size, 2, opts.Transparent), size, opts.Transparent),
		}
		var buf bytes.Buffer
		if err := Encode(&buf, 0x57BBD603, src, opts); err != nil {
			t.Fatal(err)
		}
		f, err := Open(buf.Bytes(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if f.Signature != 0x57BBD603 || f.Count != 3 {
			t.Fatalf("header %x %d", f.Signature, f.Count)
		}
		for id := uint32(1); id <= 3; id++ {
			c, err := f.Compressed(id)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(c, src[id-1]) {
				t.Fatalf("sprite %d mismatch", id)
			}
		}
		if _, err := f.Compressed(4); err == nil {
			t.Fatal("out of range id must fail")
		}
		if _, err := f.Compressed(0); err == nil {
			t.Fatal("id 0 must fail")
		}
		px, err := f.Pixels(1)
		if err != nil || len(px) != size*size*4 {
			t.Fatal("Pixels")
		}
		// Empty sprites have address 0.
		at := opts.headerSize() + 4
		if binary.LittleEndian.Uint32(buf.Bytes()[at:]) != 0 {
			t.Fatal("empty sprite must have address 0")
		}
	}
}

func TestEncodeLimits(t *testing.T) {
	big := make(SliceSource, 0x10000)
	if err := Encode(&bytes.Buffer{}, 1, big, Options{}); err == nil {
		t.Fatal("65536 sprites without extended must fail")
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open([]byte{1, 2}, Options{}); err == nil {
		t.Fatal("tiny file")
	}
	// Header claims 1000 sprites but has no table.
	data := []byte{0, 0, 0, 0, 0xE8, 0x03}
	if _, err := Open(data, Options{}); err == nil {
		t.Fatal("missing table")
	}
	// Address points outside the file.
	data = []byte{0, 0, 0, 0, 1, 0, 0xFF, 0, 0, 0}
	f, err := Open(data, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Compressed(1); err == nil {
		t.Fatal("bad address")
	}
}
