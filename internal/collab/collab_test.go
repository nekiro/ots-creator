package collab

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/core/otitems"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func testItems() *otitems.DB {
	var types []*otitems.Type
	for id := uint16(100); id < 140; id++ {
		flags := otitems.Flags(0)
		if id < 110 {
			flags = otitems.FlagGround
		}
		types = append(types, &otitems.Type{ID: id, ClientID: id, Flags: flags, Name: "item " + strconv.Itoa(int(id))})
	}
	return otitems.NewDB(otitems.ModeOTB, types)
}

var testCompat = Compat{Client: "test", DatSignature: 1, SprSignature: 2, ItemsMode: "otb", Items: 40}

// editorSide is one editor of a test session: a map behind a lock, as the
// MapService keeps it.
type editorSide struct {
	mu     sync.Mutex
	editor *mapedit.Editor
}

func (side *editorSide) edit(fn func(editor *mapedit.Editor)) {
	side.mu.Lock()
	defer side.mu.Unlock()
	fn(side.editor)
}

type testSession struct {
	items  *otitems.DB
	host   *Host
	sides  []*editorSide
	guests []*Guest
}

func startHost(t *testing.T, gameMap *otmap.Map, password string) *testSession {
	t.Helper()
	session := &testSession{items: testItems()}
	hostSide := &editorSide{editor: mapedit.New(gameMap, session.items, nil)}
	session.sides = append(session.sides, hostSide)
	cert, fingerprint, err := LoadCertificate("")
	if err != nil {
		t.Fatal(err)
	}
	var host *Host
	host, err = Listen(HostConfig{
		Session: "test", Name: "host", Password: password, Port: 0, Certificate: cert, Fingerprint: fingerprint, Compat: testCompat,
	}, HostHooks{
		Snapshot: func(attach func()) ([]byte, map[string][]byte, error) {
			hostSide.mu.Lock()
			defer hostSide.mu.Unlock()
			attach()
			return otmap.EncodeOTMAP(hostSide.editor.Map, session.items)
		},
		Edit: func(edit *Edit) error {
			hostSide.mu.Lock()
			defer hostSide.mu.Unlock()
			hostSide.editor.ApplyRemote(edit.Tiles, edit.Meta)
			host.Publish(edit)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	hostSide.editor.Observe(func(commit mapedit.Commit) {
		host.Publish(&Edit{Label: commit.Label, Tiles: commit.Tiles, Meta: commit.Meta})
	})
	session.host = host
	t.Cleanup(func() {
		for _, guest := range session.guests {
			guest.Leave()
		}
		host.Close()
	})
	return session
}

func (session *testSession) invite(password string) Invite {
	return Invite{
		Addresses:   []string{net.JoinHostPort("127.0.0.1", strconv.Itoa(session.host.Port()))},
		Fingerprint: session.host.Fingerprint(), Password: password,
	}
}

func (session *testSession) join(t *testing.T, name string, invite Invite) (*editorSide, error) {
	t.Helper()
	guest, snapshot, err := Join(context.Background(), invite, name, testCompat, nil)
	if err != nil {
		return nil, err
	}
	gameMap, _, err := otmap.DecodeOTMAP(snapshot.World, snapshot.Sectors, session.items, nil)
	if err != nil {
		t.Fatal(err)
	}
	side := &editorSide{editor: mapedit.New(gameMap, session.items, nil)}
	side.editor.Observe(guest.Send)
	guest.Start(GuestHooks{Edit: func(edit *Edit) {
		side.mu.Lock()
		defer side.mu.Unlock()
		tiles, meta := guest.Accept(edit)
		side.editor.ApplyRemote(tiles, meta)
	}})
	session.sides = append(session.sides, side)
	session.guests = append(session.guests, guest)
	return side, nil
}

// differences lists the tiles in the area that differ between two maps.
func differences(first, second *otmap.Map, size int) []string {
	var out []string
	for z := range 2 {
		for y := range size {
			for x := range size {
				pos := otmap.Position{X: uint16(x), Y: uint16(y), Z: uint8(6 + z)}
				if !mapedit.SameTile(first.Tile(pos), second.Tile(pos)) {
					out = append(out, fmt.Sprintf("%v: %+v vs %+v", pos, first.Tile(pos), second.Tile(pos)))
				}
			}
		}
	}
	if len(first.Towns) != len(second.Towns) {
		out = append(out, fmt.Sprintf("towns: %d vs %d", len(first.Towns), len(second.Towns)))
	}
	return out
}

// waitSame waits until every editor has the map of the host.
func (session *testSession) waitSame(t *testing.T, size int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var diff []string
		host := session.sides[0]
		host.mu.Lock()
		for i, side := range session.sides[1:] {
			side.mu.Lock()
			if found := differences(host.editor.Map, side.editor.Map, size); len(found) > 0 {
				diff = append(diff, fmt.Sprintf("guest %d: %d tiles differ, first %s", i+1, len(found), found[0]))
			}
			side.mu.Unlock()
		}
		host.mu.Unlock()
		if len(diff) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the maps differ: %v", diff)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSnapshotAndEdits(t *testing.T) {
	gameMap := otmap.New(256, 256)
	gameMap.SetTile(otmap.Position{X: 3, Y: 4, Z: 7}, &otmap.Tile{
		Ground: otmap.Item{ID: 100}, Items: []otmap.Item{{ID: 120, Subtype: 3}}, HouseID: 5,
		Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat", SpawnTime: 60}}, Spawn: &otmap.Spawn{Radius: 3}},
	})
	gameMap.Houses[5] = &otmap.House{ID: 5, Name: "Flat", TownID: 1}
	gameMap.Towns[1] = &otmap.Town{ID: 1, Name: "Town", Temple: otmap.Position{X: 3, Y: 4, Z: 7}}
	session := startHost(t, gameMap, "secret")
	guest, err := session.join(t, "guest", session.invite("secret"))
	if err != nil {
		t.Fatal(err)
	}
	session.waitSame(t, 16)
	if got := guest.editor.Map.Houses[5]; got == nil || got.Name != "Flat" {
		t.Fatalf("house not received: %+v", got)
	}

	// An edit of the guest reaches the host, one of the host the guest.
	guest.edit(func(editor *mapedit.Editor) {
		tx := editor.Begin("Draw", 0)
		tx.Edit(otmap.Position{X: 1, Y: 1, Z: 7}).Ground = otmap.Item{ID: 101}
		tx.Commit()
	})
	session.sides[0].edit(func(editor *mapedit.Editor) {
		tx := editor.Begin("Draw", 0)
		tx.Edit(otmap.Position{X: 2, Y: 1, Z: 7}).Ground = otmap.Item{ID: 102}
		tx.Meta()
		editor.Map.Towns[2] = &otmap.Town{ID: 2, Name: "Other"}
		tx.Commit()
	})
	session.waitSame(t, 16)
	if guest.editor.Map.Towns[2] == nil {
		t.Fatal("the new town did not reach the guest")
	}
	if !session.sides[0].editor.Changed() {
		t.Fatal("an edit of a guest does not mark the host map changed")
	}
}

func TestConcurrentEditsConverge(t *testing.T) {
	const size = 6
	session := startHost(t, otmap.New(64, 64), "")
	for i := range 3 {
		if _, err := session.join(t, "guest"+strconv.Itoa(i), session.invite("")); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for index, side := range session.sides {
		wg.Add(1)
		go func() {
			defer wg.Done()
			random := rand.New(rand.NewPCG(uint64(index), 7))
			for step := range 300 {
				side.edit(func(editor *mapedit.Editor) {
					if step%17 == 16 && editor.CanUndo() {
						editor.Undo()
						return
					}
					if step%23 == 22 && editor.CanRedo() {
						editor.Redo()
						return
					}
					tx := editor.Begin("Draw", 0)
					for range 1 + random.IntN(4) {
						pos := otmap.Position{X: uint16(random.IntN(size)), Y: uint16(random.IntN(size)), Z: uint8(6 + random.IntN(2))}
						if random.IntN(8) == 0 {
							tile := tx.Edit(pos)
							*tile = otmap.Tile{}
							continue
						}
						tile := tx.Edit(pos)
						tile.Ground = otmap.Item{ID: uint16(100 + random.IntN(10))}
						tile.Items = append(tile.Items, otmap.Item{ID: uint16(110 + random.IntN(30))})
					}
					if random.IntN(20) == 0 {
						tx.Meta()
						id := uint32(random.IntN(5) + 1)
						editor.Map.Towns[id] = &otmap.Town{ID: id, Name: "Town " + strconv.Itoa(index)}
					}
					tx.Commit()
				})
				if random.IntN(4) == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
	session.waitSame(t, size)
}

func TestJoinRefused(t *testing.T) {
	session := startHost(t, otmap.New(64, 64), "secret")
	var rejected *RejectedError
	if _, err := session.join(t, "guest", session.invite("wrong")); !errors.As(err, &rejected) {
		t.Fatalf("a wrong password joined: %v", err)
	}
	other := testCompat
	other.Items = 41
	_, _, err := Join(context.Background(), session.invite("secret"), "guest", other, nil)
	if !errors.As(err, &rejected) {
		t.Fatalf("other items joined: %v", err)
	}
	invite := session.invite("secret")
	invite.Fingerprint = "00112233445566778899aabbccddeeff"
	if _, _, err := Join(context.Background(), invite, "guest", testCompat, nil); !errors.Is(err, ErrFingerprint) {
		t.Fatalf("a wrong fingerprint joined: %v", err)
	}
	info, err := Probe(context.Background(), session.invite(""))
	if err != nil || !info.Password || info.Name != "test" {
		t.Fatalf("probe: %+v %v", info, err)
	}
}

func TestUndoKeepsOthersEdits(t *testing.T) {
	session := startHost(t, otmap.New(64, 64), "")
	guest, err := session.join(t, "guest", session.invite(""))
	if err != nil {
		t.Fatal(err)
	}
	pos := otmap.Position{X: 1, Y: 1, Z: 7}
	other := otmap.Position{X: 2, Y: 1, Z: 7}
	guest.edit(func(editor *mapedit.Editor) {
		tx := editor.Begin("Draw", 0)
		tx.Edit(pos).Ground = otmap.Item{ID: 101}
		tx.Edit(other).Ground = otmap.Item{ID: 101}
		tx.Commit()
	})
	session.waitSame(t, 4)
	session.sides[0].edit(func(editor *mapedit.Editor) {
		tx := editor.Begin("Draw", 0)
		tx.Edit(pos).Ground = otmap.Item{ID: 105}
		tx.Commit()
	})
	session.waitSame(t, 4)
	guest.edit(func(editor *mapedit.Editor) { editor.Undo() })
	session.waitSame(t, 4)
	host := session.sides[0].editor.Map
	if tile := host.Tile(pos); tile == nil || tile.Ground.ID != 105 {
		t.Fatalf("the undo of the guest removed the host's edit: %+v", tile)
	}
	if tile := host.Tile(other); tile != nil {
		t.Fatalf("the undo of the guest did not reach the host: %+v", tile)
	}
}

func TestCodecRoundTrip(t *testing.T) {
	dest := otmap.Position{X: 9, Y: 8, Z: 7}
	edit := &Edit{Seq: 1 << 40, Author: 3, Local: 77, Label: "Draw", Tiles: []mapedit.TileState{
		{Pos: otmap.Position{X: 1, Y: 2, Z: 3}, Tile: &otmap.Tile{
			Ground: otmap.Item{ID: 100}, Flags: otmap.FlagPZ, HouseID: 4,
			Items: []otmap.Item{{ID: 120, Ext: &otmap.ItemExt{
				ActionID: 1000, Text: "hello", Dest: &dest, DoorID: 2,
				Items: []otmap.Item{{ID: 121, Subtype: 5}},
				Attrs: []otmap.Attr{{Key: "k", Type: otmap.AttrFloat, Num: 1.5}},
				Raw:   []otmap.RawAttr{{Code: 9, Data: []byte{1, 2}}},
			}}},
			Ext: &otmap.TileExt{Creatures: []otmap.Creature{{Name: "Rat", NPC: true, SpawnTime: -1, Direction: 2, Weight: 3}}, SpawnNPC: &otmap.Spawn{Radius: 2}, Zones: []uint16{4}},
		}},
		{Pos: otmap.Position{X: 5, Y: 6, Z: 7}},
	}, Meta: &mapedit.MetaData{Width: 10, Description: "d", Towns: []otmap.Town{{ID: 1, Name: "T"}}, Houses: []otmap.House{{ID: 2, Beds: 3}}, Zones: []otmap.Zone{{ID: 1, Name: "z"}}}}
	decoded, err := decodeEdit(encodeEdit(edit))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Seq != edit.Seq || decoded.Local != 77 || decoded.Label != "Draw" || len(decoded.Tiles) != 2 {
		t.Fatalf("header: %+v", decoded)
	}
	if !mapedit.SameTile(decoded.Tiles[0].Tile, edit.Tiles[0].Tile) || decoded.Tiles[1].Tile != nil {
		t.Fatalf("tiles: %+v", decoded.Tiles)
	}
	if decoded.Meta == nil || decoded.Meta.Houses[0].Beds != 3 || decoded.Meta.Zones[0].Name != "z" {
		t.Fatalf("meta: %+v", decoded.Meta)
	}
	// Broken data fails without panicking.
	data := encodeEdit(edit)
	for cut := range len(data) {
		_, _ = decodeEdit(data[:cut])
	}
	huge := append([]byte(nil), data...)
	huge[28], huge[29], huge[30], huge[31] = 0xFF, 0xFF, 0xFF, 0x7F
	if _, err := decodeEdit(huge); err == nil {
		t.Fatal("a huge count decoded")
	}
}

func TestInviteRoundTrip(t *testing.T) {
	invite := Invite{Addresses: []string{"203.0.113.5:47816", "[2001:db8::1]:5000", "myhost.example.org:1234"}, Fingerprint: "00112233445566778899aabbccddeeff", Password: "pa ss"}
	parsed, err := ParseInvite(invite.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(parsed) != fmt.Sprint(invite) {
		t.Fatalf("got %+v, want %+v", parsed, invite)
	}
	typed, err := ParseInvite(" 192.168.1.5 ")
	if err != nil || typed.Addresses[0] != "192.168.1.5:47816" || typed.Fingerprint != "" {
		t.Fatalf("typed address: %+v %v", typed, err)
	}
	if _, err := ParseInvite("ots:!!"); err == nil {
		t.Fatal("a broken code parsed")
	}
}

func TestBrowseFindsLocalHost(t *testing.T) {
	announcer, err := Announce(func() SessionInfo { return SessionInfo{Name: "lan", Port: 1234, Fingerprint: "ab"} })
	if err != nil {
		t.Skipf("the discovery port is taken: %v", err)
	}
	defer announcer.Close()
	found, err := Browse(context.Background(), 700*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Name != "lan" || !strings.HasSuffix(found[0].Address, ":1234") {
		t.Fatalf("found %+v", found)
	}
}
