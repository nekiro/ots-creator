package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Source reads the public market bucket over HTTP. Bases are tried in
// order (mirrors); each ends in "/".
type Source struct {
	Bases []string
	HTTP  *http.Client
}

// NewSource returns a source reading one base URL.
func NewSource(base string) *Source {
	return &Source{Bases: []string{strings.TrimSuffix(base, "/") + "/"}}
}

// PreviewBase is the base URL the UI loads preview images from.
func (s *Source) PreviewBase() string { return s.Bases[0] }

func (s *Source) get(ctx context.Context, bases []string, name string, limit int64) ([]byte, error) {
	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	var errs []error
	for _, base := range bases {
		data, err := func() ([]byte, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+name, nil)
			if err != nil {
				return nil, err
			}
			if name == IndexFile {
				req.Header.Set("Cache-Control", "no-cache") // a refresh shows new entries
			}
			response, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer response.Body.Close()
			if response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("%s: %w", base+name, errNotFound)
			}
			if response.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("%s: %s", base+name, response.Status)
			}
			data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
			if err == nil && int64(len(data)) > limit {
				err = fmt.Errorf("%s is larger than %d bytes", name, limit)
			}
			return data, err
		}()
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		errs = append(errs, err)
	}
	return nil, errors.Join(errs...)
}

var errNotFound = errors.New("not found")

// maxIndexBytes bounds index.json (about 50k entries).
const maxIndexBytes = 32 << 20

// Index downloads index.json.
func (s *Source) Index(ctx context.Context) (*Index, error) {
	data, err := s.get(ctx, s.Bases, IndexFile, maxIndexBytes)
	if errors.Is(err, errNotFound) {
		return &Index{Version: IndexVersion, Entries: []Entry{}}, nil // nothing shared yet
	}
	if err != nil {
		return nil, err
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("%s: %w", IndexFile, err)
	}
	if index.Version > IndexVersion {
		return nil, errors.New("the market needs a newer OTS Creator")
	}
	if index.Entries == nil {
		index.Entries = []Entry{}
	}
	return &index, nil
}

// File downloads the file of an entry (thing.obd or sprites.png).
func (s *Source) File(ctx context.Context, entry *Entry) ([]byte, error) {
	if !ValidID(entry.ID) || !strings.HasPrefix(entry.File, EntriesDir+"/"+entry.ID+"/") {
		return nil, fmt.Errorf("bad entry %q", entry.ID)
	}
	return s.get(ctx, s.Bases, entry.File, MaxPackBytes)
}
