package collab

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

// UPnP IGD: the host asks the router to forward its port, so guests from
// the internet can connect without the user setting up port forwarding.
// Most home routers support it; routers behind another NAT (carrier-grade
// NAT of the provider) cannot be reached this way at all.

const (
	ssdpAddress  = "239.255.255.250:1900"
	upnpLease    = time.Hour
	upnpTimeout  = 3 * time.Second
	maxUPnPReply = 1 << 20
)

// PortMapping is a port forwarded by the router.
type PortMapping struct {
	controlURL string
	service    string
	port       int
	external   string
	stop       chan struct{}
	once       sync.Once
	permanent  bool
}

var ErrNoGateway = errors.New("no router answered (UPnP may be off on the router)")

// MapPort asks the router to forward the TCP port to this computer.
func MapPort(ctx context.Context, port int) (*PortMapping, error) {
	locations, err := discoverGateways(ctx)
	if err != nil {
		return nil, err
	}
	var lastErr error = ErrNoGateway
	for _, location := range locations {
		controlURL, service, err := gatewayService(ctx, location)
		if err != nil {
			lastErr = err
			continue
		}
		internal, err := localAddressFor(controlURL)
		if err != nil {
			lastErr = err
			continue
		}
		mapping := &PortMapping{controlURL: controlURL, service: service, port: port, stop: make(chan struct{})}
		if err := mapping.add(ctx, internal, upnpLease); err != nil {
			// Some routers only take permanent mappings.
			if err2 := mapping.add(ctx, internal, 0); err2 != nil {
				lastErr = err
				continue
			}
			mapping.permanent = true
		}
		mapping.external, _ = mapping.externalAddress(ctx)
		if !mapping.permanent {
			go mapping.refresh(internal)
		}
		return mapping, nil
	}
	return nil, lastErr
}

// External is the internet address of the router ("" when it did not say).
func (m *PortMapping) External() string { return m.external }

// Close removes the forwarding.
func (m *PortMapping) Close() {
	m.once.Do(func() {
		close(m.stop)
		ctx, cancel := context.WithTimeout(context.Background(), upnpTimeout)
		defer cancel()
		_, _ = m.call(ctx, "DeletePortMapping", fmt.Sprintf(
			"<NewRemoteHost></NewRemoteHost><NewExternalPort>%d</NewExternalPort><NewProtocol>TCP</NewProtocol>", m.port))
	})
}

func (m *PortMapping) refresh(internal string) {
	ticker := time.NewTicker(upnpLease / 2)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), upnpTimeout)
			_ = m.add(ctx, internal, upnpLease)
			cancel()
		}
	}
}

func (m *PortMapping) add(ctx context.Context, internal string, lease time.Duration) error {
	_, err := m.call(ctx, "AddPortMapping", fmt.Sprintf(
		"<NewRemoteHost></NewRemoteHost><NewExternalPort>%d</NewExternalPort><NewProtocol>TCP</NewProtocol>"+
			"<NewInternalPort>%d</NewInternalPort><NewInternalClient>%s</NewInternalClient><NewEnabled>1</NewEnabled>"+
			"<NewPortMappingDescription>OTS Creator map session</NewPortMappingDescription><NewLeaseDuration>%d</NewLeaseDuration>",
		m.port, m.port, html.EscapeString(internal), int(lease.Seconds())))
	return err
}

func (m *PortMapping) externalAddress(ctx context.Context) (string, error) {
	reply, err := m.call(ctx, "GetExternalIPAddress", "")
	if err != nil {
		return "", err
	}
	text := tagText(reply, "NewExternalIPAddress")
	if _, err := netip.ParseAddr(text); err != nil {
		return "", errors.New("the router did not tell its address")
	}
	return text, nil
}

func (m *PortMapping) call(ctx context.Context, action, arguments string) ([]byte, error) {
	body := `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body><u:` + action + ` xmlns:u="` + m.service + `">` + arguments + `</u:` + action + `></s:Body></s:Envelope>`
	ctx, cancel := context.WithTimeout(ctx, upnpTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.controlURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	request.Header.Set("SOAPAction", `"`+m.service+"#"+action+`"`)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	reply, err := io.ReadAll(io.LimitReader(response.Body, maxUPnPReply))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		if description := tagText(reply, "errorDescription"); description != "" {
			return nil, fmt.Errorf("the router refused: %s", description)
		}
		return nil, fmt.Errorf("the router refused (%s)", response.Status)
	}
	return reply, nil
}

// tagText returns the text of the first element named name in an XML
// document, whatever its namespace.
func tagText(document []byte, name string) string {
	decoder := xml.NewDecoder(bytes.NewReader(document))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == name {
			var text string
			if decoder.DecodeElement(&text, &start) == nil {
				return strings.TrimSpace(text)
			}
			return ""
		}
	}
}

// discoverGateways sends an SSDP search and returns the description URLs
// of the routers that answer.
func discoverGateways(ctx context.Context) ([]string, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	target, err := net.ResolveUDPAddr("udp4", ssdpAddress)
	if err != nil {
		return nil, err
	}
	for _, searchTarget := range []string{"urn:schemas-upnp-org:device:InternetGatewayDevice:1", "urn:schemas-upnp-org:device:InternetGatewayDevice:2", "urn:schemas-upnp-org:service:WANIPConnection:1"} {
		query := "M-SEARCH * HTTP/1.1\r\nHOST: " + ssdpAddress + "\r\nST: " + searchTarget + "\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\n\r\n"
		_, _ = conn.WriteToUDP([]byte(query), target)
	}
	deadline := time.Now().Add(upnpTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetReadDeadline(deadline)
	var locations []string
	seen := map[string]bool{}
	buf := make([]byte, 4096)
	for {
		count, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		for line := range strings.SplitSeq(string(buf[:count]), "\r\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "location") {
				continue
			}
			location := strings.TrimSpace(value)
			if parsed, err := url.Parse(location); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && !seen[location] {
				seen[location] = true
				locations = append(locations, location)
			}
		}
		if len(locations) > 0 && time.Until(deadline) > 500*time.Millisecond {
			// Answers come together: wait a little for others.
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		}
	}
	if len(locations) == 0 {
		return nil, ErrNoGateway
	}
	return locations, nil
}

type upnpDevice struct {
	Services []struct {
		Type       string `xml:"serviceType"`
		ControlURL string `xml:"controlURL"`
	} `xml:"serviceList>service"`
	Devices []upnpDevice `xml:"deviceList>device"`
}

// gatewayService reads a router description and returns the control URL
// of its WAN connection service.
func gatewayService(ctx context.Context, location string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, upnpTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return "", "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	var root struct {
		URLBase string     `xml:"URLBase"`
		Device  upnpDevice `xml:"device"`
	}
	if err := xml.NewDecoder(io.LimitReader(response.Body, maxUPnPReply)).Decode(&root); err != nil {
		return "", "", err
	}
	base, err := url.Parse(location)
	if err != nil {
		return "", "", err
	}
	if root.URLBase != "" {
		if parsed, err := url.Parse(root.URLBase); err == nil {
			base = parsed
		}
	}
	var find func(device upnpDevice) (string, string)
	find = func(device upnpDevice) (string, string) {
		for _, service := range device.Services {
			if strings.Contains(service.Type, ":WANIPConnection:") || strings.Contains(service.Type, ":WANPPPConnection:") {
				return service.ControlURL, service.Type
			}
		}
		for _, inner := range device.Devices {
			if control, kind := find(inner); control != "" {
				return control, kind
			}
		}
		return "", ""
	}
	control, service := find(root.Device)
	if control == "" {
		return "", "", errors.New("the router does not forward ports")
	}
	resolved, err := base.Parse(control)
	if err != nil {
		return "", "", err
	}
	return resolved.String(), service, nil
}

// localAddressFor returns the address of this computer on the network of
// the router at controlURL.
func localAddressFor(controlURL string) (string, error) {
	parsed, err := url.Parse(controlURL)
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "80"
	}
	conn, err := net.DialTimeout("udp4", net.JoinHostPort(host, port), upnpTimeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	address := conn.LocalAddr().(*net.UDPAddr)
	return address.IP.String(), nil
}
