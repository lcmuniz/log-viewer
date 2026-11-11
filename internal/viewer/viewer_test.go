package viewer

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setup creates <tmp>/logs with some files and a sibling <tmp>/logs-secret
// folder that must never be readable.
func setup(t *testing.T, maxBytes int64, checkHost bool) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	logs := filepath.Join(root, "logs")
	mustMkdir(t, logs)
	mustMkdir(t, filepath.Join(logs, "sub"))
	mustMkdir(t, filepath.Join(root, "logs-secret"))

	mustWrite(t, filepath.Join(logs, "app.log"), "line 1\nline 2\n")
	mustWrite(t, filepath.Join(logs, "notes.txt"), "hello\n")
	mustWrite(t, filepath.Join(logs, "server"), "no extension\n")
	mustWrite(t, filepath.Join(logs, ".env"), "DB_PASSWORD=x\n")
	mustWrite(t, filepath.Join(logs, "data.json"), "{}\n")
	mustWrite(t, filepath.Join(logs, "sub", "inner.log"), "inner\n")
	mustWrite(t, filepath.Join(root, "logs-secret", "x.log"), "SECRET\n")

	srv := New(Config{LogFolder: logs, RefreshInterval: 5, Theme: "dark"},
		Options{MaxBytes: maxBytes, CheckHost: checkHost, Logger: log.New(io.Discard, "", 0)})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, logs
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, ts *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func getLog(t *testing.T, ts *httptest.Server, name string) (int, string) {
	t.Helper()
	resp, body := get(t, ts, "/api/log?file="+url.QueryEscape(name))
	return resp.StatusCode, body
}

func TestListOnlyAllowedFiles(t *testing.T) {
	ts, _ := setup(t, 0, false)
	resp, body := get(t, ts, "/api/logs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var files []LogFile
	if err := json.Unmarshal([]byte(body), &files); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	got := strings.Join(names, ",")
	if got != "app.log,notes.txt,server" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(body, string(filepath.Separator)+"logs") {
		t.Errorf("listing leaks absolute paths: %s", body)
	}
}

func TestReadLog(t *testing.T) {
	ts, _ := setup(t, 0, false)
	code, body := getLog(t, ts, "app.log")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	var c LogContent
	if err := json.Unmarshal([]byte(body), &c); err != nil {
		t.Fatal(err)
	}
	if c.Content != "line 1\nline 2\n" || c.Truncated {
		t.Fatalf("unexpected content %+v", c)
	}
}

func TestRejectsUnsafeNames(t *testing.T) {
	ts, _ := setup(t, 0, false)
	cases := []string{
		"",
		"../logs-secret/x.log", // sibling folder sharing the prefix (old bug)
		"../../etc/passwd",
		"/etc/passwd",
		"sub/inner.log",
		`sub\inner.log`,
		".env",
		"data.json",
		"log-viewer.json",
		"..",
		".",
		"C:x.log",
	}
	for _, name := range cases {
		code, body := getLog(t, ts, name)
		if code == http.StatusOK {
			t.Errorf("%q: expected rejection, got 200: %s", name, body)
		}
		if strings.Contains(body, "SECRET") || strings.Contains(body, "DB_PASSWORD") {
			t.Errorf("%q: leaked content", name)
		}
	}
}

func TestMissingFileIs404WithoutPath(t *testing.T) {
	ts, logs := setup(t, 0, false)
	code, body := getLog(t, ts, "missing.log")
	if code != http.StatusNotFound {
		t.Fatalf("status %d", code)
	}
	if strings.Contains(body, logs) {
		t.Errorf("error leaks path: %s", body)
	}
}

func TestRejectsSymlink(t *testing.T) {
	ts, logs := setup(t, 0, false)
	target := filepath.Join(filepath.Dir(logs), "logs-secret", "x.log")
	if err := os.Symlink(target, filepath.Join(logs, "link.log")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	code, body := getLog(t, ts, "link.log")
	if code == http.StatusOK || strings.Contains(body, "SECRET") {
		t.Fatalf("symlink followed: %d %s", code, body)
	}
	_, list := get(t, ts, "/api/logs")
	if strings.Contains(list, "link.log") {
		t.Errorf("symlink listed: %s", list)
	}
}

func TestTailIsLimited(t *testing.T) {
	ts, logs := setup(t, 16, false)
	mustWrite(t, filepath.Join(logs, "big.log"), "aaaaaaaaaa\nbbbbbbbbbb\ncccccc\n")
	code, body := getLog(t, ts, "big.log")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	var c LogContent
	if err := json.Unmarshal([]byte(body), &c); err != nil {
		t.Fatal(err)
	}
	if !c.Truncated || c.Content != "cccccc\n" || c.Size != 29 {
		t.Fatalf("unexpected tail %+v", c)
	}
}

func TestHostGuard(t *testing.T) {
	ts, _ := setup(t, 0, true)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/logs", nil)
	req.Host = "evil.example:3000"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign Host, got %d", resp.StatusCode)
	}

	for _, host := range []string{"localhost:3000", "127.0.0.1:3000", "[::1]:3000"} {
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", host, resp.StatusCode)
		}
	}
}

func TestConfigDoesNotLeakAbsolutePath(t *testing.T) {
	ts, logs := setup(t, 0, false)
	resp, body := get(t, ts, "/api/config")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if strings.Contains(body, logs) {
		t.Errorf("config leaks path: %s", body)
	}
	if !strings.Contains(body, `"refreshInterval":5`) || !strings.Contains(body, `"theme":"dark"`) {
		t.Errorf("unexpected config: %s", body)
	}
}

func TestStaticAndSecurityHeaders(t *testing.T) {
	ts, _ := setup(t, 0, false)
	resp, body := get(t, ts, "/")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "app.js") {
		t.Fatalf("index not served: %d", resp.StatusCode)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}

func TestOnlyGET(t *testing.T) {
	ts, _ := setup(t, 0, false)
	resp, err := http.Post(ts.URL+"/api/logs", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:3000": true,
		"localhost:3000": true,
		"[::1]:3000":     true,
		"0.0.0.0:3000":   false,
		":3000":          false,
		"192.168.0.10:1": false,
		"bad":            false,
	}
	for addr, want := range cases {
		if got := IsLoopback(addr); got != want {
			t.Errorf("IsLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}
