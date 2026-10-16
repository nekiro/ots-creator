package app

import (
	"cmp"
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

// Market timeouts.
const (
	// marketIndexTTL is how long a downloaded index is reused.
	marketIndexTTL = 5 * time.Minute
	// marketRequestTimeout bounds every market request; callers add their
	// own deadline.
	marketRequestTimeout = 2 * time.Minute
	marketIndexTimeout   = 30 * time.Second
	marketFileTimeout    = 60 * time.Second
	marketSubmitTimeout  = 2 * time.Minute
	marketDeleteTimeout  = 30 * time.Second
)

// shareSpriteSizes are the sprite sizes a shared image may be cut into.
var shareSpriteSizes = []int{32, 64}

var httpClient = &http.Client{Timeout: marketRequestTimeout}

// MarketService browses the market, imports entries and shares objects
// and sprites.
type MarketService struct {
	session  *Session
	settings *settings.Store
	src      *market.Source // nil when not configured
	api      *market.API    // nil when not configured

	mu      sync.Mutex
	index   *market.Index
	fetched time.Time
}

// NewMarketService returns the service for the configured market.
func NewMarketService(session *Session, store *settings.Store) *MarketService {
	filesURL := cmp.Or(os.Getenv("OTS_MARKET_URL"), MarketFilesURL)
	apiURL := cmp.Or(os.Getenv("OTS_MARKET_API"), MarketAPIURL)
	return newMarketService(session, store, filesURL, apiURL, httpClient)
}

// newMarketService returns the service for a market at filesURL and apiURL
// ("" for none).
func newMarketService(session *Session, store *settings.Store, filesURL, apiURL string, client *http.Client) *MarketService {
	m := &MarketService{session: session, settings: store}
	if filesURL != "" {
		m.src = market.NewSource(filesURL)
		m.src.HTTP = client
	}
	if apiURL != "" {
		m.api = market.NewAPI(apiURL)
		m.api.HTTP = client
	}
	return m
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
	config := MarketConfig{Browse: m.src != nil, Share: m.api != nil && m.src != nil, Author: m.settings.Get().MarketAuthor}
	if m.api != nil {
		config.CaptchaURL = m.api.CaptchaURL()
		if captcha, err := url.Parse(config.CaptchaURL); err == nil {
			config.CaptchaOrigin = captcha.Scheme + "://" + captcha.Host
		}
	}
	return config
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
	index := m.index
	if refresh || time.Since(m.fetched) > marketIndexTTL {
		index = nil
	}
	m.mu.Unlock()
	if index == nil {
		ctx, cancel := context.WithTimeout(context.Background(), marketIndexTimeout)
		defer cancel()
		var err error
		if index, err = m.src.Index(ctx); err != nil {
			return nil, fmt.Errorf("market: %w", err)
		}
		m.mu.Lock()
		m.index, m.fetched = index, time.Now()
		m.mu.Unlock()
	}
	prefs := m.settings.Get()
	out := &MarketIndex{Base: m.src.PreviewBase(), Entries: index.Entries, Mine: []string{}, Admin: prefs.MarketAdminToken != ""}
	for _, entry := range index.Entries {
		if prefs.MarketShared[entry.ID] != "" {
			out.Mine = append(out.Mine, entry.ID)
		}
	}
	return out, nil
}

func (m *MarketService) entry(id string) (*market.Entry, error) {
	index, err := m.Index(false)
	if err != nil {
		return nil, err
	}
	for i := range index.Entries {
		if index.Entries[i].ID == id {
			return &index.Entries[i], nil
		}
	}
	return nil, fmt.Errorf("market entry %s does not exist", id)
}

func (m *MarketService) download(id string) (*market.Entry, []byte, error) {
	entry, err := m.entry(id)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), marketFileTimeout)
	defer cancel()
	data, err := m.src.File(ctx, entry)
	if err != nil {
		return nil, nil, fmt.Errorf("download %s: %w", entry.Name, err)
	}
	return entry, data, nil
}

// Read downloads an object entry for previewing. No client needs to be
// open.
func (m *MarketService) Read(id string) (*ObjectFile, error) {
	entry, data, err := m.download(id)
	if err != nil {
		return nil, err
	}
	if entry.Kind != market.KindObject {
		return nil, fmt.Errorf("%s is not an object", entry.Name)
	}
	object, _, err := market.ReadObject(&entry.Meta, data)
	if err != nil {
		return nil, err
	}
	return newObjectFile(entry.File, object), nil
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
	p, err := m.session.Project()
	if err != nil {
		return nil, err
	}
	entry, data, err := m.download(id)
	if err != nil {
		return nil, err
	}
	if entry.SpriteSize != p.SpriteSize() {
		return nil, fmt.Errorf("%s uses %dpx sprites, client uses %dpx", entry.Name, entry.SpriteSize, p.SpriteSize())
	}
	result := &MarketImport{Kind: entry.Kind, Sprites: []uint32{}}
	switch entry.Kind {
	case market.KindObject:
		object, _, err := market.ReadObject(&entry.Meta, data)
		if err != nil {
			return nil, err
		}
		result.Category = object.Thing.Category
		if result.ID, err = p.ImportOBD(object, 0); err != nil {
			return nil, err
		}
	case market.KindSprites:
		sprites, _, err := market.ReadPack(&entry.Meta, data)
		if err != nil {
			return nil, err
		}
		if result.Sprites, err = p.AddSprites(sprites); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown entry kind %q", entry.Kind)
	}
	m.session.Changed()
	return result, nil
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

func (req ShareRequest) meta(kind string, spriteSize int) market.Meta {
	return market.Meta{
		Name:        strings.TrimSpace(req.Name),
		Kind:        kind,
		Tags:        market.NormalizeTags(req.Tags),
		Description: strings.TrimSpace(req.Description),
		License:     req.License,
		Author:      strings.TrimSpace(req.Author),
		SpriteSize:  spriteSize,
	}
}

// ShareObject publishes one object and returns the new entry id.
func (m *MarketService) ShareObject(category thing.Category, id uint32, req ShareRequest) (string, error) {
	p, err := m.session.Project()
	if err != nil {
		return "", err
	}
	object, err := p.ExportOBD(category, id, obd.Version3)
	if err != nil {
		return "", err
	}
	return m.shareOBD(object, req)
}

// shareOBD publishes OBD data as an object entry.
func (m *MarketService) shareOBD(object *obd.Data, req ShareRequest) (string, error) {
	object.Thing.Name, object.Thing.Description = "", "" // assets names do not travel
	object.Version = obd.Version3
	data, err := obd.Encode(object)
	if err != nil {
		return "", err
	}
	meta := req.meta(market.KindObject, object.SpriteSize)
	meta.Category = object.Thing.Category.String()
	decoded, info, err := market.ReadObject(&meta, data)
	if err != nil {
		return "", err
	}
	preview, err := market.Preview(decoded)
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
	p, err := m.session.Project()
	if err != nil {
		return "", err
	}
	var sprites [][]byte
	for _, id := range ids {
		pixels, err := p.SpritePixels(id)
		if err != nil {
			return "", err
		}
		sprites = append(sprites, pixels)
	}
	return m.sharePixels(sprites, p.SpriteSize(), req)
}

// sharePixels publishes sprites as a pack; empty sprites are left out.
func (m *MarketService) sharePixels(all [][]byte, spriteSize int, req ShareRequest) (string, error) {
	var sprites [][]byte
	for _, pixels := range all {
		if !isTransparent(pixels) {
			sprites = append(sprites, pixels)
		}
	}
	img, err := market.PackImage(sprites, spriteSize)
	if err != nil {
		return "", err
	}
	data, err := imaging.EncodePNG(img)
	if err != nil {
		return "", err
	}
	meta := req.meta(market.KindSprites, spriteSize)
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
		object, err := readObject(path)
		if err != nil {
			return "", err
		}
		return m.shareOBD(object, req)
	}
	if !slices.Contains(shareSpriteSizes, spriteSize) {
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

func (m *MarketService) submit(submission market.Submission) (string, error) {
	if m.api == nil || m.src == nil {
		return "", errNoMarket
	}
	if err := submission.Meta.Validate(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), marketSubmitTimeout)
	defer cancel()
	shared, err := m.api.Submit(ctx, submission)
	if err != nil {
		return "", err
	}
	m.settings.SetMarketShared(shared.ID, shared.DeleteToken)
	m.settings.SetMarketAuthor(submission.Meta.Author)
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
	prefs := m.settings.Get()
	token := cmp.Or(prefs.MarketShared[id], prefs.MarketAdminToken)
	if token == "" {
		return errors.New("only the author can delete this entry")
	}
	ctx, cancel := context.WithTimeout(context.Background(), marketDeleteTimeout)
	defer cancel()
	if err := m.api.Delete(ctx, id, token); err != nil {
		return err
	}
	m.settings.SetMarketShared(id, "")
	m.mu.Lock()
	if m.index != nil {
		index := *m.index
		index.Entries = slices.DeleteFunc(slices.Clone(index.Entries), func(entry market.Entry) bool { return entry.ID == id })
		m.index = &index
	}
	m.mu.Unlock()
	return nil
}
