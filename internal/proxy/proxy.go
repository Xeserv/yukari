package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"

	"github.com/Xeserv/yukari/internal/worker/cache"
)

func Handler(p *httputil.ReverseProxy, cacheDir string, upstream url.URL) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = upstream.Host
		r.URL.Host = upstream.Host
		r.URL.Scheme = upstream.Scheme

		lg := slog.With(
			"component", "handler",
			"method", r.Method,
			"path", r.URL.Path,
		)

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			lg.Info("method not supported, serving from upstream")
			p.ServeHTTP(w, r)
			return
		}

		cachePath := path.Join(cacheDir, r.URL.Path)
		if _, err := os.Stat(cachePath); err == nil {
			lg.Info("serving", "source", "cache")

			http.ServeFile(w, r, cachePath)
			return
		}

		// File does not exist in cache. Queue the download & serve from upstream
		lg.Info("serving", "source", "origin")

		cache.QueueFileForDownload(cachePath)
		p.ServeHTTP(w, r)
	}
}
