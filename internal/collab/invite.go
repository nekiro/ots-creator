package collab

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/nekiro/ots-creator/internal/formats/binio"
)

// Invite is what a guest needs to join a host: where to connect, the host's
// certificate fingerprint and the password.
type Invite struct {
	Addresses   []string `json:"addresses"`
	Fingerprint string   `json:"fingerprint"`
	Password    string   `json:"password"`
}

const invitePrefix = "ots:"

const inviteVersion = 1

// Encode returns the invite code: "ots:" and the invite in URL-safe base64.
func (invite Invite) Encode() string {
	w := binio.NewWriter(64)
	w.U8(inviteVersion)
	addresses := invite.Addresses[:min(len(invite.Addresses), 255)]
	w.U8(uint8(len(addresses)))
	for _, address := range addresses {
		host, portText, err := net.SplitHostPort(address)
		port, _ := strconv.Atoi(portText)
		if err != nil {
			host, port = address, DefaultPort
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			ip = ip.Unmap()
			raw := ip.AsSlice()
			w.U8(uint8(len(raw)))
			w.Write(raw)
		} else {
			name := host[:min(len(host), 200)]
			w.U8(0)
			w.U8(uint8(len(name)))
			w.Write([]byte(name))
		}
		w.U16(uint16(port))
	}
	fingerprint, _ := hex.DecodeString(invite.Fingerprint)
	if len(fingerprint) != fingerprintBytes {
		fingerprint = make([]byte, fingerprintBytes)
	}
	w.Write(fingerprint)
	password := invite.Password[:min(len(invite.Password), 255)]
	w.U8(uint8(len(password)))
	w.Write([]byte(password))
	return invitePrefix + base64.RawURLEncoding.EncodeToString(w.Bytes())
}

var errInvite = errors.New("this is not an invite code or an address")

// ParseInvite reads an invite code, or an address typed by hand ("host",
// "host:port"): that one has no fingerprint, so any host there is trusted.
func ParseInvite(text string) (Invite, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Invite{}, errInvite
	}
	if code, ok := strings.CutPrefix(text, invitePrefix); ok {
		return decodeInvite(code)
	}
	if strings.ContainsAny(text, " \t/\\") {
		return Invite{}, errInvite
	}
	if _, _, err := net.SplitHostPort(text); err != nil {
		text = net.JoinHostPort(strings.Trim(text, "[]"), strconv.Itoa(DefaultPort))
	}
	return Invite{Addresses: []string{text}}, nil
}

func decodeInvite(code string) (Invite, error) {
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(code, "="))
	if err != nil {
		return Invite{}, errInvite
	}
	r := binio.NewReader(data)
	if r.U8() != inviteVersion {
		return Invite{}, errors.New("the invite code is from another version of the editor")
	}
	var invite Invite
	count := int(r.U8())
	for range count {
		size := int(r.U8())
		var host string
		switch size {
		case 0:
			host = string(r.Bytes(int(r.U8())))
		case 4, 16:
			ip, ok := netip.AddrFromSlice(r.Bytes(size))
			if !ok {
				return Invite{}, errInvite
			}
			host = ip.String()
		default:
			return Invite{}, errInvite
		}
		port := r.U16()
		if r.Err() != nil {
			return Invite{}, errInvite
		}
		invite.Addresses = append(invite.Addresses, net.JoinHostPort(host, strconv.Itoa(int(port))))
	}
	fingerprint := r.Bytes(fingerprintBytes)
	password := r.Bytes(int(r.U8()))
	if r.Err() != nil || len(invite.Addresses) == 0 {
		return Invite{}, errInvite
	}
	if hex.EncodeToString(fingerprint) != strings.Repeat("0", 2*fingerprintBytes) {
		invite.Fingerprint = hex.EncodeToString(fingerprint)
	}
	invite.Password = string(password)
	return invite, nil
}
