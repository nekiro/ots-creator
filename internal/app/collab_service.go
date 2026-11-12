package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nekiro/ots-creator/internal/collab"
	"github.com/nekiro/ots-creator/internal/fsutil"
	"github.com/nekiro/ots-creator/internal/mapping/mapedit"
	"github.com/nekiro/ots-creator/internal/mapping/otmap"
)

// EventCollab carries a CollabEvent: the session changed, a peer moved, a
// chat message came, or another editor changed the shared map.
const EventCollab = "collab:event"

type CollabEvent struct {
	Kind     string           `json:"kind"`
	Status   *CollabStatus    `json:"status,omitempty"`
	ID       int              `json:"id"`
	Change   *MapChange       `json:"change,omitempty"`
	Presence *collab.Presence `json:"presence,omitempty"`
	Chat     *collab.Chat     `json:"chat,omitempty"`
	Progress *Progress        `json:"progress,omitempty"`
}

// CollabSettings are remembered between sessions.
type CollabSettings struct {
	Name         string `json:"name"`
	Session      string `json:"session"`
	Port         int    `json:"port"`
	UPnP         bool   `json:"upnp"`
	Discoverable bool   `json:"discoverable"`
}

// KnownSession is a session joined before: the guest can see whether it is
// open again and join it with one click.
type KnownSession struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Map        string `json:"map"`
	Invite     string `json:"invite"`
	LastJoined int64  `json:"lastJoined"`
}

type collabStoreData struct {
	Settings CollabSettings `json:"settings"`
	Known    []KnownSession `json:"known"`
}

const maxKnownSessions = 20

// NetworkState tells the host how others can reach it.
type NetworkState struct {
	// UPnP is "off", "working", "ok" or the reason it failed.
	UPnP      string   `json:"upnp"`
	PublicIP  string   `json:"publicIp"`
	Local     []string `json:"local"`
	CarrierNA bool     `json:"carrierNat"`
	LAN       bool     `json:"lan"`
	LANError  string   `json:"lanError"`
}

type CollabStatus struct {
	// Role is "" (no session), "joining", "host" or "guest".
	Role     string             `json:"role"`
	MapID    int                `json:"mapId"`
	Session  collab.SessionInfo `json:"session"`
	You      uint32             `json:"you"`
	Peers    []collab.Peer      `json:"peers"`
	Invite   string             `json:"invite"`
	Address  string             `json:"address"`
	Network  NetworkState       `json:"network"`
	Error    string             `json:"error"`
	Chat     []collab.Chat      `json:"chat"`
	Settings CollabSettings     `json:"settings"`
}

const maxChatHistory = 200

// changeDelay gathers the edits of other editors into fewer redraws.
const changeDelay = 40 * time.Millisecond

// CollabService shares a map with other editors (see internal/collab). Lock
// order: the MapService lock before cs.mu, never the other way around.
type CollabService struct {
	maps      *MapService
	storePath string
	certPath  string

	mu        sync.Mutex
	store     collabStoreData
	role      string
	mapID     int
	host      *collab.Host
	guest     *collab.Guest
	announcer *collab.Announcer
	mapping   *collab.PortMapping
	network   NetworkState
	invite    collab.Invite
	lastError string
	chat      []collab.Chat
	cancel    context.CancelFunc
	// generation changes with every session, so late callbacks of an ended
	// one do nothing.
	generation int
	mapName    atomic.Value
	changeDue  bool
	// offline skips the router and the internet address lookups (tests).
	offline bool
}

func NewCollabService(maps *MapService) *CollabService {
	cs := &CollabService{maps: maps}
	if dir := maps.store.Dir(); dir != "" {
		cs.storePath = filepath.Join(dir, "collab.json")
		cs.certPath = filepath.Join(dir, "collab-host.pem")
		if data, err := os.ReadFile(cs.storePath); err == nil {
			_ = json.Unmarshal(data, &cs.store)
		}
	}
	settings := &cs.store.Settings
	if settings.Name == "" {
		settings.Name = defaultPeerName()
	}
	if settings.Port <= 0 || settings.Port > 0xFFFF {
		settings.Port = collab.DefaultPort
		settings.UPnP = true
		settings.Discoverable = true
	}
	cs.mapName.Store("")
	maps.closed = append(maps.closed, cs.mapClosed)
	return cs
}

func defaultPeerName() string {
	if current, err := user.Current(); err == nil {
		name := current.Username
		if _, after, found := strings.Cut(name, `\`); found {
			name = after
		}
		if name != "" {
			return name
		}
	}
	if host, err := os.Hostname(); err == nil {
		return host
	}
	return "Mapper"
}

func (cs *CollabService) saveLocked() {
	if cs.storePath != "" {
		_ = fsutil.WriteJSONPrivate(cs.storePath, cs.store, "\t")
	}
}

func (cs *CollabService) notify(event CollabEvent) {
	cs.maps.session.notify(EventCollab, event)
}

func (cs *CollabService) statusLocked() CollabStatus {
	status := CollabStatus{
		Role: cs.role, MapID: cs.mapID, Network: cs.network, Error: cs.lastError, Chat: slices.Clone(cs.chat),
		Settings: cs.store.Settings,
	}
	switch {
	case cs.host != nil:
		status.Session = cs.host.Info()
		status.You = collab.HostID
		status.Peers = cs.host.Peers()
		status.Invite = cs.invite.Encode()
	case cs.guest != nil:
		status.Session = cs.guest.Session()
		status.You = cs.guest.You()
		status.Peers = cs.guest.Peers()
		status.Address = cs.guest.Address()
	}
	return status
}

func (cs *CollabService) Status() CollabStatus {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.statusLocked()
}

func (cs *CollabService) statusChanged() {
	cs.mu.Lock()
	status := cs.statusLocked()
	cs.mu.Unlock()
	cs.notify(CollabEvent{Kind: "status", Status: &status})
}

// compat describes the item data of a map.
func (ms *MapService) compat(opened *openMap) collab.Compat { return compatOf(ms.dataFor(opened)) }

func compatOf(data *mapData) collab.Compat {
	compat := collab.Compat{ItemsMode: string(data.items.Mode), Items: data.items.Count()}
	if data.items.MajorVersion != 0 || data.items.MinorVersion != 0 {
		compat.OTB = fmt.Sprintf("%d.%d.%d", data.items.MajorVersion, data.items.MinorVersion, data.items.BuildNumber)
	}
	info := data.project.Info()
	compat.Client, compat.DatSignature, compat.SprSignature = info.Version.Name, info.Version.DatSignature, info.Version.SprSignature
	return compat
}

type HostRequest struct {
	MapID        int    `json:"mapId"`
	Session      string `json:"session"`
	Name         string `json:"name"`
	Password     string `json:"password"`
	Port         int    `json:"port"`
	UPnP         bool   `json:"upnp"`
	Discoverable bool   `json:"discoverable"`
}

var errBusy = errors.New("a map is shared already: leave that session first")

// Host shares a map: other editors can join until Leave or until the map
// closes.
func (cs *CollabService) Host(request HostRequest) (CollabStatus, error) {
	ms := cs.maps
	ms.mu.Lock()
	opened, _, err := ms.get(request.MapID)
	if err != nil {
		ms.mu.Unlock()
		return CollabStatus{}, err
	}
	compat := ms.compat(opened)
	mapName := opened.name()
	ms.mu.Unlock()

	cs.mu.Lock()
	if cs.role != "" {
		cs.mu.Unlock()
		return CollabStatus{}, errBusy
	}
	cs.store.Settings = CollabSettings{
		Name: cleanSetting(request.Name, defaultPeerName()), Session: strings.TrimSpace(request.Session), Port: request.Port,
		UPnP: request.UPnP, Discoverable: request.Discoverable,
	}
	if cs.store.Settings.Port <= 0 || cs.store.Settings.Port > 0xFFFF {
		cs.store.Settings.Port = collab.DefaultPort
	}
	cs.saveLocked()
	settings := cs.store.Settings
	cs.mu.Unlock()

	cert, fingerprint, err := collab.LoadCertificate(cs.certPath)
	if err != nil {
		return CollabStatus{}, err
	}
	session := settings.Session
	if session == "" {
		session = settings.Name + "'s map"
	}
	cs.mapName.Store(mapName)
	id := request.MapID
	var host *collab.Host
	host, err = collab.Listen(collab.HostConfig{
		Session: session, Name: settings.Name, Password: request.Password, Port: settings.Port,
		Certificate: cert, Fingerprint: fingerprint, Compat: compat,
		Map: func() string { return cs.mapName.Load().(string) },
	}, collab.HostHooks{
		Snapshot: func(attach func()) ([]byte, map[string][]byte, error) {
			ms.mu.RLock()
			defer ms.mu.RUnlock()
			opened := ms.maps[id]
			if opened == nil || !opened.editor.Shared() {
				return nil, nil, errors.New("the map is not shared now")
			}
			attach()
			return otmap.EncodeOTMAP(opened.editor.Map, ms.dataFor(opened).items)
		},
		Edit: func(edit *collab.Edit) error {
			ms.mu.Lock()
			defer ms.mu.Unlock()
			opened := ms.maps[id]
			if opened == nil || !opened.editor.Shared() {
				return ErrNoMap
			}
			opened.editor.ApplyRemote(edit.Tiles, edit.Meta)
			host.Publish(edit)
			cs.scheduleChange(id)
			return nil
		},
		Peers:    cs.statusChanged,
		Presence: func(presence collab.Presence) { cs.notify(CollabEvent{Kind: "presence", Presence: &presence}) },
		Chat:     cs.addChat,
	})
	if err != nil {
		return CollabStatus{}, err
	}

	ms.mu.Lock()
	opened = ms.maps[id]
	if opened == nil || opened.editor.Shared() {
		ms.mu.Unlock()
		host.Close()
		if opened == nil {
			return CollabStatus{}, ErrNoMap
		}
		return CollabStatus{}, errBusy
	}
	opened.editor.Observe(func(commit mapedit.Commit) {
		host.Publish(&collab.Edit{Label: commit.Label, Tiles: commit.Tiles, Meta: commit.Meta})
	})
	cs.mu.Lock()
	cs.generation++
	generation := cs.generation
	cs.role, cs.mapID, cs.host, cs.lastError, cs.chat = "host", id, host, "", nil
	cs.network = NetworkState{UPnP: "off", Local: addressList(collab.LocalAddresses(), host.Port())}
	if settings.UPnP {
		cs.network.UPnP = "working"
	}
	if settings.Discoverable {
		announcer, err := collab.Announce(host.Info)
		if err != nil {
			cs.network.LANError = "another session on this computer answers on the local network"
		} else {
			cs.announcer = announcer
			cs.network.LAN = true
		}
	}
	cs.invite = collab.Invite{Addresses: cs.network.Local, Fingerprint: fingerprint, Password: request.Password}
	ctx, cancel := context.WithCancel(context.Background())
	cs.cancel = cancel
	status := cs.statusLocked()
	cs.mu.Unlock()
	ms.mu.Unlock()

	if !cs.offline {
		go cs.findPublicAddress(ctx, generation, settings.UPnP, host.Port(), fingerprint, request.Password)
	}
	cs.notify(CollabEvent{Kind: "status", Status: &status})
	return status, nil
}

func cleanSetting(value, fallback string) string {
	if value = strings.TrimSpace(value); value == "" {
		return fallback
	}
	return value
}

func addressList(addresses []netip.Addr, port int) []string {
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, net.JoinHostPort(address.String(), strconv.Itoa(port)))
	}
	return out
}

// findPublicAddress forwards the port on the router (UPnP) and looks up the
// internet address, then puts it first in the invite.
func (cs *CollabService) findPublicAddress(ctx context.Context, generation int, upnp bool, port int, fingerprint, password string) {
	var mapping *collab.PortMapping
	var upnpState, external string
	if upnp {
		var err error
		mapping, err = collab.MapPort(ctx, port)
		if err != nil {
			upnpState = err.Error()
		} else {
			upnpState, external = "ok", mapping.External()
		}
	}
	stunCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	public, stunErr := collab.PublicAddress(stunCtx)
	cancel()

	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.generation != generation || cs.role != "host" {
		if mapping != nil {
			go mapping.Close()
		}
		return
	}
	cs.mapping = mapping
	if upnp {
		cs.network.UPnP = upnpState
	}
	var publicIP netip.Addr
	if stunErr == nil {
		publicIP = public
	} else if ip, err := netip.ParseAddr(external); err == nil && !collab.IsPrivate(ip) {
		publicIP = ip
	}
	if ip, err := netip.ParseAddr(external); err == nil && collab.IsPrivate(ip) {
		// The router itself is behind the provider's NAT: forwarding on
		// it cannot help.
		cs.network.CarrierNA = true
	}
	if publicIP.IsValid() {
		cs.network.PublicIP = publicIP.String()
		addresses := append([]string{net.JoinHostPort(publicIP.String(), strconv.Itoa(port))}, cs.network.Local...)
		cs.invite = collab.Invite{Addresses: addresses, Fingerprint: fingerprint, Password: password}
	}
	status := cs.statusLocked()
	go cs.notify(CollabEvent{Kind: "status", Status: &status})
}

type JoinRequest struct {
	// Code is an invite code or an address.
	Code     string `json:"code"`
	Password string `json:"password"`
	Name     string `json:"name"`
	// Fingerprint pins the host of an address (a session found on the
	// network); invite codes carry their own.
	Fingerprint string `json:"fingerprint"`
}

// Join connects to a host and opens its map as a new tab.
func (cs *CollabService) Join(request JoinRequest) (MapInfo, error) {
	invite, err := collab.ParseInvite(request.Code)
	if err != nil {
		return MapInfo{}, err
	}
	if request.Password != "" {
		invite.Password = request.Password
	}
	if invite.Fingerprint == "" {
		invite.Fingerprint = request.Fingerprint
	}
	ms := cs.maps
	ms.mu.Lock()
	data, err := ms.ensure("")
	if err != nil {
		ms.mu.Unlock()
		return MapInfo{}, err
	}
	compat := compatOf(data)
	ms.mu.Unlock()

	cs.mu.Lock()
	if cs.role != "" {
		cs.mu.Unlock()
		return MapInfo{}, errBusy
	}
	name := cleanSetting(request.Name, cs.store.Settings.Name)
	if name != cs.store.Settings.Name {
		cs.store.Settings.Name = name
		cs.saveLocked()
	}
	cs.generation++
	generation := cs.generation
	ctx, cancel := context.WithCancel(context.Background())
	cs.role, cs.cancel, cs.lastError, cs.chat = "joining", cancel, "", nil
	cs.mu.Unlock()
	cs.statusChanged()

	fail := func(err error) (MapInfo, error) {
		cs.mu.Lock()
		if cs.generation == generation {
			cs.role, cs.cancel = "", nil
		}
		cs.mu.Unlock()
		cancel()
		cs.statusChanged()
		if errors.Is(err, context.Canceled) {
			return MapInfo{}, errors.New("canceled")
		}
		return MapInfo{}, err
	}
	report := ms.mapProgress()
	report("Connecting to the host", 0)
	guest, snapshot, err := collab.Join(ctx, invite, name, compat, func(done, total int) {
		report("Receiving the map", 600*done/max(total, 1))
	})
	if err != nil {
		return fail(err)
	}
	report("Reading the map", 600)
	gameMap, _, err := otmap.DecodeOTMAP(snapshot.World, snapshot.Sectors, data.items, func(stage otmap.Stage, done, total int) {
		if stage == otmap.StageTiles && total > 0 {
			report("Reading the map", 600+350*done/total)
		}
	})
	snapshot = nil
	if err != nil {
		guest.Leave()
		return fail(err)
	}
	report("Preparing the editor", progressPreparing)

	ms.mu.Lock()
	cs.mu.Lock()
	if cs.generation != generation {
		cs.mu.Unlock()
		ms.mu.Unlock()
		guest.Leave()
		return MapInfo{}, errors.New("canceled")
	}
	opened := ms.add(gameMap, "", data)
	session := guest.Session()
	opened.title = session.Name
	opened.editor.MarkSaved()
	opened.editor.Observe(guest.Send)
	id := opened.id
	cs.role, cs.mapID, cs.guest = "guest", id, guest
	cs.rememberLocked(session, collab.Invite{Addresses: []string{guest.Address()}, Fingerprint: guest.Fingerprint(), Password: invite.Password}, invite)
	info := ms.info(opened, true)
	cs.mu.Unlock()
	ms.mu.Unlock()

	guest.Start(collab.GuestHooks{
		Edit: func(edit *collab.Edit) {
			ms.mu.Lock()
			defer ms.mu.Unlock()
			opened := ms.maps[id]
			if opened == nil {
				return
			}
			tiles, meta := guest.Accept(edit)
			opened.editor.ApplyRemote(tiles, meta)
			if len(tiles) > 0 || meta != nil {
				cs.scheduleChange(id)
			}
		},
		Presence: func(presence collab.Presence) { cs.notify(CollabEvent{Kind: "presence", Presence: &presence}) },
		Peers:    func([]collab.Peer) { cs.statusChanged() },
		Chat:     cs.addChat,
		Closed:   func(err error) { cs.guestClosed(generation, id, err) },
	})
	cs.statusChanged()
	return info, nil
}

// rememberLocked keeps a joined session in the list of known sessions. The
// invite keeps the address that answered first and the original ones.
func (cs *CollabService) rememberLocked(session collab.SessionInfo, used, original collab.Invite) {
	addresses := slices.Clone(used.Addresses)
	for _, address := range original.Addresses {
		if !slices.Contains(addresses, address) {
			addresses = append(addresses, address)
		}
	}
	invite := collab.Invite{Addresses: addresses, Fingerprint: used.Fingerprint, Password: used.Password}
	key := used.Fingerprint
	if key == "" {
		key = strings.Join(addresses, ",")
	}
	entry := KnownSession{Key: key, Name: session.Name, Host: session.Host, Map: session.Map, Invite: invite.Encode(), LastJoined: time.Now().UnixMilli()}
	known := slices.DeleteFunc(cs.store.Known, func(other KnownSession) bool { return other.Key == key })
	cs.store.Known = append([]KnownSession{entry}, known...)
	if len(cs.store.Known) > maxKnownSessions {
		cs.store.Known = cs.store.Known[:maxKnownSessions]
	}
	cs.saveLocked()
}

func (cs *CollabService) guestClosed(generation, id int, err error) {
	ms := cs.maps
	ms.mu.Lock()
	if opened := ms.maps[id]; opened != nil {
		opened.editor.Observe(nil)
	}
	ms.mu.Unlock()
	cs.mu.Lock()
	if cs.generation != generation {
		cs.mu.Unlock()
		return
	}
	cs.role, cs.guest, cs.mapID = "", nil, 0
	if err != nil {
		cs.lastError = "Disconnected from the host: " + friendlyError(err) + ". The map stays open as your own copy."
	}
	cs.mu.Unlock()
	cs.statusChanged()
}

func friendlyError(err error) string {
	var rejected *collab.RejectedError
	if errors.As(err, &rejected) {
		return rejected.Reason
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "the host stopped answering"
	}
	if errors.Is(err, net.ErrClosed) {
		return "the connection was closed"
	}
	return err.Error()
}

// CancelJoin stops a join in progress.
func (cs *CollabService) CancelJoin() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.role == "joining" && cs.cancel != nil {
		cs.cancel()
		cs.generation++
		cs.role, cs.cancel = "", nil
	}
}

// Leave ends the session: a host ends it for everyone, a guest keeps the
// map as its own copy.
func (cs *CollabService) Leave() {
	ms := cs.maps
	ms.mu.Lock()
	cs.mu.Lock()
	if id := cs.mapID; id != 0 {
		if opened := ms.maps[id]; opened != nil {
			opened.editor.Observe(nil)
		}
	}
	cs.stopLocked()
	cs.mu.Unlock()
	ms.mu.Unlock()
	cs.statusChanged()
}

// stopLocked ends the session; the caller holds the map lock (or the map
// is gone).
func (cs *CollabService) stopLocked() {
	cs.generation++
	if cs.cancel != nil {
		cs.cancel()
	}
	if cs.host != nil {
		cs.host.Close()
	}
	if cs.guest != nil {
		cs.guest.Leave()
	}
	if cs.announcer != nil {
		go cs.announcer.Close()
	}
	if cs.mapping != nil {
		go cs.mapping.Close()
	}
	cs.role, cs.mapID, cs.host, cs.guest, cs.announcer, cs.mapping, cs.cancel = "", 0, nil, nil, nil, nil, nil
	cs.network = NetworkState{}
	cs.invite = collab.Invite{}
}

// mapClosed ends the session of a map that closed (under the map lock).
func (cs *CollabService) mapClosed(id int) {
	cs.mu.Lock()
	if cs.mapID != id || cs.role == "" {
		cs.mu.Unlock()
		return
	}
	cs.stopLocked()
	cs.mu.Unlock()
	go cs.statusChanged()
}

// ServiceShutdown ends the session when the app quits.
func (cs *CollabService) ServiceShutdown() error {
	cs.mu.Lock()
	cs.stopLocked()
	cs.mu.Unlock()
	return nil
}

// scheduleChange tells the editor about the edits of others after a short
// delay (called under the map lock).
func (cs *CollabService) scheduleChange(id int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.changeDue {
		return
	}
	cs.changeDue = true
	time.AfterFunc(changeDelay, func() { cs.flushChange(id) })
}

func (cs *CollabService) flushChange(id int) {
	ms := cs.maps
	ms.mu.Lock()
	cs.mu.Lock()
	cs.changeDue = false
	cs.mu.Unlock()
	opened := ms.maps[id]
	if opened == nil {
		ms.mu.Unlock()
		return
	}
	change := ms.change(opened)
	ms.mu.Unlock()
	cs.notify(CollabEvent{Kind: "change", ID: id, Change: &change})
	if len(change.Chunks) > 0 {
		ms.session.notify(EventMapChanged, MapChanged{ID: id, Chunks: change.Chunks})
	}
}

func (cs *CollabService) addChat(chat collab.Chat) {
	cs.mu.Lock()
	cs.chat = append(cs.chat, chat)
	if len(cs.chat) > maxChatHistory {
		cs.chat = slices.Delete(cs.chat, 0, len(cs.chat)-maxChatHistory)
	}
	cs.mu.Unlock()
	cs.notify(CollabEvent{Kind: "chat", Chat: &chat})
}

// SendChat sends a chat message to the session.
func (cs *CollabService) SendChat(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	cs.mu.Lock()
	host, guest := cs.host, cs.guest
	cs.mu.Unlock()
	switch {
	case host != nil:
		cs.addChat(host.SendChat(text))
	case guest != nil:
		guest.SendChat(text)
	default:
		return errors.New("no session")
	}
	return nil
}

// SetPresence shares where this editor points on the shared map (pos nil:
// nowhere) and what it draws with.
func (cs *CollabService) SetPresence(mapID int, pos *otmap.Position, brush string) {
	cs.mu.Lock()
	host, guest := cs.host, cs.guest
	if mapID != cs.mapID {
		pos = nil
	}
	cs.mu.Unlock()
	switch {
	case host != nil:
		host.SetPresence(pos, brush)
	case guest != nil:
		guest.SetPresence(pos, brush)
	}
}

// Kick removes a guest from the hosted session.
func (cs *CollabService) Kick(peer uint32) {
	cs.mu.Lock()
	host := cs.host
	cs.mu.Unlock()
	if host != nil {
		host.Kick(peer)
	}
}

// Browse looks for sessions on the local network.
func (cs *CollabService) Browse() ([]collab.Found, error) {
	return collab.Browse(context.Background(), 1500*time.Millisecond)
}

func (cs *CollabService) KnownSessions() []KnownSession {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return slices.Clone(cs.store.Known)
}

func (cs *CollabService) ForgetSession(key string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.store.Known = slices.DeleteFunc(cs.store.Known, func(known KnownSession) bool { return known.Key == key })
	cs.saveLocked()
}

// Probe asks the host of an invite code or address about its session
// (whether it is open now).
func (cs *CollabService) Probe(code string) (collab.SessionInfo, error) {
	invite, err := collab.ParseInvite(code)
	if err != nil {
		return collab.SessionInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return collab.Probe(ctx, invite)
}
