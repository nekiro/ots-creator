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
	server := &Server{Admin: "admin", files: map[string][]byte{}, tokens: map[string]string{}}
	server.Server = httptest.NewServer(http.HandlerFunc(server.serve))
	return server
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

// maxFormBytes bounds the memory of a parsed upload.
const maxFormBytes = 16 << 20

func fail(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
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
		if err := r.ParseMultipartForm(maxFormBytes); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		var entry market.Entry
		if err := json.Unmarshal([]byte(r.FormValue("meta")), &entry); err != nil {
			fail(w, http.StatusBadRequest, "meta is not JSON")
			return
		}
		if r.FormValue("captcha") == "" {
			fail(w, http.StatusForbidden, "the captcha was not solved")
			return
		}
		read := func(field string) []byte {
			upload, _, err := r.FormFile(field)
			if err != nil {
				return nil
			}
			defer upload.Close()
			data, _ := io.ReadAll(upload)
			return data
		}
		file := read("file")
		s.seq++
		entry.ID = market.NormalizeTags([]string{entry.Name})[0] + "-00000" + string(rune('0'+s.seq%10))
		dir := market.EntriesDir + "/" + entry.ID + "/"
		entry.Created = time.Date(2026, 10, s.seq, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		entry.Bytes = len(file)
		if entry.Kind == market.KindObject {
			entry.File, entry.Preview = dir+market.ObjectFile, dir+market.PreviewFile
			s.files[entry.Preview] = read("preview")
		} else {
			entry.File, entry.Preview = dir+market.SpritesFile, dir+market.SpritesFile
		}
		s.files[entry.File] = file
		s.entries = append([]market.Entry{entry}, s.entries...)
		token := "tok-" + entry.ID
		s.tokens[entry.ID] = token
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(market.Shared{ID: entry.ID, DeleteToken: token})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/entries/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/entries/")
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if _, ok := s.tokens[id]; !ok {
			fail(w, http.StatusNotFound, "no such entry")
			return
		}
		if token != s.tokens[id] && token != s.Admin {
			fail(w, http.StatusForbidden, "wrong token")
			return
		}
		for i, entry := range s.entries {
			if entry.ID == id {
				delete(s.files, entry.File)
				delete(s.files, entry.Preview)
				s.entries = append(s.entries[:i], s.entries[i+1:]...)
				break
			}
		}
		delete(s.tokens, id)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	default:
		fail(w, http.StatusNotFound, "not found")
	}
}
