// Package markettest is an in-memory market (files and API) for tests,
// without the checks of the real API.
package markettest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/nekiro/ots-creator/internal/market"
)

// Server serves the files at URL+"/files/" and the API at URL+"/".
type Server struct {
	*httptest.Server
	Admin string

	mu      sync.Mutex
	files   map[string][]byte
	entries []market.Entry
	tokens  map[string]string
	seq     int
}

// New starts a server; close it with Close.
func New() *Server {
	s := &Server{Admin: "admin", files: map[string][]byte{}, tokens: map[string]string{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// FilesURL is the files base URL.
func (s *Server) FilesURL() string { return s.URL + "/files/" }

// APIURL is the API base URL.
func (s *Server) APIURL() string { return s.URL + "/" }

// Has reports whether the bucket holds a file.
func (s *Server) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.files[key]
	return ok
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/files/"+market.IndexFile:
		json.NewEncoder(w).Encode(market.Index{Version: 1, Entries: append([]market.Entry{}, s.entries...)})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/files/"):
		data, ok := s.files[strings.TrimPrefix(r.URL.Path, "/files/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	case r.Method == http.MethodPost && r.URL.Path == "/api/entries":
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			fail(w, 400, err.Error())
			return
		}
		var e market.Entry
		if err := json.Unmarshal([]byte(r.FormValue("meta")), &e); err != nil {
			fail(w, 400, "meta is not JSON")
			return
		}
		if r.FormValue("captcha") == "" {
			fail(w, 403, "the captcha was not solved")
			return
		}
		read := func(field string) []byte {
			f, _, err := r.FormFile(field)
			if err != nil {
				return nil
			}
			defer f.Close()
			data, _ := io.ReadAll(f)
			return data
		}
		file := read("file")
		s.seq++
		e.ID = market.NormalizeTags([]string{e.Name})[0] + "-00000" + string(rune('0'+s.seq%10))
		dir := market.EntriesDir + "/" + e.ID + "/"
		e.Created = time.Date(2026, 10, s.seq, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		e.Bytes = len(file)
		if e.Kind == market.KindObject {
			e.File, e.Preview = dir+market.ObjectFile, dir+market.PreviewFile
			s.files[e.Preview] = read("preview")
		} else {
			e.File, e.Preview = dir+market.SpritesFile, dir+market.SpritesFile
		}
		s.files[e.File] = file
		s.entries = append([]market.Entry{e}, s.entries...)
		token := "tok-" + e.ID
		s.tokens[e.ID] = token
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(market.Shared{ID: e.ID, DeleteToken: token})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/entries/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/entries/")
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if _, ok := s.tokens[id]; !ok {
			fail(w, 404, "no such entry")
			return
		}
		if token != s.tokens[id] && token != s.Admin {
			fail(w, 403, "wrong token")
			return
		}
		for i, e := range s.entries {
			if e.ID == id {
				delete(s.files, e.File)
				delete(s.files, e.Preview)
				s.entries = append(s.entries[:i], s.entries[i+1:]...)
				break
			}
		}
		delete(s.tokens, id)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	default:
		fail(w, 404, "not found")
	}
}
