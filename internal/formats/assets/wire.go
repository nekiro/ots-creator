package assets

import (
	"errors"
	"slices"

	"google.golang.org/protobuf/encoding/protowire"
)

// field is one encoded protobuf field. raw holds the whole field, tag
// included, so unknown fields are written back byte for byte.
type field struct {
	num     protowire.Number
	typ     protowire.Type
	value   uint64 // varint and fixed values
	payload []byte // length-delimited payload
	raw     []byte
}

type message []field

var errWire = errors.New("appearances: malformed protobuf data")

func parse(data []byte) (message, error) {
	var msg message
	for len(data) > 0 {
		num, typ, tagSize := protowire.ConsumeTag(data)
		if tagSize < 0 {
			return nil, errWire
		}
		parsed := field{num: num, typ: typ}
		body := data[tagSize:]
		var bodySize int
		switch typ {
		case protowire.VarintType:
			parsed.value, bodySize = protowire.ConsumeVarint(body)
		case protowire.Fixed32Type:
			var value uint32
			value, bodySize = protowire.ConsumeFixed32(body)
			parsed.value = uint64(value)
		case protowire.Fixed64Type:
			parsed.value, bodySize = protowire.ConsumeFixed64(body)
		case protowire.BytesType:
			parsed.payload, bodySize = protowire.ConsumeBytes(body)
		default:
			bodySize = protowire.ConsumeFieldValue(num, typ, body)
		}
		if bodySize < 0 {
			return nil, errWire
		}
		parsed.raw = data[:tagSize+bodySize]
		msg = append(msg, parsed)
		data = data[tagSize+bodySize:]
	}
	return msg, nil
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
	found, _ := m.get(num)
	return found.value
}

func (m message) bool(num protowire.Number) bool { return m.uint(num) != 0 }

func (m message) sub(num protowire.Number) message {
	found, ok := m.get(num)
	if !ok || found.typ != protowire.BytesType {
		return nil
	}
	nested, _ := parse(found.payload)
	return nested
}

// uints returns every value of a repeated varint field, packed or not.
func (m message) uints(num protowire.Number) []uint64 {
	var out []uint64
	for _, entry := range m {
		if entry.num != num {
			continue
		}
		if entry.typ == protowire.VarintType {
			out = append(out, entry.value)
			continue
		}
		for packed := entry.payload; len(packed) > 0; {
			value, size := protowire.ConsumeVarint(packed)
			if size < 0 {
				break
			}
			out = append(out, value)
			packed = packed[size:]
		}
	}
	return out
}

func (m message) all(num protowire.Number) []message {
	var out []message
	for _, entry := range m {
		if entry.num == num && entry.typ == protowire.BytesType {
			nested, _ := parse(entry.payload)
			out = append(out, nested)
		}
	}
	return out
}

// appendOthers appends the raw fields whose number is not in known.
func (m message) appendOthers(out []byte, known ...protowire.Number) []byte {
	for _, entry := range m {
		if !slices.Contains(known, entry.num) {
			out = append(out, entry.raw...)
		}
	}
	return out
}

func putUint(out []byte, num protowire.Number, value uint64) []byte {
	out = protowire.AppendTag(out, num, protowire.VarintType)
	return protowire.AppendVarint(out, value)
}

// putInt writes a proto int32 (negative values are sign extended).
func putInt(out []byte, num protowire.Number, value int32) []byte {
	return putUint(out, num, uint64(int64(value)))
}

func putBool(out []byte, num protowire.Number, value bool) []byte {
	if !value {
		return out
	}
	return putUint(out, num, 1)
}

func putBytes(out []byte, num protowire.Number, value []byte) []byte {
	out = protowire.AppendTag(out, num, protowire.BytesType)
	return protowire.AppendBytes(out, value)
}
