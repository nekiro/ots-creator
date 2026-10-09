package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/market"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/settings"
	"github.com/nekiro/ots-creator/internal/thing"
)

// Market endpoints. MarketFilesURL serves the public files (index.json and
// entries), MarketAPIURL is the market API that writes them. The
// environment variables OTS_MARKET_URL and OTS_MARKET_API override them,
// for example to test against a local server.
var (
	MarketFilesURL = "https://otscreatorbucket.nekiro.dev"
	MarketAPIURL   = "https://otscreatormarket.nekiro.dev"
)

// marketIndexTTL is how long a downloaded index is reused.
const marketIndexTTL = 5 * time.Minute

// httpClient bounds every market request; callers add their own deadline.
var httpClient = &http.Client{Timeout: 2 * time.Minute}

// MarketService browses the market, imports entries and shares objects
// and sprites.
type MarketService struct {
	s   *Session
	st  *settings.Store
	src *market.Source // nil when not configured
	api *market.API    // nil when not configured

	mu      sync.Mutex
	index   *market.Index
	fetched time.Time
}

// NewMarketService returns the service for the configured market.
func NewMarketService(s *Session, st *settings.Store) *MarketService {
	m := &MarketService{s: s, st: st}
	if u := cmpOr(os.Getenv("OTS_MARKET_URL"), MarketFilesURL); u != "" {
		m.src = market.NewSource(u)
		m.src.HTTP = httpClient
	}
	if u := cmpOr(os.Getenv("OTS_MARKET_API"), MarketAPIURL); u != "" {
		m.api = market.NewAPI(u)
		m.api.HTTP = httpClient
	}
	return m
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var errNoMarket = errors.New("this build has no market configured")

// MarketConfig tells the UI what the market can do.
type MarketConfig struct {
	// Browse and Share are false when the build has no market URLs.
	Browse bool `json:"browse"`
	Share  bool `json:"share"`
	// CaptchaURL is the captcha page for the share window and
	// CaptchaOrigin the origin its messages come from.
	CaptchaURL    string `json:"captchaUrl"`
	CaptchaOrigin string `json:"captchaOrigin"`
	// Author is the nickname last used to share.
	Author string `json:"author"`
}

// Config returns the market configuration.
func (m *MarketService) Config() MarketConfig {
	c := MarketConfig{Browse: m.src != nil, Share: m.api != nil && m.src != nil, Author: m.st.Get().MarketAuthor}
	if m.api != nil {
		c.CaptchaURL = m.api.CaptchaURL()
		if u, err := url.Parse(c.CaptchaURL); err == nil {
			c.CaptchaOrigin = u.Scheme + "://" + u.Host
		}
	}
	return c
}

// MarketIndex is the index for the UI. Preview and file paths are relative
// to Base. Mine lists entries shared from this computer; Admin is set when
// an admin token is configured (every entry can be deleted).
type MarketIndex struct {
	Base    string         `json:"base"`
	Entries []market.Entry `json:"entries"`
	Mine    []string       `json:"mine"`
	Admin   bool           `json:"admin"`
}

// Index returns the market entries, downloading them again when refresh is
// set or the cached copy is old.
func (m *MarketService) Index(refresh bool) (*MarketIndex, error) {
	if m.src == nil {
		return nil, errNoMarket
	}
	m.mu.Lock()
	idx := m.index
	if refresh || time.Since(m.fetched) > marketIndexTTL {
		idx = nil
	}
	m.mu.Unlock()
	if idx == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if idx, err = m.src.Index(ctx); err != nil {
			return nil, fmt.Errorf("market: %w", err)
		}
		m.mu.Lock()
		m.index, m.fetched = idx, time.Now()
		m.mu.Unlock()
	}
	prefs := m.st.Get()
	out := &MarketIndex{Base: m.src.PreviewBase(), Entries: idx.Entries, Mine: []string{}, Admin: prefs.MarketAdminToken != ""}
	for _, e := range idx.Entries {
		if prefs.MarketShared[e.ID] != "" {
			out.Mine = append(out.Mine, e.ID)
		}
	}
	return out, nil
}

func (m *MarketService) entry(id string) (*market.Entry, error) {
	idx, err := m.Index(false)
	if err != nil {
		return nil, err
	}
	for i := range idx.Entries {
		if idx.Entries[i].ID == id {
			return &idx.Entries[i], nil
		}
	}
	return nil, fmt.Errorf("market entry %s does not exist", id)
}

func (m *MarketService) download(id string) (*market.Entry, []byte, error) {
	e, err := m.entry(id)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	data, err := m.src.File(ctx, e)
	if err != nil {
		return nil, nil, fmt.Errorf("download %s: %w", e.Name, err)
	}
	return e, data, nil
}

// Read downloads an object entry for previewing. No client needs to be
// open.
func (m *MarketService) Read(id string) (*ObjectFile, error) {
	e, data, err := m.download(id)
	if err != nil {
		return nil, err
	}
	if e.Kind != market.KindObject {
		return nil, fmt.Errorf("%s is not an object", e.Name)
	}
	d, _, err := market.ReadObject(&e.Meta, data)
	if err != nil {
		return nil, err
	}
	return newObjectFile(e.File, d), nil
}

// MarketImport reports an imported entry: an object (Category and ID) or
// the new sprite ids of a sprite pack.
type MarketImport struct {
	Kind     string         `json:"kind"`
	Category thing.Category `json:"category"`
	ID       uint32         `json:"id"`
	Sprites  []uint32       `json:"sprites"`
}

// Import downloads an entry into the open client: objects are appended,
// sprite packs add their sprites. Files are checked like any other
// untrusted file.
func (m *MarketService) Import(id string) (*MarketImport, error) {
	p, err := m.s.Project()
	if err != nil {
		return nil, err
	}
	e, data, err := m.download(id)
	if err != nil {
		return nil, err
	}
	if e.SpriteSize != p.SpriteSize() {
		return nil, fmt.Errorf("%s uses %dpx sprites, client uses %dpx", e.Name, e.SpriteSize, p.SpriteSize())
	}
	res := &MarketImport{Kind: e.Kind, Sprites: []uint32{}}
	switch e.Kind {
	case market.KindObject:
		d, _, err := market.ReadObject(&e.Meta, data)
		if err != nil {
			return nil, err
		}
		res.Category = d.Thing.Category
		if res.ID, err = p.ImportOBD(d, 0); err != nil {
			return nil, err
		}
	case market.KindSprites:
		sprites, _, err := market.ReadPack(&e.Meta, data)
		if err != nil {
			return nil, err
		}
		if res.Sprites, err = p.AddSprites(sprites); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown entry kind %q", e.Kind)
	}
	m.s.Changed()
	return res, nil
}

// ShareRequest is what the author fills in. Captcha is the captcha token.
type ShareRequest struct {
	Name        string   `json:"name"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	License     string   `json:"license"`
	Captcha     string   `json:"captcha"`
}

func (r ShareRequest) meta(kind string, size int) market.Meta {
	return market.Meta{
		Name:        strings.TrimSpace(r.Name),
		Kind:        kind,
		Tags:        market.NormalizeTags(r.Tags),
		Description: strings.TrimSpace(r.Description),
		License:     r.License,
		Author:      strings.TrimSpace(r.Author),
		SpriteSize:  size,
	}
}

// ShareObject publishes one object and returns the new entry id.
func (m *MarketService) ShareObject(c thing.Category, id uint32, req ShareRequest) (string, error) {
	p, err := m.s.Project()
	if err != nil {
		return "", err
	}
	d, err := p.ExportOBD(c, id, obd.Version3)
	if err != nil {
		return "", err
	}
	return m.shareOBD(d, req)
}

// shareOBD publishes OBD data as an object entry.
func (m *MarketService) shareOBD(d *obd.Data, req ShareRequest) (string, error) {
	d.Thing.Name, d.Thing.Description = "", "" // assets names do not travel
	d.Version = obd.Version3
	data, err := obd.Encode(d)
	if err != nil {
		return "", err
	}
	meta := req.meta(market.KindObject, d.SpriteSize)
	meta.Category = d.Thing.Category.String()
	back, info, err := market.ReadObject(&meta, data)
	if err != nil {
		return "", err
	}
	preview, err := market.Preview(back)
	if err != nil {
		return "", err
	}
	if len(preview) > market.MaxPreviewBytes {
		return "", errors.New("the preview is too big; the object has too many large frames")
	}
	return m.submit(market.Submission{Meta: meta, Info: info, File: data, Preview: preview, Captcha: req.Captcha})
}

// ShareSprites publishes sprites as a pack and returns the new entry id.
// Empty sprites are left out.
func (m *MarketService) ShareSprites(ids []uint32, req ShareRequest) (string, error) {
	p, err := m.s.Project()
	if err != nil {
		return "", err
	}
	var sprites [][]byte
	for _, id := range ids {
		px, err := p.SpritePixels(id)
		if err != nil {
			return "", err
		}
		sprites = append(sprites, px)
	}
	return m.sharePixels(sprites, p.SpriteSize(), req)
}

// sharePixels publishes sprites as a pack; empty sprites are left out.
func (m *MarketService) sharePixels(all [][]byte, size int, req ShareRequest) (string, error) {
	var sprites [][]byte
	for _, px := range all {
		if !isTransparent(px) {
			sprites = append(sprites, px)
		}
	}
	img, err := market.PackImage(sprites, size)
	if err != nil {
		return "", err
	}
	data, err := imaging.EncodePNG(img)
	if err != nil {
		return "", err
	}
	meta := req.meta(market.KindSprites, size)
	_, info, err := market.ReadPack(&meta, data)
	if err != nil {
		return "", err
	}
	return m.submit(market.Submission{Meta: meta, Info: info, File: data, Captcha: req.Captcha})
}

// ShareFile publishes a file from disk: an object file (.otobj or .obd;
// the market stores objects as OBD) as an object, or an
// image cut into sprites of spriteSize px (32 or 64; magenta is
// transparent) as a sprite pack. No client needs to be open.
func (m *MarketService) ShareFile(path string, spriteSize int, req ShareRequest) (string, error) {
	if isObjectFile(path) {
		d, err := readObject(path)
		if err != nil {
			return "", err
		}
		return m.shareOBD(d, req)
	}
	if spriteSize != 32 && spriteSize != 64 {
		return "", fmt.Errorf("sprite size %d is not 32 or 64", spriteSize)
	}
	img, err := imaging.Load(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if img.Rect.Dx()%spriteSize != 0 || img.Rect.Dy()%spriteSize != 0 {
		return "", fmt.Errorf("%s is %dx%d, not a grid of %d px sprites", filepath.Base(path), img.Rect.Dx(), img.Rect.Dy(), spriteSize)
	}
	imaging.RemoveMagenta(img)
	imaging.Normalize(img)
	return m.sharePixels(imaging.SliceTiles(img, spriteSize), spriteSize, req)
}

func (m *MarketService) submit(s market.Submission) (string, error) {
	if m.api == nil || m.src == nil {
		return "", errNoMarket
	}
	if err := s.Meta.Validate(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	shared, err := m.api.Submit(ctx, s)
	if err != nil {
		return "", err
	}
	m.st.SetMarketShared(shared.ID, shared.DeleteToken)
	m.st.SetMarketAuthor(s.Meta.Author)
	m.mu.Lock()
	m.index = nil // the next read shows the new entry
	m.mu.Unlock()
	return shared.ID, nil
}

// Delete removes an entry shared from this computer, or any entry with the
// admin token.
func (m *MarketService) Delete(id string) error {
	if m.api == nil {
		return errNoMarket
	}
	prefs := m.st.Get()
	token := cmpOr(prefs.MarketShared[id], prefs.MarketAdminToken)
	if token == "" {
		return errors.New("only the author can delete this entry")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.api.Delete(ctx, id, token); err != nil {
		return err
	}
	m.st.SetMarketShared(id, "")
	m.mu.Lock()
	if m.index != nil {
		idx := *m.index
		idx.Entries = slices.DeleteFunc(slices.Clone(idx.Entries), func(e market.Entry) bool { return e.ID == id })
		m.index = &idx
	}
	m.mu.Unlock()
	return nil
}
