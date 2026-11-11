package collab

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"
)

// Guests find sessions on the local network (and on VPNs that pass
// broadcasts, like ZeroTier or Radmin VPN) by a UDP broadcast to the
// discovery port; hosts answer with their SessionInfo.

const DiscoveryPort = 47817

const discoveryQuery = "OTS-CREATOR-DISCOVER 1"

// Announcer answers discovery queries for a host.
type Announcer struct {
	conn *net.UDPConn
	info func() SessionInfo
	wg   sync.WaitGroup
}

// Announce answers discovery queries with info until Close. It fails when
// another host on this computer answers already.
func Announce(info func() SessionInfo) (*Announcer, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: DiscoveryPort})
	if err != nil {
		return nil, err
	}
	announcer := &Announcer{conn: conn, info: info}
	announcer.wg.Add(1)
	go announcer.serve()
	return announcer, nil
}

func (a *Announcer) serve() {
	defer a.wg.Done()
	buf := make([]byte, 512)
	for {
		count, from, err := a.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		if string(buf[:count]) != discoveryQuery {
			continue
		}
		data, err := json.Marshal(a.info())
		if err == nil {
			_, _ = a.conn.WriteToUDP(data, from)
		}
	}
}

func (a *Announcer) Close() {
	_ = a.conn.Close()
	a.wg.Wait()
}

// Found is a session found on the network.
type Found struct {
	SessionInfo
	Address string `json:"address"`
}

// Browse looks for sessions on the local networks for the given time.
func Browse(ctx context.Context, wait time.Duration) ([]Found, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	targets := []netip.Addr{netip.AddrFrom4([4]byte{255, 255, 255, 255}), netip.AddrFrom4([4]byte{127, 0, 0, 1})}
	for _, network := range localNetworks() {
		targets = append(targets, broadcastOf(network))
	}
	send := func() {
		for _, target := range targets {
			_, _ = conn.WriteToUDPAddrPort([]byte(discoveryQuery), netip.AddrPortFrom(target, DiscoveryPort))
		}
	}
	send()
	deadline := time.Now().Add(wait)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	resend := time.Now().Add(wait / 3)
	seen := map[string]Found{}
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		// One more query a moment later: a lost broadcast is not answered.
		next := deadline
		if !resend.IsZero() {
			if time.Now().After(resend) {
				send()
				resend = time.Time{}
			} else if resend.Before(deadline) {
				next = resend
			}
		}
		_ = conn.SetReadDeadline(next)
		count, from, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			continue
		}
		var info SessionInfo
		if json.Unmarshal(buf[:count], &info) != nil || info.Port <= 0 || info.Port > 0xFFFF {
			continue
		}
		address := net.JoinHostPort(from.Addr().Unmap().String(), strconv.Itoa(info.Port))
		key := info.Fingerprint + "|" + strconv.Itoa(info.Port)
		// A host found on many interfaces is listed once, preferably by a
		// network address over the loopback.
		if old, ok := seen[key]; ok && !isLoopback(old.Address) {
			continue
		}
		seen[key] = Found{SessionInfo: info, Address: address}
	}
	found := make([]Found, 0, len(seen))
	for _, entry := range seen {
		found = append(found, entry)
	}
	slices.SortFunc(found, func(a, b Found) int {
		if a.Name != b.Name {
			if a.Name < b.Name {
				return -1
			}
			return 1
		}
		if a.Address < b.Address {
			return -1
		}
		return 1
	})
	return found, nil
}

func isLoopback(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// localNetworks lists the IPv4 networks of the interfaces that are up.
func localNetworks() []netip.Prefix {
	var networks []netip.Prefix
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil || !prefix.Addr().Unmap().Is4() {
				continue
			}
			networks = append(networks, netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()))
		}
	}
	return networks
}

func broadcastOf(network netip.Prefix) netip.Addr {
	ip := network.Addr().As4()
	bits := network.Bits()
	for i := range 4 {
		for bit := range 8 {
			if i*8+bit >= bits {
				ip[i] |= 0x80 >> bit
			}
		}
	}
	return netip.AddrFrom4(ip)
}

// LocalAddresses lists the addresses of this computer other editors may
// reach: IPv4 addresses of the local networks and VPNs first, then global
// IPv6 addresses.
func LocalAddresses() []netip.Addr {
	var v4, v6 []netip.Addr
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				continue
			}
			ip := prefix.Addr().Unmap()
			switch {
			case ip.Is4() && !ip.IsLinkLocalUnicast():
				v4 = append(v4, ip)
			case ip.Is6() && ip.IsGlobalUnicast() && !ip.IsPrivate():
				v6 = append(v6, ip)
			}
		}
	}
	return append(v4, v6...)
}

// IsPrivate reports whether ip is not reachable from the internet: private,
// shared (carrier-grade NAT), loopback or link-local.
func IsPrivate(ip netip.Addr) bool {
	ip = ip.Unmap()
	carrierNAT := netip.MustParsePrefix("100.64.0.0/10")
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || carrierNAT.Contains(ip) || ip.IsUnspecified()
}
