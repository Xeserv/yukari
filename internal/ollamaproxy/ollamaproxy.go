package ollamaproxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata-community/yukari/internal/download"
)

func Handler(p *httputil.ReverseProxy, d *download.Downloader, bucketName string, upstream url.URL, s3c *s3.Client) http.Handler {
	presignClient := s3.NewPresignClient(s3c)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = upstream.Host
		r.URL.Host = upstream.Host
		r.URL.Scheme = upstream.Scheme

		lg := slog.With(
			"component", "handler",
			"method", r.Method,
		)

		switch r.Method {
		case http.MethodGet, http.MethodHead:
		default:
			lg.Error("method not supported, this is a cache, not a writable sink")
			http.Error(w, "method not supported, this is a cache, not a writable sink", http.StatusMethodNotAllowed)
			return
		}

		cachePath := r.URL.Path
		endComponent := path.Base(r.URL.Path)
		if strings.HasPrefix(endComponent, "sha256:") {
			cachePath = path.Join("blobs", endComponent)
		}

		cachePath = strings.TrimPrefix(cachePath, "/")

		lg = lg.With(
			"bucket", bucketName,
			"cachePath", cachePath,
		)

		if _, err := s3c.HeadObject(r.Context(), &s3.HeadObjectInput{
			Bucket: &bucketName,
			Key:    &cachePath,
		}); err == nil {
			lg.Debug("object in bucket")

			// if strings.Contains(r.URL.Path, "sha256:") {
			// Object is in bucket, send back a redirect to a presigned URL for blobs
			var req *v4.PresignedHTTPRequest
			var err error
			switch r.Method {
			case http.MethodHead:
				req, err = presignClient.PresignHeadObject(r.Context(), &s3.HeadObjectInput{
					Bucket: &bucketName,
					Key:    &cachePath,
				})
			case http.MethodGet:
				req, err = presignClient.PresignGetObject(r.Context(), &s3.GetObjectInput{
					Bucket: &bucketName,
					Key:    &cachePath,
				})
			}

			if err != nil {
				lg.Error("can't get presigned url", "err", err)
				http.Error(w, "can't make presigned url, sorry :(", http.StatusInternalServerError)
				return
			}

			lg.Info("serving", "from", "tigris")
			http.Redirect(w, r, req.URL, http.StatusTemporaryRedirect)
			return
		}

		// File does not exist in cache. Queue the download & serve from upstream
		lg.Info("serving", "source", "origin")

		d.Fetch(bucketName, cachePath, r.URL.String(), "", r.Header.Get("Authorization"))

		p.ServeHTTP(w, r)
	})
}
