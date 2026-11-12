package collab

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// maxPeers bounds the guests of one session.
const maxPeers = 32

// HostID is the peer id of the host itself.
const HostID uint32 = 1

type HostConfig struct {
	// Session is the name of the session, Name the host's own name.
	Session     string
	Name        string
	Password    string
	Port        int
	Certificate tls.Certificate
	Fingerprint string
	Compat      Compat
	// Map returns the name of the shared map.
	Map func() string
}

// HostHooks connect the host to the map. Snapshot and Edit run on the
// connection goroutines.
type HostHooks struct {
	// Snapshot encodes the map for a joining guest. It must call attach
	// while the map cannot change: the guest gets every edit published
	// after that.
	Snapshot func(attach func()) (world []byte, sectors map[string][]byte, err error)
	// Edit applies the edit of a guest to the map and publishes it with
	// Publish, both under the lock of the map.
	Edit func(edit *Edit) error
	// Peers is called when guests join or leave.
	Peers    func()
	Presence func(Presence)
	Chat     func(Chat)
}

type Host struct {
	config   HostConfig
	hooks    HostHooks
	listener net.Listener

	mu     sync.Mutex
	peers  map[uint32]*hostPeer
	self   Peer
	nextID uint32
	seq    uint64
	closed bool
	wg     sync.WaitGroup
}

type hostPeer struct {
	Peer
	conn     *conn
	out      *outbox
	attached bool
}

// Listen starts hosting on the port of config (0: any free port).
func Listen(config HostConfig, hooks HostHooks) (*Host, error) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(config.Port))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on port %d: %w", config.Port, err)
	}
	config.Port = listener.Addr().(*net.TCPAddr).Port
	host := &Host{
		config: config, hooks: hooks, listener: listener, peers: map[uint32]*hostPeer{}, nextID: HostID,
		self: Peer{ID: HostID, Name: config.Name, Color: peerColor(HostID), Host: true, JoinedAt: time.Now().UnixMilli()},
	}
	host.wg.Add(1)
	go host.accept()
	return host, nil
}

func (h *Host) Port() int { return h.config.Port }

func (h *Host) Fingerprint() string { return h.config.Fingerprint }

// Info describes the session to those who look for one.
func (h *Host) Info() SessionInfo {
	h.mu.Lock()
	peers := 1 + len(h.peers)
	h.mu.Unlock()
	mapName := ""
	if h.config.Map != nil {
		mapName = h.config.Map()
	}
	return SessionInfo{
		Name: h.config.Session, Host: h.config.Name, Map: mapName, Peers: peers, Password: h.config.Password != "",
		Protocol: Protocol, Port: h.config.Port, Fingerprint: h.config.Fingerprint, Client: h.config.Compat.Client,
	}
}

// Peers lists the host and the guests in the order they joined; for the
// host the guests have their addresses.
func (h *Host) Peers() []Peer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.peerListLocked(true)
}

func (h *Host) peerListLocked(addresses bool) []Peer {
	list := []Peer{h.self}
	for _, peer := range h.peers {
		if !peer.attached {
			continue
		}
		entry := peer.Peer
		if !addresses {
			entry.Address = ""
		}
		list = append(list, entry)
	}
	slices.SortFunc(list, func(a, b Peer) int { return int(a.ID) - int(b.ID) })
	return list
}

// Publish gives the edit the next number and sends it to every guest
// (its author takes it as the confirmation). Call it under the lock of
// the map, in the order the edits were applied.
func (h *Host) Publish(edit *Edit) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	edit.Seq = h.seq
	if edit.Author == 0 {
		edit.Author = HostID
	}
	payload := encodeEdit(edit)
	for _, peer := range h.peers {
		if peer.attached {
			peer.out.push(kindEdit, payload)
		}
	}
}

// SetPresence shares where the host points on the map.
func (h *Host) SetPresence(pos *otmap.Position, brush string) {
	h.mu.Lock()
	h.self.Pos, h.self.Brush = pos, cleanName(brush, "")
	presence := Presence{Peer: HostID, Pos: pos, Brush: h.self.Brush}
	h.broadcastLocked(kindPresence, presence, 0)
	h.mu.Unlock()
}

// SendChat sends a chat message of the host.
func (h *Host) SendChat(text string) Chat {
	chat := Chat{Peer: HostID, Name: h.config.Name, Text: limitText(text), Time: time.Now().UnixMilli()}
	h.mu.Lock()
	h.broadcastLocked(kindChat, chat, 0)
	h.mu.Unlock()
	return chat
}

// Kick disconnects a guest.
func (h *Host) Kick(id uint32) {
	h.mu.Lock()
	peer := h.peers[id]
	h.mu.Unlock()
	if peer != nil {
		h.bye(peer, "the host removed you from the session")
	}
}

func (h *Host) bye(peer *hostPeer, reason string) {
	data, _ := json.Marshal(reject{Reason: reason})
	peer.out.push(kindBye, data)
	peer.out.close()
	// The writer sends the goodbye and stops; the connection closes when it
	// did or after a moment.
	time.AfterFunc(2*time.Second, func() { _ = peer.conn.close() })
}

// Close ends the session for everyone.
func (h *Host) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	peers := make([]*hostPeer, 0, len(h.peers))
	for _, peer := range h.peers {
		peers = append(peers, peer)
	}
	h.mu.Unlock()
	_ = h.listener.Close()
	for _, peer := range peers {
		h.bye(peer, "the host ended the session")
	}
}

func (h *Host) broadcastLocked(kind byte, value any, except uint32) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	for _, peer := range h.peers {
		if peer.attached && peer.ID != except {
			peer.out.push(kind, data)
		}
	}
}

func (h *Host) peersChanged() {
	h.mu.Lock()
	h.broadcastLocked(kindPeers, peersMessage{Peers: h.peerListLocked(false)}, 0)
	h.mu.Unlock()
	if h.hooks.Peers != nil {
		h.hooks.Peers()
	}
}

func (h *Host) accept() {
	defer h.wg.Done()
	for {
		raw, err := h.listener.Accept()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return
		}
		go h.serve(raw)
	}
}

func (h *Host) serve(raw net.Conn) {
	tlsConn := tls.Server(raw, serverTLS(h.config.Certificate))
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTime)
	err := tlsConn.HandshakeContext(ctx)
	cancel()
	if err != nil {
		_ = raw.Close()
		return
	}
	c := newConn(tlsConn)
	kind, payload, err := c.receiveUpTo(maxHello, handshakeTime)
	if err != nil {
		_ = c.close()
		return
	}
	switch kind {
	case kindInfoRequest:
		_ = c.sendJSON(kindInfo, h.Info())
		_ = c.close()
		return
	case kindHello:
	default:
		_ = c.close()
		return
	}
	var greeting hello
	if err := json.Unmarshal(payload, &greeting); err != nil {
		_ = c.close()
		return
	}
	if reason := h.check(greeting); reason != "" {
		_ = c.sendJSON(kindReject, reject{Reason: reason})
		_ = c.close()
		return
	}
	h.join(c, greeting, raw.RemoteAddr().String())
}

// check returns why a guest cannot join ("": it can).
func (h *Host) check(greeting hello) string {
	if greeting.Protocol != Protocol {
		return "the host uses another version of OTS Creator: update both to the same version"
	}
	if subtle.ConstantTimeCompare([]byte(greeting.Password), []byte(h.config.Password)) != 1 {
		// Slows down guessing.
		time.Sleep(time.Second)
		if greeting.Password == "" {
			return "the session needs a password"
		}
		return "wrong password"
	}
	if reason := h.config.Compat.Mismatch(greeting.Compat); reason != "" {
		return reason
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return "the session has ended"
	}
	if len(h.peers) >= maxPeers {
		return "the session is full"
	}
	return ""
}

func (h *Host) join(c *conn, greeting hello, address string) {
	h.mu.Lock()
	h.nextID++
	peer := &hostPeer{
		Peer: Peer{
			ID: h.nextID, Name: cleanName(greeting.Name, "Guest "+strconv.Itoa(int(h.nextID))), Color: peerColor(h.nextID),
			Address: address, JoinedAt: time.Now().UnixMilli(),
		},
		conn: c, out: newOutbox(),
	}
	h.peers[peer.ID] = peer
	h.mu.Unlock()
	defer func() {
		peer.out.close()
		_ = c.close()
		h.mu.Lock()
		delete(h.peers, peer.ID)
		h.mu.Unlock()
		h.peersChanged()
	}()

	if err := c.sendJSON(kindWelcome, welcome{You: peer.ID, Session: h.Info(), Peers: h.Peers()}); err != nil {
		return
	}
	var seq uint64
	world, sectors, err := h.hooks.Snapshot(func() {
		h.mu.Lock()
		peer.attached = true
		seq = h.seq
		h.mu.Unlock()
	})
	if err != nil {
		_ = c.sendJSON(kindReject, reject{Reason: "the host cannot send the map: " + err.Error()})
		return
	}
	writerDone := make(chan error, 1)
	go func() {
		parts, err := encodeSnapshot(world, sectors)
		if err == nil {
			for _, part := range parts {
				if err = c.send(kindSnapshot, part); err != nil {
					break
				}
			}
		}
		if err == nil {
			data, _ := json.Marshal(snapshotDone{Seq: seq})
			err = c.send(kindSnapshotDone, data)
		}
		if err != nil {
			_ = c.close()
			writerDone <- err
			return
		}
		writerDone <- peer.out.run(c)
		_ = c.close()
	}()
	h.peersChanged()
	h.read(peer)
	// The writer sends what is queued (a goodbye) and stops; a guest that
	// is gone does not get a long wait.
	peer.out.close()
	var writeErr error
	select {
	case writeErr = <-writerDone:
	case <-time.After(2 * time.Second):
		_ = c.close()
		writeErr = <-writerDone
	}
	if writeErr != nil && !errors.Is(writeErr, net.ErrClosed) {
		log.Printf("collab: %s: %v", peer.Name, writeErr)
	}
}

// read handles the messages of a guest until it leaves.
func (h *Host) read(peer *hostPeer) {
	for {
		kind, payload, err := peer.conn.receive(readTimeout)
		if err != nil {
			return
		}
		switch kind {
		case kindEdit:
			edit, err := decodeEdit(payload)
			if err != nil {
				h.bye(peer, "your editor sent a damaged edit")
				return
			}
			edit.Author, edit.Seq = peer.ID, 0
			if err := h.hooks.Edit(edit); err != nil {
				h.bye(peer, "the host could not apply your edit: "+err.Error())
				return
			}
		case kindPresence:
			var presence Presence
			if json.Unmarshal(payload, &presence) != nil {
				continue
			}
			if presence.Pos != nil && presence.Pos.Z > otmap.MaxFloor {
				presence.Pos = nil
			}
			presence.Peer = peer.ID
			presence.Brush = cleanName(presence.Brush, "")
			h.mu.Lock()
			peer.Pos, peer.Brush = presence.Pos, presence.Brush
			h.broadcastLocked(kindPresence, presence, peer.ID)
			h.mu.Unlock()
			if h.hooks.Presence != nil {
				h.hooks.Presence(presence)
			}
		case kindChat:
			var chat Chat
			if json.Unmarshal(payload, &chat) != nil {
				continue
			}
			chat = Chat{Peer: peer.ID, Name: peer.Name, Text: limitText(chat.Text), Time: time.Now().UnixMilli()}
			if chat.Text == "" {
				continue
			}
			h.mu.Lock()
			h.broadcastLocked(kindChat, chat, 0)
			h.mu.Unlock()
			if h.hooks.Chat != nil {
				h.hooks.Chat(chat)
			}
		case kindBye:
			return
		}
	}
}

func limitText(text string) string {
	runes := []rune(text)
	if len(runes) > maxChat {
		runes = runes[:maxChat]
	}
	return string(runes)
}
