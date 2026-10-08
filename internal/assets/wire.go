package assets

import (
	"errors"
	"slices"

	"google.golang.org/protobuf/encoding/protowire"
)

// field is one encoded protobuf field. raw holds the whole field, tag
// included, so unknown fields are written back byte for byte.
type field struct {
	num protowire.Number
	typ protowire.Type
	v   uint64 // varint and fixed values
	b   []byte // length-delimited payload
	raw []byte
}

type message []field

var errWire = errors.New("appearances: malformed protobuf data")

func parse(b []byte) (message, error) {
	var m message
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, errWire
		}
		f := field{num: num, typ: typ}
		m0 := n
		switch typ {
		case protowire.VarintType:
			f.v, n = protowire.ConsumeVarint(b[m0:])
		case protowire.Fixed32Type:
			var v uint32
			v, n = protowire.ConsumeFixed32(b[m0:])
			f.v = uint64(v)
		case protowire.Fixed64Type:
			f.v, n = protowire.ConsumeFixed64(b[m0:])
		case protowire.BytesType:
			f.b, n = protowire.ConsumeBytes(b[m0:])
		default:
			n = protowire.ConsumeFieldValue(num, typ, b[m0:])
		}
		if n < 0 {
			return nil, errWire
		}
		f.raw = b[:m0+n]
		m = append(m, f)
		b = b[m0+n:]
	}
	return m, nil
}

// get returns the last occurrence of a field (proto2 semantics).
func (m message) get(num protowire.Number) (field, bool) {
	for i := len(m) - 1; i >= 0; i-- {
		if m[i].num == num {
			return m[i], true
		}
	}
	return field{}, false
}

func (m message) has(num protowire.Number) bool {
	_, ok := m.get(num)
	return ok
}

func (m message) uint(num protowire.Number) uint64 {
	f, _ := m.get(num)
	return f.v
}

func (m message) bool(num protowire.Number) bool { return m.uint(num) != 0 }

func (m message) sub(num protowire.Number) message {
	f, ok := m.get(num)
	if !ok || f.typ != protowire.BytesType {
		return nil
	}
	s, _ := parse(f.b)
	return s
}

// uints returns every value of a repeated varint field, packed or not.
func (m message) uints(num protowire.Number) []uint64 {
	var out []uint64
	for _, f := range m {
		if f.num != num {
			continue
		}
		if f.typ == protowire.VarintType {
			out = append(out, f.v)
			continue
		}
		for b := f.b; len(b) > 0; {
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				break
			}
			out = append(out, v)
			b = b[n:]
		}
	}
	return out
}

func (m message) all(num protowire.Number) []message {
	var out []message
	for _, f := range m {
		if f.num == num && f.typ == protowire.BytesType {
			s, _ := parse(f.b)
			out = append(out, s)
		}
	}
	return out
}

// appendOthers appends the raw fields whose number is not in known.
func (m message) appendOthers(b []byte, known ...protowire.Number) []byte {
	for _, f := range m {
		if !slices.Contains(known, f.num) {
			b = append(b, f.raw...)
		}
	}
	return b
}

func putUint(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// putInt writes a proto int32 (negative values are sign extended).
func putInt(b []byte, num protowire.Number, v int32) []byte {
	return putUint(b, num, uint64(int64(v)))
}

func putBool(b []byte, num protowire.Number, v bool) []byte {
	if !v {
		return b
	}
	return putUint(b, num, 1)
}

func putBytes(b []byte, num protowire.Number, v []byte) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}
