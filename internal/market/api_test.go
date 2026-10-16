package market_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nekiro/ots-creator/internal/market"
	"github.com/nekiro/ots-creator/internal/market/markettest"
)

func TestSubmitAndDelete(t *testing.T) {
	server := markettest.New()
	defer server.Close()
	api := market.NewAPI(server.APIURL())
	api.HTTP = server.Client()
	meta := market.Meta{Name: "Sand", Kind: market.KindSprites, Tags: []string{}, License: "CC0-1.0", Author: "bob", SpriteSize: 32}
	ctx := context.Background()

	shared, err := api.Submit(ctx, market.Submission{Meta: meta, File: []byte("png"), Captcha: "solved"})
	if err != nil {
		t.Fatal(err)
	}
	source := market.NewSource(server.FilesURL())
	if source.PreviewBase() != server.FilesURL() {
		t.Fatalf("preview base %q", source.PreviewBase())
	}
	index, err := source.Index(ctx)
	if err != nil || len(index.Entries) != 1 || index.Entries[0].ID != shared.ID {
		t.Fatalf("index %+v %v", index, err)
	}
	data, err := source.File(ctx, &index.Entries[0])
	if err != nil || string(data) != "png" {
		t.Fatalf("file %q %v", data, err)
	}

	if err := api.Delete(ctx, shared.ID, "wrong"); err == nil {
		t.Fatal("a wrong token must fail")
	}
	if err := api.Delete(ctx, shared.ID, shared.DeleteToken); err != nil {
		t.Fatal(err)
	}
	if server.Has(index.Entries[0].File) {
		t.Fatal("the file must be gone")
	}
	if err := api.Delete(ctx, shared.ID, server.Admin); err == nil {
		t.Fatal("a deleted entry must be gone")
	}

	// The admin token deletes any entry.
	again, err := api.Submit(ctx, market.Submission{Meta: meta, File: []byte("png"), Captcha: "solved"})
	if err != nil {
		t.Fatal(err)
	}
	if err := api.Delete(ctx, again.ID, server.Admin); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitBadReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"../x"}`))
	}))
	defer server.Close()
	meta := market.Meta{Name: "Sand", Kind: market.KindSprites, License: "CC0-1.0", Author: "bob", SpriteSize: 32}
	if _, err := market.NewAPI(server.URL).Submit(context.Background(), market.Submission{Meta: meta, Captcha: "x"}); err == nil {
		t.Fatal("a reply without a valid id and token must fail")
	}
}
