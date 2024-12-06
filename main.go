package main

import (
	"flag"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/Xeserv/yukari/internal"
	"github.com/Xeserv/yukari/internal/proxy"
	"github.com/Xeserv/yukari/internal/worker/cache"
	"github.com/Xeserv/yukari/internal/worker/invalidator"
	"github.com/facebookgo/flagenv"
)

var (
	bind              = flag.String("bind", ":9200", "host:port to bind on")
	cacheDir          = flag.String("cache-dir", "./cache_dir", "local directory to cache things in")
	downloadWorkerNum = flag.Int("download-worker-num", 1, "number of parallel download workers to use")
	manifestLifetime  = flag.Duration("manifest-lifetime", 240*time.Hour, "how long to keep cached manifests before invalidating them")
	slogLevel         = flag.String("slog-level", "INFO", "log level")
	upstreamRegistry  = flag.String("upstream-registry", "https://registry.ollama.ai/", "upstream registry URL")
)

func main() {
	flagenv.Parse()
	flag.Parse()

	internal.InitSlog(*slogLevel)

	upstream, err := url.Parse(*upstreamRegistry)
	if err != nil {
		log.Fatalf("can't parse upstream registry URL %q: %v", *upstreamRegistry, err)
	}

	go invalidator.Run(*cacheDir, *manifestLifetime)
	for i := 0; i < *downloadWorkerNum; i++ {
		go cache.Run(*cacheDir, upstream)
	}

	singleHostReverseProxy := httputil.NewSingleHostReverseProxy(upstream)

	mux := http.NewServeMux()

	mux.HandleFunc("/", proxy.Handler(
		singleHostReverseProxy,
		*cacheDir,
		*upstream,
	))

	slog.Info("starting server on", "url", "http://0.0.0.0"+*bind)
	log.Fatalf("can't start HTTP server: %v", http.ListenAndServe(*bind, mux))
}
