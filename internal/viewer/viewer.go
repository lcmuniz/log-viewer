// Package viewer implements the HTTP API and static UI of log-viewer.
//
// Security model: the server is meant to run on the developer's own machine.
// It only serves regular files that sit directly inside the configured log
// folder and have an allowed extension. Path separators, symlinks,
// directories and dot-files are rejected. When bound to a loopback address
// the Host header is checked to block DNS-rebinding attacks.
package viewer

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed static
var staticFiles embed.FS

// DefaultMaxBytes is how much of the end of a log file is returned by default.
const DefaultMaxBytes int64 = 1 << 20 // 1 MiB

// Config is the runtime configuration exposed to the UI.
type Config struct {
	LogFolder       string `json:"logFolder"`
	RefreshInterval int    `json:"refreshInterval"`
	Theme           string `json:"theme"`
}

// LogFile is one entry of the file list.
type LogFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// LogContent is the response of /api/log.
type LogContent struct {
	Content      string `json:"content"`
	LastModified string `json:"lastModified"`
	Size         int64  `json:"size"`
	Truncated    bool   `json:"truncated"`
}

type publicConfig struct {
	Folder          string `json:"folder"`
	RefreshInterval int    `json:"refreshInterval"`
	Theme           string `json:"theme"`
}

// Server serves the log viewer.
type Server struct {
	cfg      Config
	maxBytes int64
	// checkHost enables the DNS-rebinding protection (only localhost names
	// are accepted in the Host header).
	checkHost bool
	logger    *log.Logger
}

// Options configures a Server.
type Options struct {
	MaxBytes  int64
	CheckHost bool
	Logger    *log.Logger
}

// New returns a Server for cfg. cfg.LogFolder must be an absolute path.
func New(cfg Config, opts Options) *Server {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	return &Server{cfg: cfg, maxBytes: opts.MaxBytes, checkHost: opts.CheckHost, logger: opts.Logger}
}

// Handler returns the HTTP handler with all routes and middleware.
func (s *Server) Handler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err) // embedded FS is fixed at build time
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("GET /api/log", s.handleLog)
	mux.Handle("GET /", http.FileServer(http.FS(sub)))

	return s.securityHeaders(s.hostGuard(mux))
}

// IsLoopback reports whether addr (host:port) listens only on loopback.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.checkHost && !isLocalHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, publicConfig{
		Folder:          filepath.Base(s.cfg.LogFolder),
		RefreshInterval: s.cfg.RefreshInterval,
		Theme:           s.cfg.Theme,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.cfg.LogFolder)
	if err != nil {
		s.logger.Printf("list %s: %v", s.cfg.LogFolder, err)
		http.Error(w, "could not list log folder", http.StatusInternalServerError)
		return
	}

	files := []LogFile{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !allowedName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, LogFile{Name: e.Name(), Size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	writeJSON(w, files)
}

func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("file")
	if !allowedName(name) {
		http.Error(w, "invalid file name", http.StatusBadRequest)
		return
	}

	path := filepath.Join(s.cfg.LogFolder, name)
	content, err := readTail(path, s.maxBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegular) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		s.logger.Printf("read %s: %v", path, err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
		return
	}
	writeJSON(w, content)
}

// allowedName accepts only plain file names (no directories, no dot-files)
// with a .log or .txt extension, or no extension at all.
func allowedName(name string) bool {
	if name == "" || len(name) > 255 {
		return false
	}
	if strings.ContainsAny(name, `/\:`) || strings.ContainsRune(name, 0) {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".log", ".txt", "":
		return true
	}
	return false
}

var errNotRegular = errors.New("not a regular file")

// readTail returns at most maxBytes from the end of the file at path.
// Symlinks and non-regular files are refused.
func readTail(path string, maxBytes int64) (LogContent, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return LogContent{}, err
	}
	if !info.Mode().IsRegular() {
		return LogContent{}, errNotRegular
	}

	f, err := os.Open(path)
	if err != nil {
		return LogContent{}, err
	}
	defer f.Close()

	// Re-check after open to avoid swapping the file for a symlink in between.
	opened, err := f.Stat()
	if err != nil {
		return LogContent{}, err
	}
	if !os.SameFile(info, opened) {
		return LogContent{}, errNotRegular
	}

	size := opened.Size()
	truncated := false
	if size > maxBytes {
		if _, err := f.Seek(size-maxBytes, io.SeekStart); err != nil {
			return LogContent{}, err
		}
		truncated = true
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return LogContent{}, err
	}
	if truncated {
		// Drop the partial first line so the view starts on a line boundary.
		if i := strings.IndexByte(string(data), '\n'); i >= 0 && i+1 < len(data) {
			data = data[i+1:]
		}
	}

	return LogContent{
		Content:      string(data),
		LastModified: opened.ModTime().Format(time.RFC3339),
		Size:         size,
		Truncated:    truncated,
	}, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
