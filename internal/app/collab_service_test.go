package app

import (
	"net"
	"testing"
	"time"

	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func groundAt(t *testing.T, ms *MapService, id int, pos otmap.Position) uint16 {
	t.Helper()
	var ground uint16
	_ = ms.withMap(id, func(opened *openMap, _ *mapData) error {
		if tile := opened.editor.Map.Tile(pos); tile != nil {
			ground = tile.Ground.ID
		}
		return nil
	})
	return ground
}

func TestCollabServiceHostAndJoin(t *testing.T) {
	hostMaps, hostID := newTestMap(t)
	hostMaps.store = NewMapStore("")
	host := NewCollabService(hostMaps)
	host.offline = true
	port := freePort(t)
	status, err := host.Host(HostRequest{MapID: hostID, Session: "Test", Name: "Host", Password: "pw", Port: port})
	if err != nil {
		t.Fatal(err)
	}
	if status.Role != "host" || status.Invite == "" {
		t.Fatalf("status = %+v", status)
	}
	defer host.Leave()

	guestMaps, _ := mapTestService(t)
	guest := NewCollabService(guestMaps)
	guest.offline = true
	info, err := guest.Join(JoinRequest{Code: status.Invite, Name: "Guest"})
	if err != nil {
		t.Fatal(err)
	}
	guestID := info.ID
	if info.Name != "Test" || info.Tiles != 3 {
		t.Fatalf("joined map = %+v", info)
	}
	waitFor(t, "the guest in the host's list", func() bool { return len(host.Status().Peers) == 2 })

	// The guest draws, the host sees it; the host draws, the guest sees it.
	pos := otmap.Position{X: 20, Y: 20, Z: 7}
	mustChange(t)(guestMaps.Draw(guestID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{pos}}))
	waitFor(t, "the guest's edit on the host", func() bool { return groundAt(t, hostMaps, hostID, pos) == 100 })
	other := otmap.Position{X: 21, Y: 20, Z: 7}
	mustChange(t)(hostMaps.Draw(hostID, mapedit.DrawRequest{Kind: mapedit.KindRaw, Item: 100, Points: []otmap.Position{other}}))
	waitFor(t, "the host's edit on the guest", func() bool { return groundAt(t, guestMaps, guestID, other) == 100 })

	// Undo on the guest goes to the host too.
	mustChange(t)(guestMaps.Undo(guestID))
	waitFor(t, "the guest's undo on the host", func() bool { return groundAt(t, hostMaps, hostID, pos) == 0 })

	// A town added by the host reaches the guest.
	if _, err := hostMaps.SaveTown(hostID, otmap.Town{ID: 1, Name: "Thais", Temple: pos}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the town on the guest", func() bool {
		infoNow, _ := guestMaps.Info(guestID)
		return len(infoNow.Towns) == 1 && infoNow.Towns[0].Name == "Thais"
	})

	// Chat goes both ways.
	if err := guest.SendChat("hello"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the chat on the host", func() bool { return len(host.Status().Chat) == 1 })

	// Ending the session leaves the guest with its copy.
	host.Leave()
	waitFor(t, "the guest to notice the end", func() bool { return guest.Status().Role == "" })
	if guest.Status().Error == "" {
		t.Fatal("the guest got no reason")
	}
	if groundAt(t, guestMaps, guestID, other) != 100 {
		t.Fatal("the guest lost its copy")
	}
	// Closing a hosted map ends its session.
	status, err = host.Host(HostRequest{MapID: hostID, Port: freePort(t)})
	if err != nil {
		t.Fatal(err)
	}
	hostMaps.CloseMap(hostID)
	if host.Status().Role != "" {
		t.Fatal("closing the map did not end the session")
	}
}
