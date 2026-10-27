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

type API struct {
	Base string // API URL, ending in "/"
	HTTP *http.Client
}

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

const maxReplyBytes = 1 << 20

func (a *API) do(req *http.Request, out any) error {
	response, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxReplyBytes))
	if err != nil {
		return err
	}
	if response.StatusCode >= 300 {
		var reply struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &reply) != nil || reply.Error == "" {
			reply.Error = response.Status
		}
		return fmt.Errorf("market: %s", reply.Error)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func (a *API) Submit(ctx context.Context, submission Submission) (*Shared, error) {
	if err := submission.Meta.Validate(); err != nil {
		return nil, err
	}
	meta, err := json.Marshal(struct {
		Meta
		Info
	}{submission.Meta, submission.Info})
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("meta", string(meta))
	form.WriteField("captcha", submission.Captcha)
	files := []struct {
		field string
		data  []byte
	}{{"file", submission.File}, {"preview", submission.Preview}}
	for _, file := range files {
		if file.data == nil {
			continue
		}
		part, err := form.CreateFormFile(file.field, file.field)
		if err != nil {
			return nil, err
		}
		part.Write(file.data)
	}
	if err := form.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Base+"api/entries", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	var out Shared
	if err := a.do(req, &out); err != nil {
		return nil, err
	}
	if !ValidID(out.ID) || out.DeleteToken == "" {
		return nil, errors.New("market: bad reply")
	}
	return &out, nil
}

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
