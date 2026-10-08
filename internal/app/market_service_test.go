package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/client"
	"github.com/nekiro/ots-creator/internal/market"
	"github.com/nekiro/ots-creator/internal/market/markettest"
	"github.com/nekiro/ots-creator/internal/settings"
	"github.com/nekiro/ots-creator/internal/thing"
)

func marketEnv(t *testing.T, srv *markettest.Server) (*MarketService, *settings.Store) {
	t.Helper()
	t.Setenv("OTS_MARKET_URL", srv.FilesURL())
	t.Setenv("OTS_MARKET_API", srv.APIURL())
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, ps, _, _, _ := newEnv(t)
	ps.New(v1098(), client.Features{})
	return NewMarketService(s, st), st
}

// TestMarketShareImportDelete shares an object and a sprite pack, imports
// both into another client and deletes them with the delete token and the
// admin token.
func TestMarketShareImportDelete(t *testing.T) {
	srv := markettest.New()
	defer srv.Close()
	m, st := marketEnv(t, srv)
	if c := m.Config(); !c.Browse || !c.Share || c.CaptchaURL != srv.URL+"/captcha" || c.CaptchaOrigin != srv.URL {
		t.Fatalf("config %+v", c)
	}
	p, _ := m.s.Project()
	img := filepath.Join(t.TempDir(), "tiles.png")
	writePNG(t, img, 64, 32, [4]byte{200, 10, 10, 255})
	ids, err := NewSpriteService(m.s).ImportImages([]string{img})
	if err != nil {
		t.Fatal(err)
	}
	ts := NewThingService(m.s)
	it, _ := ts.Get(thing.CategoryItem, 100)
	it.FrameGroups[0].Sprites[0] = ids[0]
	if err := ts.Update(it); err != nil {
		t.Fatal(err)
	}
	_ = p

	req := ShareRequest{Name: "Red box", Author: " nekiro ", Tags: []string{"Box", " red "}, License: "CC0-1.0", Captcha: "ok"}
	boxID, err := m.ShareObject(thing.CategoryItem, 100, req)
	if err != nil {
		t.Fatal(err)
	}
	if !srv.Has(market.EntriesDir + "/" + boxID + "/" + market.PreviewFile) {
		t.Fatal("object preview was not uploaded")
	}
	req.Name = "Red tiles"
	if _, err := m.ShareSprites([]uint32{ids[0], 999999}, req); err == nil {
		t.Fatal("missing sprite must fail")
	}
	req.Captcha = ""
	if _, err := m.ShareSprites(ids, req); err == nil || !strings.Contains(err.Error(), "captcha") {
		t.Fatalf("no captcha: %v", err)
	}
	req.Captcha = "ok"
	tilesID, err := m.ShareSprites(ids, req)
	if err != nil {
		t.Fatal(err)
	}
	prefs := st.Get()
	if prefs.MarketAuthor != "nekiro" || prefs.MarketShared[boxID] == "" || prefs.MarketShared[tilesID] == "" {
		t.Fatalf("settings %+v", prefs)
	}

	idx, err := m.Index(false)
	if err != nil || len(idx.Entries) != 2 || len(idx.Mine) != 2 || idx.Admin {
		t.Fatalf("index %+v %v", idx, err)
	}
	box := idx.Entries[1]
	if box.ID != boxID || box.Author != "nekiro" || strings.Join(box.Tags, ",") != "box,red" || box.SpriteSize != 32 || box.Frames != 1 || box.Category != "item" {
		t.Fatalf("entry %+v", box)
	}

	// Another computer imports both and cannot delete them.
	srv2, st2 := marketEnv(t, srv)
	for _, e := range idx.Entries {
		res, err := srv2.Import(e.ID)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		switch e.Kind {
		case market.KindObject:
			th, _ := NewThingService(srv2.s).Get(res.Category, res.ID)
			if res.Category != thing.CategoryItem || th == nil || th.FrameGroups[0].Sprites[0] == 0 {
				t.Fatalf("object %+v", res)
			}
			if f, err := srv2.Read(e.ID); err != nil || len(f.Sprites) != 1 {
				t.Fatalf("read %v", err)
			}
		case market.KindSprites:
			if len(res.Sprites) != 2 {
				t.Fatalf("sprites %+v", res)
			}
		}
	}
	if err := srv2.Delete(boxID); err == nil {
		t.Fatal("others cannot delete")
	}

	// The author deletes with the stored token, the admin deletes anything.
	if err := m.Delete(boxID); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get().MarketShared[boxID]; ok {
		t.Fatal("token must be forgotten")
	}
	prefs2 := st2.Get()
	prefs2.MarketAdminToken = srv.Admin
	st2.Update(prefs2)
	if err := srv2.Delete(tilesID); err != nil {
		t.Fatal(err)
	}
	if idx, _ := m.Index(true); len(idx.Entries) != 0 {
		t.Fatalf("left %+v", idx.Entries)
	}
}

func TestMarketNotConfigured(t *testing.T) {
	os.Unsetenv("OTS_MARKET_URL")
	os.Unsetenv("OTS_MARKET_API")
	files, api := MarketFilesURL, MarketAPIURL
	MarketFilesURL, MarketAPIURL = "", ""
	defer func() { MarketFilesURL, MarketAPIURL = files, api }()
	st, _ := settings.Open(filepath.Join(t.TempDir(), "s.json"))
	s, _, _, _, _ := newEnv(t)
	m := NewMarketService(s, st)
	if c := m.Config(); c.Browse || c.Share {
		t.Fatalf("config %+v", c)
	}
	if _, err := m.Index(false); err == nil {
		t.Fatal("index must fail")
	}
	if err := m.Delete("rat-000001"); err == nil {
		t.Fatal("delete must fail")
	}
}

func TestSettingsKeepMarketShared(t *testing.T) {
	st, _ := settings.Open(filepath.Join(t.TempDir(), "s.json"))
	stale := st.Get()
	st.SetMarketShared("rat-000001", "tok")
	stale.ListColumns = 8 // the settings window saves an older copy
	if err := st.Update(stale); err != nil {
		t.Fatal(err)
	}
	if st.Get().MarketShared["rat-000001"] != "tok" {
		t.Fatal("update must keep shared entries")
	}
}

// TestMarketShareFile shares an OBD file and an image from disk without an
// open client.
func TestMarketShareFile(t *testing.T) {
	srv := markettest.New()
	defer srv.Close()
	m, _ := marketEnv(t, srv)
	dir := t.TempDir()
	files, err := NewThingService(m.s).ExportOBD(thing.CategoryOutfit, []uint32{1}, dir)
	if err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "tiles.png")
	writePNG(t, img, 64, 32, [4]byte{10, 200, 10, 255})
	bad := filepath.Join(dir, "bad.png")
	writePNG(t, bad, 40, 32, [4]byte{10, 200, 10, 255})

	// A new session: nothing is open.
	s, _, _, _, _ := newEnv(t)
	m.s = s
	req := ShareRequest{Name: "Outfit", Author: "nekiro", License: "CC0-1.0", Captcha: "ok"}
	if _, err := m.ShareFile(files[0], 0, req); err != nil {
		t.Fatal(err)
	}
	req.Name = "Tiles"
	if _, err := m.ShareFile(img, 48, req); err == nil {
		t.Fatal("48 px sprites must fail")
	}
	if _, err := m.ShareFile(bad, 32, req); err == nil {
		t.Fatal("40 px wide image must fail")
	}
	if _, err := m.ShareFile(img, 32, req); err != nil {
		t.Fatal(err)
	}
	idx, err := m.Index(true)
	if err != nil || len(idx.Entries) != 2 {
		t.Fatalf("%v %v", idx, err)
	}
	if e := idx.Entries[1]; e.Kind != market.KindObject || e.Category != "outfit" || e.SpriteSize != 32 {
		t.Fatalf("object %+v", e)
	}
	if e := idx.Entries[0]; e.Kind != market.KindSprites || e.Sprites != 2 {
		t.Fatalf("pack %+v", e)
	}
}
