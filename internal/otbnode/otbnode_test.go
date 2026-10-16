package otbnode

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf, [4]byte{'O', 'T', 'B', 'M'})
	writer.Start(1)
	writer.U32(0xFEFDFF00)
	writer.String("a\xfeb")
	writer.Start(2)
	writer.U16(0xFFFE)
	writer.End()
	writer.Start(3)
	writer.LongString("long")
	writer.Start(4)
	writer.End()
	writer.End()
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	root, ident, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if string(ident[:]) != "OTBM" || root.Type != 1 || len(root.Children) != 2 {
		t.Fatalf("bad root: %q %d %d", ident, root.Type, len(root.Children))
	}
	reader := NewReader(root.Props)
	if value := reader.U32(); value != 0xFEFDFF00 {
		t.Fatalf("u32 = %x", value)
	}
	if text := reader.String(); text != "a\xfeb" {
		t.Fatalf("string = %q", text)
	}
	if reader.More() || reader.Err() != nil {
		t.Fatal("unexpected trailing data")
	}
	if value := NewReader(root.Children[0].Props).U16(); value != 0xFFFE {
		t.Fatalf("child u16 = %x", value)
	}
	second := root.Children[1]
	if text := NewReader(second.Props).LongString(); text != "long" || len(second.Children) != 1 || second.Children[0].Type != 4 {
		t.Fatalf("bad second child %q", text)
	}
}

func TestMalformed(t *testing.T) {
	for _, data := range [][]byte{
		{0, 0, 0, 0, 0xFE, 1, 2},          // no end
		{0, 0, 0, 0, 1, 1, 0xFF},          // no start
		{0, 0, 0, 0, 0xFE, 1, 0xFD},       // dangling escape
		{0, 0, 0, 0, 0xFE, 1, 0xFE, 2, 3}, // unterminated child
	} {
		if _, _, err := Parse(data); err == nil {
			t.Errorf("Parse(%x) succeeded", data)
		}
	}
}

func TestReaderErrors(t *testing.T) {
	reader := NewReader([]byte{1})
	reader.U16()
	if reader.Err() == nil {
		t.Fatal("short read did not fail")
	}
	if reader.U8() != 0 {
		t.Fatal("read after error returned data")
	}
}

func TestReaderValues(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf, [4]byte{})
	writer.Start(1)
	writer.U8(0xFD)
	writer.U64(0x0102030405FEFDFF)
	writer.Bytes([]byte{9, 8, 7})
	writer.String("skip")
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	root, _, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	reader := NewReader(root.Props)
	if value := reader.U8(); value != 0xFD {
		t.Fatalf("u8 = %x", value)
	}
	if value := reader.U64(); value != 0x0102030405FEFDFF {
		t.Fatalf("u64 = %x", value)
	}
	if data := reader.Bytes(3); !bytes.Equal(data, []byte{9, 8, 7}) {
		t.Fatalf("bytes = %v", data)
	}
	if reader.Remaining() != 6 {
		t.Fatalf("remaining = %d", reader.Remaining())
	}
	reader.Skip(6)
	if reader.More() || reader.Remaining() != 0 || reader.Err() != nil {
		t.Fatalf("more %v remaining %d err %v", reader.More(), reader.Remaining(), reader.Err())
	}
}

func TestReaderErrorFormat(t *testing.T) {
	reader := NewReader([]byte{1})
	reader.U32()
	if !errors.Is(reader.Err(), ErrFormat) {
		t.Fatalf("short read error %v does not wrap ErrFormat", reader.Err())
	}
	if reader.More() || reader.String() != "" || reader.U64() != 0 {
		t.Fatal("reads after error returned data")
	}

	tooLong := NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	if tooLong.LongString() != "" || !errors.Is(tooLong.Err(), ErrFormat) {
		t.Fatalf("long string error %v", tooLong.Err())
	}
	if NewReader([]byte{5, 0, 'a'}).String() != "" {
		t.Fatal("truncated string returned data")
	}
}

func TestWriterLimits(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf, [4]byte{})
	writer.Start(1)
	writer.String(strings.Repeat("x", math.MaxUint16+10))
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	root, _, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if text := NewReader(root.Props).String(); len(text) != math.MaxUint16 {
		t.Fatalf("string length %d, want truncated to %d", len(text), math.MaxUint16)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriterError(t *testing.T) {
	writer := NewWriter(failingWriter{}, [4]byte{})
	writer.Start(1)
	writer.End()
	if err := writer.Flush(); err == nil {
		t.Fatal("write error lost")
	}
}

// tree writes root(1) > [a(2) > [x(4)], b(3)] with one property byte each.
func tree(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := NewWriter(&buf, [4]byte{'T', 'E', 'S', 'T'})
	writer.Start(1)
	writer.U8(10)
	writer.Start(2)
	writer.U8(20)
	writer.Start(4)
	writer.U8(40)
	writer.End()
	writer.End()
	writer.Start(3)
	writer.U8(30)
	writer.End()
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStream(t *testing.T) {
	var visited []byte
	root, ident, err := Stream(tree(t), 1, nil, func(stack []*Node, node *Node) error {
		if len(stack) != 1 || stack[0].Type != 1 || !bytes.Equal(stack[0].Props, []byte{10}) {
			t.Fatalf("stack %+v", stack)
		}
		visited = append(visited, node.Type, node.Props[0])
		if node.Type == 2 && (len(node.Children) != 1 || node.Children[0].Props[0] != 40) {
			t.Fatalf("subtree of streamed node %+v", node)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(ident[:]) != "TEST" || len(root.Children) != 0 || !bytes.Equal(root.Props, []byte{10}) {
		t.Fatalf("root %+v", root)
	}
	if !bytes.Equal(visited, []byte{2, 20, 3, 30}) {
		t.Fatalf("visited %v", visited)
	}

	stop := errors.New("stop")
	_, _, err = Stream(tree(t), 1, nil, func([]*Node, *Node) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("visit error %v", err)
	}
}

func TestParseProgress(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf, [4]byte{})
	writer.Start(1)
	writer.Bytes(bytes.Repeat([]byte{1}, 3*progressStep))
	writer.End()
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	root, _, err := ParseProgress(buf.Bytes(), func(done, total int) {
		calls++
		if done > total || total != buf.Len() {
			t.Fatalf("progress %d/%d", done, total)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 || len(root.Props) != 3*progressStep {
		t.Fatalf("%d progress calls, %d props", calls, len(root.Props))
	}
	if _, _, err := Parse([]byte{0, 0, 0, 0, 0xFE}); err == nil {
		t.Fatal("file without node type parsed")
	}
}
