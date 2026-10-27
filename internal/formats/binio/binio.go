package binio

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrUnexpectedEOF = errors.New("binio: unexpected end of data")

// Reader reads little-endian values from a byte slice.
// After the first error every read returns a zero value and Err reports it.
type Reader struct {
	buf []byte
	pos int
	err error
}

func NewReader(buf []byte) *Reader {
	return &Reader{buf: buf}
}

func (r *Reader) Err() error { return r.err }

func (r *Reader) Pos() int { return r.pos }

func (r *Reader) Len() int { return len(r.buf) }

func (r *Reader) Remaining() int {
	if r.pos >= len(r.buf) {
		return 0
	}
	return len(r.buf) - r.pos
}

func (r *Reader) Seek(pos int) {
	if r.err != nil {
		return
	}
	if pos < 0 || pos > len(r.buf) {
		r.err = fmt.Errorf("binio: seek to %d out of range [0,%d]", pos, len(r.buf))
		return
	}
	r.pos = pos
}

func (r *Reader) Fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *Reader) take(count int) []byte {
	if r.err != nil {
		return nil
	}
	if count < 0 || r.pos+count > len(r.buf) {
		r.err = fmt.Errorf("%w: need %d bytes at offset %d, have %d", ErrUnexpectedEOF, count, r.pos, r.Remaining())
		return nil
	}
	chunk := r.buf[r.pos : r.pos+count]
	r.pos += count
	return chunk
}

func (r *Reader) U8() uint8 {
	chunk := r.take(1)
	if chunk == nil {
		return 0
	}
	return chunk[0]
}

func (r *Reader) I8() int8 { return int8(r.U8()) }

func (r *Reader) Bool() bool { return r.U8() != 0 }

func (r *Reader) U16() uint16 {
	chunk := r.take(2)
	if chunk == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(chunk)
}

func (r *Reader) I16() int16 { return int16(r.U16()) }

func (r *Reader) U32() uint32 {
	chunk := r.take(4)
	if chunk == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(chunk)
}

func (r *Reader) I32() int32 { return int32(r.U32()) }

func (r *Reader) U64() uint64 {
	chunk := r.take(8)
	if chunk == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(chunk)
}

// Bytes reads count bytes. The returned slice aliases the underlying buffer.
func (r *Reader) Bytes(count int) []byte { return r.take(count) }

func (r *Reader) Latin1(count int) string {
	chunk := r.take(count)
	if chunk == nil {
		return ""
	}
	return DecodeLatin1(chunk)
}

// UTF reads a string prefixed with a u16 byte length (Flash writeUTF format).
func (r *Reader) UTF() string {
	length := int(r.U16())
	chunk := r.take(length)
	if chunk == nil {
		return ""
	}
	return string(chunk)
}

type Writer struct {
	buf []byte
}

func NewWriter(capacity int) *Writer {
	return &Writer{buf: make([]byte, 0, capacity)}
}

// Bytes returns the written data. The slice aliases the internal buffer.
func (w *Writer) Bytes() []byte { return w.buf }

func (w *Writer) Len() int { return len(w.buf) }

func (w *Writer) U8(value uint8) { w.buf = append(w.buf, value) }

func (w *Writer) I8(value int8) { w.U8(uint8(value)) }

func (w *Writer) Bool(value bool) {
	if value {
		w.U8(1)
	} else {
		w.U8(0)
	}
}

func (w *Writer) U16(value uint16) { w.buf = binary.LittleEndian.AppendUint16(w.buf, value) }

func (w *Writer) I16(value int16) { w.U16(uint16(value)) }

func (w *Writer) U32(value uint32) { w.buf = binary.LittleEndian.AppendUint32(w.buf, value) }

func (w *Writer) I32(value int32) { w.U32(uint32(value)) }

func (w *Writer) Write(data []byte) { w.buf = append(w.buf, data...) }

func (w *Writer) PutU16At(pos int, value uint16) { binary.LittleEndian.PutUint16(w.buf[pos:], value) }

func (w *Writer) PutU32At(pos int, value uint32) { binary.LittleEndian.PutUint32(w.buf[pos:], value) }

func (w *Writer) UTF(text string) {
	w.U16(uint16(len(text)))
	w.buf = append(w.buf, text...)
}

func DecodeLatin1(data []byte) string {
	runes := make([]rune, len(data))
	for i, char := range data {
		runes[i] = rune(char)
	}
	return string(runes)
}

// EncodeLatin1 converts a string to ISO-8859-1. Runes above 0xFF become '?'.
func EncodeLatin1(text string) []byte {
	out := make([]byte, 0, len(text))
	for _, char := range text {
		if char > 0xFF {
			char = '?'
		}
		out = append(out, byte(char))
	}
	return out
}
