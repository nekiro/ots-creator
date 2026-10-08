package binio

import (
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	w := NewWriter(0)
	w.U8(0xAB)
	w.I8(-3)
	w.Bool(true)
	w.U16(0xBEEF)
	w.I16(-1234)
	w.U32(0xDEADBEEF)
	w.I32(-123456)
	w.UTF("hello")
	w.Write([]byte{1, 2, 3})

	r := NewReader(w.Bytes())
	if got := r.U8(); got != 0xAB {
		t.Fatalf("U8 = %x", got)
	}
	if got := r.I8(); got != -3 {
		t.Fatalf("I8 = %d", got)
	}
	if !r.Bool() {
		t.Fatal("Bool = false")
	}
	if got := r.U16(); got != 0xBEEF {
		t.Fatalf("U16 = %x", got)
	}
	if got := r.I16(); got != -1234 {
		t.Fatalf("I16 = %d", got)
	}
	if got := r.U32(); got != 0xDEADBEEF {
		t.Fatalf("U32 = %x", got)
	}
	if got := r.I32(); got != -123456 {
		t.Fatalf("I32 = %d", got)
	}
	if got := r.UTF(); got != "hello" {
		t.Fatalf("UTF = %q", got)
	}
	if got := r.Bytes(3); len(got) != 3 || got[2] != 3 {
		t.Fatalf("Bytes = %v", got)
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Fatalf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestLittleEndianLayout(t *testing.T) {
	w := NewWriter(0)
	w.U32(0x01020304)
	b := w.Bytes()
	if b[0] != 4 || b[3] != 1 {
		t.Fatalf("not little endian: %v", b)
	}
}

func TestStickyError(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	_ = r.U16()
	_ = r.U16() // fails, only one byte left
	if !errors.Is(r.Err(), ErrUnexpectedEOF) {
		t.Fatalf("expected EOF error, got %v", r.Err())
	}
	if got := r.U8(); got != 0 {
		t.Fatalf("read after error returned %d", got)
	}
	if r.Pos() != 2 {
		t.Fatalf("position moved after error: %d", r.Pos())
	}
}

func TestSeekOutOfRange(t *testing.T) {
	r := NewReader([]byte{1})
	r.Seek(5)
	if r.Err() == nil {
		t.Fatal("expected error")
	}
}

func TestPutAt(t *testing.T) {
	w := NewWriter(0)
	w.U32(0)
	w.U16(0)
	w.PutU32At(0, 7)
	w.PutU16At(4, 9)
	r := NewReader(w.Bytes())
	if r.U32() != 7 || r.U16() != 9 {
		t.Fatal("PutAt failed")
	}
}

func TestLatin1(t *testing.T) {
	src := []byte{'a', 0xE9, 0xFF}
	s := DecodeLatin1(src)
	if s != "aéÿ" {
		t.Fatalf("decode = %q", s)
	}
	back := EncodeLatin1(s)
	if string(back) != string(src) {
		t.Fatalf("encode = %v", back)
	}
	if got := EncodeLatin1("ż"); string(got) != "?" {
		t.Fatalf("non-latin1 = %v", got)
	}
	r := NewReader(src)
	if r.Latin1(3) != "aéÿ" {
		t.Fatal("Reader.Latin1 failed")
	}
}
