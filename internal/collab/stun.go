package collab

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"time"
)

// PublicAddress asks public STUN servers which internet address this
// computer has (the router's address). It sends nothing but the request.
func PublicAddress(ctx context.Context) (netip.Addr, error) {
	servers := []string{"stun.cloudflare.com:3478", "stun.l.google.com:19302"}
	var lastErr error = errors.New("no STUN server answered")
	for _, server := range servers {
		ip, err := stunRequest(ctx, server)
		if err == nil {
			return ip, nil
		}
		lastErr = err
	}
	return netip.Addr{}, lastErr
}

const stunMagic = 0x2112A442

func stunRequest(ctx context.Context, server string) (netip.Addr, error) {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp4", server)
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	request := make([]byte, 20)
	binary.BigEndian.PutUint16(request[0:], 0x0001) // binding request
	binary.BigEndian.PutUint32(request[4:], stunMagic)
	if _, err := rand.Read(request[8:20]); err != nil {
		return netip.Addr{}, err
	}
	buf := make([]byte, 1024)
	for attempt := range 3 {
		if _, err := conn.Write(request); err != nil {
			return netip.Addr{}, err
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Duration(attempt+1) * 500 * time.Millisecond))
		count, err := conn.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return netip.Addr{}, ctx.Err()
			}
			continue
		}
		if ip, ok := parseStunReply(buf[:count], request[8:20]); ok {
			return ip, nil
		}
	}
	return netip.Addr{}, errors.New("no answer from " + server)
}

func parseStunReply(reply, transaction []byte) (netip.Addr, bool) {
	if len(reply) < 20 || binary.BigEndian.Uint16(reply[0:]) != 0x0101 || binary.BigEndian.Uint32(reply[4:]) != stunMagic ||
		string(reply[8:20]) != string(transaction) {
		return netip.Addr{}, false
	}
	length := int(binary.BigEndian.Uint16(reply[2:]))
	attrs := reply[20:min(len(reply), 20+length)]
	var mapped netip.Addr
	for len(attrs) >= 4 {
		kind := binary.BigEndian.Uint16(attrs[0:])
		size := int(binary.BigEndian.Uint16(attrs[2:]))
		if 4+size > len(attrs) {
			break
		}
		value := attrs[4 : 4+size]
		if size >= 8 && value[1] == 0x01 { // IPv4
			var ip [4]byte
			copy(ip[:], value[4:8])
			switch kind {
			case 0x0020: // XOR-MAPPED-ADDRESS
				binary.BigEndian.PutUint32(ip[:], binary.BigEndian.Uint32(ip[:])^stunMagic)
				return netip.AddrFrom4(ip), true
			case 0x0001: // MAPPED-ADDRESS
				mapped = netip.AddrFrom4(ip)
			}
		}
		attrs = attrs[4+(size+3)&^3:]
	}
	return mapped, mapped.IsValid()
}
