package httpserver

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/AltSoyuz/lib/logger"
)

// ServeSveltekitStaticFiles serves sveltekit static files from the given fs.FS with SPA fallback logic
func ServeSveltekitStaticFiles(sub fs.FS, fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path

		// Cache long-term only for SvelteKit assets
		if strings.HasPrefix(urlPath, "/_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		// 1) serve the exact file if it exists
		if serveIfExists(w, r, sub, fileServer, strings.TrimPrefix(urlPath, "/")) {
			return
		}

		// 2) if under /app/, also try without the /app prefix
		if after, ok := strings.CutPrefix(urlPath, "/app/"); ok {
			if serveIfExists(w, r, sub, fileServer, strings.TrimPrefix(after, "/")) {
				return
			}
		}

		// 3) if no extension, try <route>.html (eg: migration.html, about.html)
		if !hasExt(urlPath) {
			p := strings.TrimPrefix(urlPath, "/")
			if p == "" {
				p = "index"
			}
			if serveAsHTMLIfExists(w, r, sub, p+".html") {
				return
			}
			if after, ok := strings.CutPrefix(urlPath, "/app/"); ok {
				p2 := strings.TrimPrefix(after, "/")
				if p2 == "" {
					p2 = "index"
				}
				if serveAsHTMLIfExists(w, r, sub, p2+".html") {
					return
				}
			}
		}

		// 4) fallback to 200.html only for HTML navigation (not for .json/.js/.css)
		if wantsHTML(r) && !hasExt(urlPath) {
			serveEmbedFileNoCache(w, r, sub, "200.html")
			return
		}

		http.NotFound(w, r)
	}
}

// serveIfExists serves the file named 'name' from fsys using fsrv if it exists.
// HTML files are served directly with no-store to prevent Safari from caching
// permanent 304s (embed.FS always returns a zero ModTime).
func serveIfExists(w http.ResponseWriter, r *http.Request, fsys fs.FS, fsrv http.Handler, name string) bool {
	if name == "" {
		name = "index.html"
	}
	if strings.HasSuffix(name, ".html") {
		return serveAsHTMLIfExists(w, r, fsys, name)
	}
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	fsrv.ServeHTTP(w, r)
	return true
}

// serveAsHTMLIfExists serves the file named 'name' from fsys as text/html if it exists.
func serveAsHTMLIfExists(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			logger.Error("failed to close file", "name", name, "err", cerr)
		}
	}()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, f)
	return true
}

// serveEmbedFileNoCache serves the file named 'name' from fsys with no-store headers.
func serveEmbedFileNoCache(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			logger.Error("failed to close file", "name", name, "err", cerr)
		}
	}()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, f)
}

// hasExt reports whether the given path has a file extension.
func hasExt(p string) bool {
	b := path.Base(p)
	return strings.Contains(b, ".")
}

// wantsHTML reports whether the request Accept header indicates a preference for HTML.
func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}
