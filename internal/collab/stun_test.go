package collab

import (
	"encoding/binary"
	"testing"
)

func TestStunReplyWithUnpaddedLastAttribute(t *testing.T) {
	transaction := make([]byte, 12)
	reply := make([]byte, 20, 29)
	binary.BigEndian.PutUint16(reply[0:], 0x0101)
	binary.BigEndian.PutUint16(reply[2:], 9)
	binary.BigEndian.PutUint32(reply[4:], stunMagic)
	reply = append(reply, 0, 0x80, 0, 5, 1, 2, 3, 4, 5)
	if _, ok := parseStunReply(reply, transaction); ok {
		t.Fatal("an address was found in a reply without one")
	}
}
