package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nekiro/ots-creator/internal/fsutil"
)

const (
	maxPerfLogLine = 4096
	perfLogFolder  = "OTS Creator"
	perfLogFile    = "map-perf.log"
)

// appendPerfLog appends the request body (one line of timings from the map
// renderer) to map-perf.log in the user's config directory, where it can be
// read without devtools.
func appendPerfLog(w http.ResponseWriter, r *http.Request) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writePerfLog(w, r, filepath.Join(configDir, perfLogFolder))
}

func writePerfLog(w http.ResponseWriter, r *http.Request, dir string) {
	line, err := io.ReadAll(io.LimitReader(r.Body, maxPerfLogLine))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = os.MkdirAll(dir, fsutil.DirMode)
	file, err := os.OpenFile(filepath.Join(dir, perfLogFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, fsutil.FileMode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%s %s\n", time.Now().Format("15:04:05.000"), strings.TrimSpace(string(line)))
	w.WriteHeader(http.StatusNoContent)
}
