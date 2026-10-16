// Package otbnode reads and writes the escaped node tree used by items.otb
// and OTBM maps: a 4-byte identifier followed by nodes. A node starts with
// 0xFE and its type byte, holds property bytes and child nodes, and ends
// with 0xFF. Property bytes equal to 0xFD, 0xFE or 0xFF are escaped with 0xFD.
package otbnode

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/nekiro/ots-creator/internal/formats/binio"
)

const (
	nodeStart = 0xFE
	nodeEnd   = 0xFF
	escape    = 0xFD
)

// ErrFormat is returned for malformed node files.
var ErrFormat = errors.New("otbnode: malformed node file")

// Node is one parsed node. Props holds the unescaped property bytes
// (without the type byte).
type Node struct {
	Type     byte
	Props    []byte
	Children []*Node
}

// Parse reads a node file. ident receives the 4-byte identifier.
func Parse(data []byte) (root *Node, ident [4]byte, err error) {
	return ParseProgress(data, nil)
}

// progressStep is how many bytes are parsed between progress calls.
const progressStep = 1 << 20

// ParseProgress is Parse that calls progress (when not nil) with the bytes
// parsed so far and the total.
func ParseProgress(data []byte, progress func(done, total int)) (root *Node, ident [4]byte, err error) {
	return (&parser{buf: make([]byte, 0, len(data)), progress: progress}).parse(data)
}

// Stream parses a node file without keeping all of it: every node at depth
// (the root is depth 0) is handed to visit with its finished subtree and
// then dropped, so only one such subtree is in memory at a time. stack holds
// the nodes above it, with their properties (their Children lack the
// streamed nodes). Nodes above depth are returned in root as usual.
// Properties and nodes passed to visit are only valid during the call.
func Stream(data []byte, depth int, progress func(done, total int), visit func(stack []*Node, node *Node) error) (root *Node, ident [4]byte, err error) {
	return (&parser{progress: progress, depth: depth, visit: visit}).parse(data)
}

func (p *parser) parse(data []byte) (root *Node, ident [4]byte, err error) {
	if len(data) < 6 {
		return nil, ident, fmt.Errorf("%w: file too small", ErrFormat)
	}
	copy(ident[:], data[:4])
	p.data, p.pos, p.nextProgress = data, 4, progressStep
	if p.data[p.pos] != nodeStart {
		return nil, ident, fmt.Errorf("%w: no root node", ErrFormat)
	}
	p.pos++
	root, err = p.node()
	return root, ident, err
}

type parser struct {
	data []byte
	pos  int
	// buf holds every node's unescaped properties back to back; nodes slice
	// into it, so parsing allocates one buffer instead of one per node.
	buf          []byte
	progress     func(done, total int)
	nextProgress int
	// Streaming (see Stream): nodes at depth go to visit, stack is the path.
	depth int
	visit func(stack []*Node, node *Node) error
	stack []*Node
}

// node parses a node whose start marker was consumed.
func (p *parser) node() (*Node, error) {
	if p.pos >= len(p.data) {
		return nil, fmt.Errorf("%w: truncated node", ErrFormat)
	}
	node := &Node{Type: p.data[p.pos]}
	p.pos++
	propsStart := len(p.buf)
	streamChildren := p.visit != nil && len(p.stack)+1 == p.depth
	if p.visit != nil {
		p.stack = append(p.stack, node)
		defer func() { p.stack = p.stack[:len(p.stack)-1] }()
	}
	childCount := 0
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case nodeStart:
			if childCount == 0 {
				// Copied: streamed children reuse the buffer after it.
				node.Props = p.buf[propsStart:len(p.buf):len(p.buf)]
				if streamChildren {
					node.Props = append([]byte(nil), node.Props...)
				}
			}
			childCount++
			p.pos++
			childStart := len(p.buf)
			child, err := p.node()
			if err != nil {
				return nil, err
			}
			if streamChildren {
				if err := p.visit(p.stack, child); err != nil {
					return nil, err
				}
				p.buf = p.buf[:childStart]
			} else {
				node.Children = append(node.Children, child)
			}
			// Properties after children are not part of the format.
			propsStart = len(p.buf)
		case nodeEnd:
			if childCount == 0 {
				node.Props = p.buf[propsStart:len(p.buf):len(p.buf)]
			}
			p.pos++
			return node, nil
		case escape:
			if p.pos+1 >= len(p.data) {
				return nil, fmt.Errorf("%w: dangling escape", ErrFormat)
			}
			p.buf = append(p.buf, p.data[p.pos+1])
			p.pos += 2
		default:
			// Copy the run of plain bytes at once.
			runEnd := p.pos + 1
			for runEnd < len(p.data) {
				value := p.data[runEnd]
				if value == nodeStart || value == nodeEnd || value == escape {
					break
				}
				runEnd++
			}
			p.buf = append(p.buf, p.data[p.pos:runEnd]...)
			p.pos = runEnd
			if p.progress != nil && p.pos >= p.nextProgress {
				p.progress(p.pos, len(p.data))
				p.nextProgress = p.pos + progressStep
			}
		}
	}
	return nil, fmt.Errorf("%w: missing node end", ErrFormat)
}

// Reader reads little-endian values from node properties with a sticky
// error. It wraps binio.Reader and adds the node file string formats;
// read errors are reported as ErrFormat.
type Reader struct {
	props binio.Reader
}

// NewReader returns a reader over props.
func NewReader(props []byte) *Reader { return &Reader{props: *binio.NewReader(props)} }

// Err returns the first read error.
func (r *Reader) Err() error {
	err := r.props.Err()
	if err == nil || errors.Is(err, ErrFormat) {
		return err
	}
	return fmt.Errorf("%w: property: %v", ErrFormat, err)
}

// Remaining is the number of unread bytes.
func (r *Reader) Remaining() int { return r.props.Remaining() }

// More reports whether bytes are left and no error happened.
func (r *Reader) More() bool { return r.props.Err() == nil && r.props.Remaining() > 0 }

// U8 reads a byte.
func (r *Reader) U8() uint8 { return r.props.U8() }

// U16 reads a little-endian uint16.
func (r *Reader) U16() uint16 { return r.props.U16() }

// U32 reads a little-endian uint32.
func (r *Reader) U32() uint32 { return r.props.U32() }

// U64 reads a little-endian uint64.
func (r *Reader) U64() uint64 { return r.props.U64() }

// Bytes reads count raw bytes.
func (r *Reader) Bytes(count int) []byte { return r.props.Bytes(count) }

// Skip skips count bytes.
func (r *Reader) Skip(count int) { r.props.Bytes(count) }

// String reads a string with a uint16 length.
func (r *Reader) String() string {
	length := int(r.props.U16())
	return string(r.props.Bytes(length))
}

// LongString reads a string with a uint32 length.
func (r *Reader) LongString() string {
	length := r.props.U32()
	if length > math.MaxInt32 {
		r.props.Fail(fmt.Errorf("%w: string too long", ErrFormat))
		return ""
	}
	return string(r.props.Bytes(int(length)))
}

// Writer writes a node file with escaping.
type Writer struct {
	out     *bufio.Writer
	err     error
	scratch [8]byte
}

// NewWriter writes the identifier and returns a writer.
func NewWriter(dest io.Writer, ident [4]byte) *Writer {
	out := bufio.NewWriterSize(dest, 1<<20)
	_, err := out.Write(ident[:])
	return &Writer{out: out, err: err}
}

// Start opens a node of the given type.
func (w *Writer) Start(nodeType byte) {
	w.raw(nodeStart)
	w.U8(nodeType)
}

// End closes the current node.
func (w *Writer) End() { w.raw(nodeEnd) }

func (w *Writer) raw(value byte) {
	if w.err == nil {
		w.err = w.out.WriteByte(value)
	}
}

// U8 writes an escaped byte.
func (w *Writer) U8(value byte) {
	if value == nodeStart || value == nodeEnd || value == escape {
		w.raw(escape)
	}
	w.raw(value)
}

// Bytes writes escaped bytes.
func (w *Writer) Bytes(data []byte) {
	for _, value := range data {
		w.U8(value)
	}
}

// U16 writes a little-endian uint16.
func (w *Writer) U16(value uint16) {
	binary.LittleEndian.PutUint16(w.scratch[:], value)
	w.Bytes(w.scratch[:2])
}

// U32 writes a little-endian uint32.
func (w *Writer) U32(value uint32) {
	binary.LittleEndian.PutUint32(w.scratch[:], value)
	w.Bytes(w.scratch[:4])
}

// U64 writes a little-endian uint64.
func (w *Writer) U64(value uint64) {
	binary.LittleEndian.PutUint64(w.scratch[:], value)
	w.Bytes(w.scratch[:8])
}

// String writes a string with a uint16 length (truncated to 65535 bytes).
func (w *Writer) String(text string) {
	if len(text) > math.MaxUint16 {
		text = text[:math.MaxUint16]
	}
	w.U16(uint16(len(text)))
	w.Bytes([]byte(text))
}

// LongString writes a string with a uint32 length.
func (w *Writer) LongString(text string) {
	w.U32(uint32(len(text)))
	w.Bytes([]byte(text))
}

// Flush writes buffered data and returns the first error.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	return w.out.Flush()
}
