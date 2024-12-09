package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/Xeserv/yukari/internal"
	"github.com/Xeserv/yukari/internal/invalidator"
	"github.com/Xeserv/yukari/internal/proxy"
	"github.com/Xeserv/yukari/tigris"
	"github.com/facebookgo/flagenv"
)

var (
	bind              = flag.String("bind", ":9200", "host:port to bind on")
	invalidatorPeriod = flag.Duration("invalidator-period", 30*time.Minute, "how often to check for invalid manifests")
	manifestLifetime  = flag.Duration("manifest-lifetime", 240*time.Hour, "how long to keep cached manifests before invalidating them")
	slogLevel         = flag.String("slog-level", "ERROR", "log level")
	tigrisBucket      = flag.String("tigris-bucket", "yukari", "tigris bucket to store blobs and manifests in")
	upstreamRegistry  = flag.String("upstream-registry", "https://registry.ollama.ai/", "upstream registry URL")
)

func main() {
	flagenv.Parse()
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	internal.InitSlog(*slogLevel)

	upstream, err := url.Parse(*upstreamRegistry)
	if err != nil {
		log.Fatalf("can't parse upstream registry URL %q: %v", *upstreamRegistry, err)
	}

	singleHostReverseProxy := httputil.NewSingleHostReverseProxy(upstream)

	s3c, err := tigris.Client(ctx)
	if err != nil {
		log.Fatalf("can't make Tigris client: %v", err)
	}

	invalWorker := invalidator.New(s3c, *tigrisBucket)
	go invalWorker.Work(ctx, *invalidatorPeriod, *manifestLifetime)

	mux := http.NewServeMux()

	mux.Handle("/v2/", proxy.Handler(
		singleHostReverseProxy,
		*tigrisBucket,
		*upstream,
		s3c,
	))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	slog.Info("starting server on", "url", "http://0.0.0.0"+*bind)
	log.Fatalf("can't start HTTP server: %v", http.ListenAndServe(*bind, mux))
}
