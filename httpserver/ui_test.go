package httpserver

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeFS implements fs.FS for testing.
type fakeFS struct {
	files map[string]string
}

func (f *fakeFS) Open(name string) (fs.File, error) {
	content, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return &fakeFile{name: name, content: []byte(content)}, nil
}

type fakeFile struct {
	name    string
	content []byte
	offset  int64
}

func (f *fakeFile) Stat() (fs.FileInfo, error) {
	return &fakeFileInfo{name: f.name, size: int64(len(f.content))}, nil
}
func (f *fakeFile) Read(p []byte) (int, error) {
	if f.offset >= int64(len(f.content)) {
		return 0, io.EOF
	}
	n := copy(p, f.content[f.offset:])
	f.offset += int64(n)
	return n, nil
}
func (f *fakeFile) Close() error { return nil }
func (f *fakeFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		f.offset = int64(len(f.content)) + offset
	}
	return f.offset, nil
}

type fakeFileInfo struct {
	name string
	size int64
}

func (fi *fakeFileInfo) Name() string       { return fi.name }
func (fi *fakeFileInfo) Size() int64        { return fi.size }
func (fi *fakeFileInfo) Mode() fs.FileMode  { return 0444 }
func (fi *fakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fi *fakeFileInfo) IsDir() bool        { return false }
func (fi *fakeFileInfo) Sys() interface{}   { return nil }

func TestServeIfExists(t *testing.T) {
	fsys := &fakeFS{files: map[string]string{"index.html": "<html>ok</html>", "foo.txt": "bar"}}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("served")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	f := func(name string, want bool) {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		got := serveIfExists(w, r, fsys, handler, name)
		if got != want {
			t.Errorf("ServeIfExists(%q) = %v, want %v", name, got, want)
		}
	}

	t.Run("file exists", func(t *testing.T) { f("index.html", true) })
	t.Run("file missing", func(t *testing.T) { f("missing.html", false) })
	t.Run("empty name uses index.html", func(t *testing.T) { f("", true) })
}

func TestServeAsHTMLIfExists(t *testing.T) {
	fsys := &fakeFS{files: map[string]string{"page.html": "<html>ok</html>"}}

	f := func(name string, want bool) {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		got := serveAsHTMLIfExists(w, r, fsys, name)
		if got != want {
			t.Errorf("ServeAsHTMLIfExists(%q) = %v, want %v", name, got, want)
		}
	}

	t.Run("file exists", func(t *testing.T) { f("page.html", true) })
	t.Run("file missing", func(t *testing.T) { f("missing.html", false) })
}

func TestServeEmbedFileNoCache(t *testing.T) {
	fsys := &fakeFS{files: map[string]string{"foo.txt": "bar"}}

	t.Run("file exists", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		serveEmbedFileNoCache(w, r, fsys, "foo.txt")
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("bar")) {
			t.Errorf("expected body to contain 'bar', got %q", body)
		}
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("expected Cache-Control: no-store header, got %q", cc)
		}
	})

	t.Run("file missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		serveEmbedFileNoCache(w, r, fsys, "missing.txt")
		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestHasExt(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"foo.txt", true},
		{"foo", false},
		{"/bar/baz.js", true},
		{"dir/", false},
		{".hidden", true},
	}
	for _, c := range cases {
		if got := hasExt(c.in); got != c.want {
			t.Errorf("HasExt(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestWantsHTML(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	if !wantsHTML(r) {
		t.Error("WantsHTML should return true for Accept: text/html")
	}
	r.Header.Set("Accept", "application/json")
	if wantsHTML(r) {
		t.Error("WantsHTML should return false for Accept: application/json")
	}
}

func TestServeSveltekitStaticFiles(t *testing.T) {
	// Setup fake FS with various files
	fsys := &fakeFS{files: map[string]string{
		"index.html":          "<html>index</html>",
		"about.html":          "<html>about</html>",
		"foo.txt":             "plain",
		"_app/immutable/x.js": "console.log(1)",
		"200.html":            "<html>fallback</html>",
	}}
	// Use a fileServer that just writes the file name for test visibility
	fileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		f, err := fsys.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				t.Errorf("failed to close file: %v", cerr)
			}
		}()
		if _, err := io.Copy(w, f); err != nil {
			t.Errorf("failed to copy file: %v", err)
		}
	})

	handler := ServeSveltekitStaticFiles(fsys, fileServer)

	t.Run("serves immutable asset with cache", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/_app/immutable/x.js", nil)
		handler.ServeHTTP(w, r)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("console.log(1)")) {
			t.Errorf("expected asset body, got %q", body)
		}
		cc := resp.Header.Get("Cache-Control")
		if !strings.Contains(cc, "immutable") {
			t.Errorf("expected immutable cache header, got %q", cc)
		}
	})

	t.Run("serves exact file", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/foo.txt", nil)
		handler.ServeHTTP(w, r)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "plain" {
			t.Errorf("expected 'plain', got %q", body)
		}
	})

	t.Run("serves /app/ prefix fallback", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/app/foo.txt", nil)
		handler.ServeHTTP(w, r)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
		if string(body) == "plain" {
			t.Errorf("did not expect 'plain', got %q", body)
		}
	})

	t.Run("serves .html fallback for route", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/about", nil)
		handler.ServeHTTP(w, r)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("about")) {
			t.Errorf("expected about.html, got %q", body)
		}
	})

	t.Run("serves 200.html fallback for HTML navigation", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/doesnotexist", nil)
		r.Header.Set("Accept", "text/html")
		handler.ServeHTTP(w, r)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("fallback")) {
			t.Errorf("expected 200.html fallback, got %q", body)
		}
	})

	t.Run("404 for missing file and not HTML", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/doesnotexist.json", nil)
		handler.ServeHTTP(w, r)
		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}
