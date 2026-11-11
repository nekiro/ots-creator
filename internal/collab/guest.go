package collab

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// GuestHooks connect a guest to its copy of the map. They run on the
// connection goroutine.
type GuestHooks struct {
	// Edit gets every edit of the host in order; apply what Accept returns
	// under the lock of the map.
	Edit     func(edit *Edit)
	Presence func(Presence)
	Peers    func([]Peer)
	Chat     func(Chat)
	// Closed is called once when the connection ends (err: why).
	Closed func(err error)
}

// Guest is the connection of a guest to a host.
type Guest struct {
	conn        *conn
	out         *outbox
	you         uint32
	session     SessionInfo
	address     string
	fingerprint string

	mu    sync.Mutex
	peers []Peer
	local uint64
	// sent keeps the tiles of own edits the host has not confirmed yet;
	// pending counts them by position. Edits of others to those tiles are
	// skipped: the own edit comes later in the host's order and wins.
	sent        map[uint64]sentEdit
	pending     map[otmap.Position]int
	pendingMeta int
	leaving     bool
}

type sentEdit struct {
	tiles []otmap.Position
	meta  bool
}

// ErrUnreachable means no address of the invite answered.
var ErrUnreachable = errors.New("cannot reach the host: check the address, that the host's firewall allows OTS Creator and that the port is open on the host's router (or use the same VPN, like Tailscale, ZeroTier or Radmin VPN)")

// dial connects to the first address of the invite that answers with the
// right certificate.
func dial(ctx context.Context, invite Invite) (*conn, string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	type result struct {
		conn        *tls.Conn
		address     string
		fingerprint string
		err         error
	}
	results := make(chan result, len(invite.Addresses))
	for _, address := range invite.Addresses {
		go func() {
			var seen string
			dialer := &tls.Dialer{NetDialer: &net.Dialer{}, Config: clientTLS(invite.Fingerprint, &seen)}
			raw, err := dialer.DialContext(ctx, "tcp", address)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{conn: raw.(*tls.Conn), address: address, fingerprint: seen}
		}()
	}
	var firstErr error
	var won *result
	for range invite.Addresses {
		got := <-results
		if got.err != nil {
			if firstErr == nil || errors.Is(got.err, ErrFingerprint) {
				firstErr = got.err
			}
			continue
		}
		if won != nil {
			_ = got.conn.Close()
			continue
		}
		won = &got
		cancel()
	}
	if won == nil {
		if errors.Is(firstErr, ErrFingerprint) {
			return nil, "", "", ErrFingerprint
		}
		return nil, "", "", ErrUnreachable
	}
	return newConn(won.conn), won.address, won.fingerprint, nil
}

// Probe asks a host about its session without joining.
func Probe(ctx context.Context, invite Invite) (SessionInfo, error) {
	c, _, _, err := dial(ctx, invite)
	if err != nil {
		return SessionInfo{}, err
	}
	defer c.close()
	if err := c.send(kindInfoRequest, nil); err != nil {
		return SessionInfo{}, err
	}
	var info SessionInfo
	err = c.expect(kindInfo, &info, dialTimeout)
	return info, err
}

// Join connects to a host and receives the map. Call Start on the guest
// once the map is open to get the edits that follow.
func Join(ctx context.Context, invite Invite, name string, compat Compat, progress func(done, total int)) (*Guest, *Snapshot, error) {
	c, address, fingerprint, err := dial(ctx, invite)
	if err != nil {
		return nil, nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = c.close() })
	defer stop()
	fail := func(err error) (*Guest, *Snapshot, error) {
		_ = c.close()
		if ctx.Err() != nil {
			return nil, nil, context.Canceled
		}
		return nil, nil, err
	}
	if err := c.sendJSON(kindHello, hello{Protocol: Protocol, Name: name, Password: invite.Password, Compat: compat}); err != nil {
		return fail(err)
	}
	var greeting welcome
	if err := c.expect(kindWelcome, &greeting, handshakeTime); err != nil {
		return fail(err)
	}
	reader := &snapshotReader{}
	for {
		kind, payload, err := c.receive(readTimeout)
		if err != nil {
			return fail(err)
		}
		switch kind {
		case kindSnapshot:
			done, total, err := reader.add(payload)
			if err != nil {
				return fail(err)
			}
			if progress != nil {
				progress(done, total)
			}
			continue
		case kindSnapshotDone:
			var end snapshotDone
			if json.Unmarshal(payload, &end) != nil || !reader.complete() {
				return fail(errSnapshot)
			}
			reader.snapshot.Seq = end.Seq
		case kindReject, kindBye:
			var refused reject
			_ = json.Unmarshal(payload, &refused)
			return fail(&RejectedError{Reason: refused.Reason})
		case kindPing:
			continue
		default:
			return fail(errSnapshot)
		}
		break
	}
	guest := &Guest{
		conn: c, out: newOutbox(), you: greeting.You, session: greeting.Session, address: address, fingerprint: fingerprint,
		peers: greeting.Peers, sent: map[uint64]sentEdit{}, pending: map[otmap.Position]int{},
	}
	return guest, &reader.snapshot, nil
}

func (g *Guest) You() uint32 { return g.you }

func (g *Guest) Session() SessionInfo { return g.session }

// Address is the address the guest connected to.
func (g *Guest) Address() string { return g.address }

// Fingerprint is the fingerprint of the host's certificate.
func (g *Guest) Fingerprint() string { return g.fingerprint }

func (g *Guest) Peers() []Peer {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.peers)
}

// Start handles the messages of the host until the connection ends.
func (g *Guest) Start(hooks GuestHooks) {
	writerDone := make(chan error, 1)
	go func() { writerDone <- g.out.run(g.conn) }()
	go func() {
		err := g.read(hooks)
		g.out.close()
		_ = g.conn.close()
		if writeErr := <-writerDone; err == nil {
			err = writeErr
		}
		g.mu.Lock()
		leaving := g.leaving
		g.mu.Unlock()
		if leaving {
			err = nil
		}
		if hooks.Closed != nil {
			hooks.Closed(err)
		}
	}()
}

func (g *Guest) read(hooks GuestHooks) error {
	for {
		kind, payload, err := g.conn.receive(readTimeout)
		if err != nil {
			return err
		}
		switch kind {
		case kindEdit:
			edit, err := decodeEdit(payload)
			if err != nil {
				return err
			}
			hooks.Edit(edit)
		case kindPresence:
			var presence Presence
			if json.Unmarshal(payload, &presence) != nil {
				continue
			}
			g.mu.Lock()
			for i := range g.peers {
				if g.peers[i].ID == presence.Peer {
					g.peers[i].Pos, g.peers[i].Brush = presence.Pos, presence.Brush
				}
			}
			g.mu.Unlock()
			if hooks.Presence != nil {
				hooks.Presence(presence)
			}
		case kindPeers:
			var message peersMessage
			if json.Unmarshal(payload, &message) != nil {
				continue
			}
			g.mu.Lock()
			g.peers = message.Peers
			g.mu.Unlock()
			if hooks.Peers != nil {
				hooks.Peers(message.Peers)
			}
		case kindChat:
			var chat Chat
			if json.Unmarshal(payload, &chat) == nil && hooks.Chat != nil {
				hooks.Chat(chat)
			}
		case kindBye:
			var bye reject
			_ = json.Unmarshal(payload, &bye)
			return &RejectedError{Reason: bye.Reason}
		}
	}
}

// Send sends an own edit to the host. Call it under the lock of the map,
// right after the edit.
func (g *Guest) Send(commit mapedit.Commit) {
	g.mu.Lock()
	g.local++
	record := sentEdit{tiles: make([]otmap.Position, len(commit.Tiles)), meta: commit.Meta != nil}
	for i, state := range commit.Tiles {
		record.tiles[i] = state.Pos
		g.pending[state.Pos]++
	}
	if record.meta {
		g.pendingMeta++
	}
	g.sent[g.local] = record
	edit := &Edit{Local: g.local, Label: commit.Label, Tiles: commit.Tiles, Meta: commit.Meta}
	g.mu.Unlock()
	g.out.push(kindEdit, encodeEdit(edit))
}

// Accept returns what of an edit of the host to apply to the map: nothing
// of own edits (already applied) and no tiles with own edits on the way.
// Call it under the lock of the map.
func (g *Guest) Accept(edit *Edit) ([]mapedit.TileState, *mapedit.MetaData) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if edit.Author == g.you {
		if record, ok := g.sent[edit.Local]; ok {
			delete(g.sent, edit.Local)
			for _, pos := range record.tiles {
				if g.pending[pos]--; g.pending[pos] <= 0 {
					delete(g.pending, pos)
				}
			}
			if record.meta {
				g.pendingMeta--
			}
		}
		return nil, nil
	}
	tiles := edit.Tiles
	if len(g.pending) > 0 {
		tiles = slices.DeleteFunc(tiles, func(state mapedit.TileState) bool { return g.pending[state.Pos] > 0 })
	}
	meta := edit.Meta
	if g.pendingMeta > 0 {
		meta = nil
	}
	return tiles, meta
}

// SetPresence shares where the guest points on the map.
func (g *Guest) SetPresence(pos *otmap.Position, brush string) {
	data, _ := json.Marshal(Presence{Pos: pos, Brush: brush})
	g.out.push(kindPresence, data)
}

func (g *Guest) SendChat(text string) {
	data, _ := json.Marshal(Chat{Text: limitText(text)})
	g.out.push(kindChat, data)
}

// Leave ends the connection.
func (g *Guest) Leave() {
	g.mu.Lock()
	g.leaving = true
	g.mu.Unlock()
	g.out.push(kindBye, []byte("{}"))
	g.out.close()
	time.AfterFunc(2*time.Second, func() { _ = g.conn.close() })
}
