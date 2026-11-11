// Package collab lets several map editors edit one map together. One of them
// hosts: it holds the map, orders all edits and sends them to the guests,
// who connect to it directly (TLS over TCP). No server of the project is
// involved: guests find hosts by an invite code, on the local network or in
// the list of sessions they joined before.
//
// Edits are shared as the whole new state of the changed tiles, so every
// editor ends with the same map whatever the order of the edits: the
// host's order decides which state of a tile wins.
package collab

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// Protocol is the version of the protocol; editors of other versions
// cannot join each other.
const Protocol = 1

// DefaultPort is the TCP port a host listens on unless set otherwise.
const DefaultPort = 47816

const (
	maxFrame = 64 << 20
	// maxSnapshotData bounds the size of the uncompressed map a guest
	// accepts (a hostile host must not fill the memory).
	maxSnapshotData = 4 << 30
	pingEvery       = 10 * time.Second
	readTimeout     = 45 * time.Second
	dialTimeout     = 8 * time.Second
	handshakeTime   = 20 * time.Second
)

// Frame kinds.
const (
	kindHello byte = iota + 1
	kindWelcome
	kindReject
	kindInfoRequest
	kindInfo
	kindSnapshot
	kindSnapshotDone
	kindEdit
	kindPresence
	kindPeers
	kindChat
	kindPing
	kindBye
)

// Compat describes the item data of an editor: editors share a map only
// when they read item ids the same way.
type Compat struct {
	Client       string `json:"client"`
	DatSignature uint32 `json:"datSignature"`
	SprSignature uint32 `json:"sprSignature"`
	ItemsMode    string `json:"itemsMode"`
	OTB          string `json:"otb"`
	Items        int    `json:"items"`
}

// Mismatch explains why a guest with other cannot join a host with c ("":
// it can).
func (c Compat) Mismatch(other Compat) string {
	switch {
	case c.DatSignature != other.DatSignature || c.SprSignature != other.SprSignature:
		return fmt.Sprintf("the host uses client %s and you use %s: open the same client", c.Client, other.Client)
	case c.ItemsMode != other.ItemsMode:
		return fmt.Sprintf("the host uses %s item ids and you use %s ids: load the same items", c.ItemsMode, other.ItemsMode)
	case c.OTB != other.OTB:
		return fmt.Sprintf("the host uses items.otb %s and you use %s: load the same items.otb", c.OTB, other.OTB)
	case c.Items != other.Items:
		return fmt.Sprintf("the host has %d items and you have %d: load the same items", c.Items, other.Items)
	}
	return ""
}

// SessionInfo describes a hosted session to those who look for one.
type SessionInfo struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Map         string `json:"map"`
	Peers       int    `json:"peers"`
	Password    bool   `json:"password"`
	Protocol    int    `json:"protocol"`
	Port        int    `json:"port"`
	Fingerprint string `json:"fingerprint"`
	Client      string `json:"client"`
}

type Peer struct {
	ID       uint32          `json:"id"`
	Name     string          `json:"name"`
	Color    string          `json:"color"`
	Host     bool            `json:"host"`
	Pos      *otmap.Position `json:"pos"`
	Brush    string          `json:"brush"`
	Address  string          `json:"address,omitempty"`
	JoinedAt int64           `json:"joinedAt"`
}

type hello struct {
	Protocol int    `json:"protocol"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Compat   Compat `json:"compat"`
}

type welcome struct {
	You     uint32      `json:"you"`
	Session SessionInfo `json:"session"`
	Peers   []Peer      `json:"peers"`
}

type reject struct {
	Reason string `json:"reason"`
}

type snapshotDone struct {
	Seq uint64 `json:"seq"`
}

// Presence is where a peer points on the map and what it draws with.
type Presence struct {
	Peer  uint32          `json:"peer"`
	Pos   *otmap.Position `json:"pos"`
	Brush string          `json:"brush"`
}

type Chat struct {
	Peer uint32 `json:"peer"`
	Name string `json:"name"`
	Text string `json:"text"`
	Time int64  `json:"time"`
}

type peersMessage struct {
	Peers []Peer `json:"peers"`
}

const maxChat = 1000

// conn sends and receives frames: a u32 length (of the kind and the
// payload), the kind and the payload.
type conn struct {
	raw     net.Conn
	reader  *bufio.Reader
	writeMu sync.Mutex
}

func newConn(raw net.Conn) *conn {
	return &conn{raw: raw, reader: bufio.NewReaderSize(raw, 64<<10)}
}

func (c *conn) send(kind byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	var header [5]byte
	binary.LittleEndian.PutUint32(header[:4], uint32(len(payload)+1))
	header[4] = kind
	_ = c.raw.SetWriteDeadline(time.Now().Add(readTimeout))
	if _, err := c.raw.Write(header[:]); err != nil {
		return err
	}
	_, err := c.raw.Write(payload)
	return err
}

func (c *conn) sendJSON(kind byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.send(kind, data)
}

func (c *conn) receive(timeout time.Duration) (byte, []byte, error) {
	_ = c.raw.SetReadDeadline(time.Now().Add(timeout))
	var header [4]byte
	if _, err := io.ReadFull(c.reader, header[:]); err != nil {
		return 0, nil, err
	}
	length := binary.LittleEndian.Uint32(header[:])
	if length == 0 || length > maxFrame {
		return 0, nil, errors.New("a message has a wrong size")
	}
	frame := make([]byte, length)
	if _, err := io.ReadFull(c.reader, frame); err != nil {
		return 0, nil, err
	}
	return frame[0], frame[1:], nil
}

// expect receives one frame of kind and decodes its JSON into value.
func (c *conn) expect(kind byte, value any, timeout time.Duration) error {
	got, payload, err := c.receive(timeout)
	if err != nil {
		return err
	}
	if got == kindReject {
		var refused reject
		_ = json.Unmarshal(payload, &refused)
		return &RejectedError{Reason: refused.Reason}
	}
	if got == kindBye {
		var bye reject
		_ = json.Unmarshal(payload, &bye)
		return &RejectedError{Reason: bye.Reason}
	}
	if got != kind {
		return fmt.Errorf("unexpected message %d (expected %d)", got, kind)
	}
	return json.Unmarshal(payload, value)
}

func (c *conn) close() error { return c.raw.Close() }

// RejectedError is the reason the host gave for refusing or ending a
// connection.
type RejectedError struct{ Reason string }

func (e *RejectedError) Error() string { return e.Reason }

// peerColors are the colors of the peers' cursors, by peer id.
var peerColors = []string{"#4fc3f7", "#ffb74d", "#81c784", "#f06292", "#ba68c8", "#fff176", "#4db6ac", "#e57373", "#9575cd", "#aed581"}

func peerColor(id uint32) string { return peerColors[int(id)%len(peerColors)] }

func cleanName(name string, fallback string) string {
	runes := []rune(name)
	out := make([]rune, 0, min(len(runes), 32))
	for _, char := range runes {
		if char >= ' ' && char != 0x7F && len(out) < 32 {
			out = append(out, char)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return string(out)
}
