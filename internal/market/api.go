package market

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// API writes to the market through its HTTP API.
type API struct {
	Base string // API URL, ending in "/"
	HTTP *http.Client
}

// NewAPI returns the API at a base URL.
func NewAPI(base string) *API { return &API{Base: strings.TrimSuffix(base, "/") + "/"} }

func (a *API) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return http.DefaultClient
}

// CaptchaURL is the captcha page the share window shows in a frame; it
// posts {captcha: token} to the parent window.
func (a *API) CaptchaURL() string { return a.Base + "captcha" }

// Submission is one entry to share. Preview is required for objects.
type Submission struct {
	Meta    Meta
	Info    Info
	File    []byte
	Preview []byte
	Captcha string
}

// Shared is a published entry; DeleteToken removes it again.
type Shared struct {
	ID          string `json:"id"`
	DeleteToken string `json:"deleteToken"`
}

func (a *API) do(req *http.Request, out any) error {
	res, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) != nil || e.Error == "" {
			e.Error = res.Status
		}
		return fmt.Errorf("market: %s", e.Error)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Submit publishes an entry.
func (a *API) Submit(ctx context.Context, s Submission) (*Shared, error) {
	if err := s.Meta.Validate(); err != nil {
		return nil, err
	}
	meta, err := json.Marshal(struct {
		Meta
		Info
	}{s.Meta, s.Info})
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("meta", string(meta))
	w.WriteField("captcha", s.Captcha)
	files := []struct {
		field string
		data  []byte
	}{{"file", s.File}, {"preview", s.Preview}}
	for _, f := range files {
		if f.data == nil {
			continue
		}
		part, err := w.CreateFormFile(f.field, f.field)
		if err != nil {
			return nil, err
		}
		part.Write(f.data)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Base+"api/entries", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	var out Shared
	if err := a.do(req, &out); err != nil {
		return nil, err
	}
	if !ValidID(out.ID) || out.DeleteToken == "" {
		return nil, errors.New("market: bad reply")
	}
	return &out, nil
}

// Delete removes an entry with its delete token or the admin token.
func (a *API) Delete(ctx context.Context, id, token string) error {
	if !ValidID(id) {
		return fmt.Errorf("bad entry id %q", id)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, a.Base+"api/entries/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return a.do(req, nil)
}
