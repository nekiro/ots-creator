package binio

import (
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	writer := NewWriter(0)
	writer.U8(0xAB)
	writer.I8(-3)
	writer.Bool(true)
	writer.U16(0xBEEF)
	writer.I16(-1234)
	writer.U32(0xDEADBEEF)
	writer.I32(-123456)
	writer.UTF("hello")
	writer.Write([]byte{1, 2, 3})

	reader := NewReader(writer.Bytes())
	if got := reader.U8(); got != 0xAB {
		t.Fatalf("U8 = %x", got)
	}
	if got := reader.I8(); got != -3 {
		t.Fatalf("I8 = %d", got)
	}
	if !reader.Bool() {
		t.Fatal("Bool = false")
	}
	if got := reader.U16(); got != 0xBEEF {
		t.Fatalf("U16 = %x", got)
	}
	if got := reader.I16(); got != -1234 {
		t.Fatalf("I16 = %d", got)
	}
	if got := reader.U32(); got != 0xDEADBEEF {
		t.Fatalf("U32 = %x", got)
	}
	if got := reader.I32(); got != -123456 {
		t.Fatalf("I32 = %d", got)
	}
	if got := reader.UTF(); got != "hello" {
		t.Fatalf("UTF = %q", got)
	}
	if got := reader.Bytes(3); len(got) != 3 || got[2] != 3 {
		t.Fatalf("Bytes = %v", got)
	}
	if reader.Err() != nil || reader.Remaining() != 0 {
		t.Fatalf("err=%v remaining=%d", reader.Err(), reader.Remaining())
	}
}

func TestLittleEndianLayout(t *testing.T) {
	writer := NewWriter(0)
	writer.U32(0x01020304)
	data := writer.Bytes()
	if data[0] != 4 || data[3] != 1 {
		t.Fatalf("not little endian: %v", data)
	}
}

func TestStickyError(t *testing.T) {
	reader := NewReader([]byte{1, 2, 3})
	_ = reader.U16()
	_ = reader.U16() // fails, only one byte left
	if !errors.Is(reader.Err(), ErrUnexpectedEOF) {
		t.Fatalf("expected EOF error, got %v", reader.Err())
	}
	if got := reader.U8(); got != 0 {
		t.Fatalf("read after error returned %d", got)
	}
	if reader.Pos() != 2 {
		t.Fatalf("position moved after error: %d", reader.Pos())
	}
}

func TestSeekOutOfRange(t *testing.T) {
	reader := NewReader([]byte{1})
	reader.Seek(5)
	if reader.Err() == nil {
		t.Fatal("expected error")
	}
}

func TestPutAt(t *testing.T) {
	writer := NewWriter(0)
	writer.U32(0)
	writer.U16(0)
	writer.PutU32At(0, 7)
	writer.PutU16At(4, 9)
	reader := NewReader(writer.Bytes())
	if reader.U32() != 7 || reader.U16() != 9 {
		t.Fatal("PutAt failed")
	}
}

func TestLatin1(t *testing.T) {
	src := []byte{'a', 0xE9, 0xFF}
	text := DecodeLatin1(src)
	if text != "aéÿ" {
		t.Fatalf("decode = %q", text)
	}
	back := EncodeLatin1(text)
	if string(back) != string(src) {
		t.Fatalf("encode = %v", back)
	}
	if got := EncodeLatin1("ż"); string(got) != "?" {
		t.Fatalf("non-latin1 = %v", got)
	}
	reader := NewReader(src)
	if reader.Latin1(3) != "aéÿ" {
		t.Fatal("Reader.Latin1 failed")
	}
}

func TestReaderPositioning(t *testing.T) {
	reader := NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	if reader.Len() != 9 || reader.Pos() != 0 {
		t.Fatalf("len %d pos %d", reader.Len(), reader.Pos())
	}
	reader.Seek(1)
	if got := reader.U64(); got != 0x0908070605040302 {
		t.Fatalf("U64 = %x", got)
	}
	if reader.Remaining() != 0 || reader.U64() != 0 || reader.Err() == nil {
		t.Fatal("U64 past the end did not fail")
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining %d", reader.Remaining())
	}
	reader.Seek(0) // ignored after an error
	if reader.Pos() != 9 {
		t.Fatalf("seek after error moved to %d", reader.Pos())
	}
}

func TestReaderFail(t *testing.T) {
	first, second := errors.New("first"), errors.New("second")
	reader := NewReader([]byte{1})
	reader.Fail(first)
	reader.Fail(second)
	if reader.Err() != first || reader.U8() != 0 {
		t.Fatalf("err %v", reader.Err())
	}
	if NewReader(nil).Bytes(-1) != nil {
		t.Fatal("negative count returned data")
	}
}

func TestShortStrings(t *testing.T) {
	reader := NewReader([]byte{5, 0, 'a'})
	if reader.UTF() != "" || reader.Err() == nil {
		t.Fatal("truncated UTF did not fail")
	}
	reader = NewReader([]byte{'a'})
	if reader.Latin1(2) != "" || reader.Err() == nil {
		t.Fatal("truncated Latin1 did not fail")
	}
}

func TestWriterLen(t *testing.T) {
	writer := NewWriter(4)
	writer.Bool(false)
	writer.UTF("ab")
	if writer.Len() != 5 || writer.Bytes()[0] != 0 {
		t.Fatalf("written %v", writer.Bytes())
	}
}
