package market

import (
	"bytes"
	"context"
	"image/gif"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nekiro/ots-creator/internal/imaging"
	"github.com/nekiro/ots-creator/internal/obd"
	"github.com/nekiro/ots-creator/internal/thing"
)

func pixels(r, g byte) []byte {
	px := make([]byte, 32*32*4)
	for i := 0; i < len(px); i += 4 {
		px[i], px[i+1], px[i+2], px[i+3] = r, g, 0x40, 255
	}
	return px
}

// sampleOutfit has a 1-frame idle group and a 2-frame walking group.
func sampleOutfit(t *testing.T) []byte {
	t.Helper()
	th := thing.New(0, thing.CategoryOutfit)
	walk := thing.NewFrameGroup()
	walk.Type = thing.FrameGroupWalking
	walk.PatternX, walk.Frames = 4, 2
	walk.Sprites = make([]uint32, 8)
	walk.Durations = []thing.FrameDuration{{Min: 300, Max: 300}, {Min: 300, Max: 300}}
	th.FrameGroups = append(th.FrameGroups, walk)
	d := &obd.Data{Version: obd.Version3, ClientVersion: 1098, Thing: th, SpriteSize: 32}
	for gi, g := range th.FrameGroups {
		var s []obd.Sprite
		for i := range g.Sprites {
			s = append(s, obd.Sprite{Pixels: pixels(byte(gi*50+i*20), byte(i))})
		}
		d.Sprites = append(d.Sprites, s)
	}
	data, err := obd.Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func validMeta() *Meta {
	return &Meta{Name: "Rat", Kind: KindObject, Category: "outfit", Tags: []string{"monster"}, License: "CC0-1.0", Author: "alice", SpriteSize: 32}
}

func TestMetaValidate(t *testing.T) {
	if err := validMeta().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []func(m *Meta){
		func(m *Meta) { m.Name = "" },
		func(m *Meta) { m.Name = " Rat" },
		func(m *Meta) { m.Name = strings.Repeat("x", MaxName+1) },
		func(m *Meta) { m.Author = "" },
		func(m *Meta) { m.Author = "a\tb" },
		func(m *Meta) { m.Author = strings.Repeat("x", MaxAuthor+1) },
		func(m *Meta) { m.Category = "monster" },
		func(m *Meta) { m.Kind = "script" },
		func(m *Meta) { m.Tags = []string{"Big Rat"} },
		func(m *Meta) { m.Tags = []string{"a", "a"} },
		func(m *Meta) { m.License = "GPL" },
		func(m *Meta) { m.SpriteSize = 48 },
		func(m *Meta) { m.Kind = KindSprites }, // packs have no category
	}
	for i, f := range bad {
		m := validMeta()
		f(m)
		if m.Validate() == nil {
			t.Errorf("case %d must fail: %+v", i, m)
		}
	}
	if got := NormalizeTags([]string{" Big  Rat ", "big-rat", "", "Sewer"}); strings.Join(got, ",") != "big-rat,sewer" {
		t.Fatalf("tags %v", got)
	}
}

func TestReadObjectAndPreview(t *testing.T) {
	data := sampleOutfit(t)
	d, info, err := ReadObject(validMeta(), data)
	if err != nil {
		t.Fatal(err)
	}
	if info.Frames != 3 || info.Sprites != 12 || info.ClientVersion != 1098 || info.Width != 1 {
		t.Fatalf("info %+v", info)
	}
	wrong := validMeta()
	wrong.Category = "item"
	if _, _, err := ReadObject(wrong, data); err == nil {
		t.Fatal("category mismatch must fail")
	}
	wrong = validMeta()
	wrong.SpriteSize = 64
	if _, _, err := ReadObject(wrong, data); err == nil {
		t.Fatal("sprite size mismatch must fail")
	}
	g, err := Preview(d)
	if err != nil {
		t.Fatal(err)
	}
	anim, err := gif.DecodeAll(bytes.NewReader(g))
	if err != nil {
		t.Fatal(err)
	}
	// Walking frames at the preview speed.
	if len(anim.Image) != 2 || anim.Delay[0] != 10 {
		t.Fatalf("preview: %d frames, delay %v", len(anim.Image), anim.Delay)
	}
}

func TestColorize(t *testing.T) {
	if c := hsiToRGB(0); c != [3]uint8{255, 255, 255} {
		t.Fatalf("white %v", c)
	}
	base := []byte{200, 200, 200, 255, 200, 200, 200, 255}
	mask := []byte{255, 255, 0, 255, 0, 0, 0, 0} // head, nothing
	colorize(base, mask)
	head := hsiToRGB(defaultColors[0])
	if base[0] != uint8(200*int(head[0])/255) || base[4] != 200 {
		t.Fatalf("colorized %v", base)
	}
}

func TestReadPackSkipsEmpty(t *testing.T) {
	m := &Meta{Kind: KindSprites, SpriteSize: 32}
	img, _ := PackImage([][]byte{pixels(1, 1), make([]byte, 32*32*4), pixels(2, 2)}, 32)
	png, _ := imaging.EncodePNG(img)
	sprites, info, err := ReadPack(m, png)
	if err != nil || len(sprites) != 2 || info.Sprites != 2 || info.Width != 3 {
		t.Fatalf("%d %+v %v", len(sprites), info, err)
	}
	m.SpriteSize = 64
	if _, _, err := ReadPack(m, png); err == nil {
		t.Fatal("96x32 is not a grid of 64 px sprites")
	}
}

func TestSourceFallback(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			io.WriteString(w, `{"version":1,"entries":[{"id":"rat-000001","name":"Rat","file":"entries/rat-000001/thing.obd"}]}`)
		case "/entries/rat-000001/thing.obd":
			io.WriteString(w, "obd")
		default:
			http.NotFound(w, r)
		}
	}))
	defer good.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer down.Close()
	s := &Source{Bases: []string{down.URL + "/", good.URL + "/"}}
	idx, err := s.Index(context.Background())
	if err != nil || len(idx.Entries) != 1 {
		t.Fatalf("%v %v", idx, err)
	}
	data, err := s.File(context.Background(), &idx.Entries[0])
	if err != nil || string(data) != "obd" {
		t.Fatalf("%q %v", data, err)
	}
	if _, err := s.File(context.Background(), &Entry{ID: "rat-000001", File: "../secret"}); err == nil {
		t.Fatal("paths outside the entry must fail")
	}
	empty := httptest.NewServer(http.NotFoundHandler())
	defer empty.Close()
	if idx, err := NewSource(empty.URL).Index(context.Background()); err != nil || len(idx.Entries) != 0 {
		t.Fatalf("empty market: %v", err)
	}
	future := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"version":99,"entries":[]}`)
	}))
	defer future.Close()
	if _, err := NewSource(future.URL).Index(context.Background()); err == nil {
		t.Fatal("newer index format must fail")
	}
}

func TestAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"error":"the captcha expired; solve it again"}`)
	}))
	defer srv.Close()
	api := NewAPI(srv.URL)
	_, err := api.Submit(context.Background(), Submission{Meta: *validMeta(), File: []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "captcha expired") {
		t.Fatalf("err %v", err)
	}
	if _, err := api.Submit(context.Background(), Submission{Meta: Meta{}}); err == nil {
		t.Fatal("invalid meta must fail before sending")
	}
	if err := api.Delete(context.Background(), "../x", "t"); err == nil {
		t.Fatal("bad id must fail")
	}
	if api.CaptchaURL() != srv.URL+"/captcha" {
		t.Fatal(api.CaptchaURL())
	}
}
