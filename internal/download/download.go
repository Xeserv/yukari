package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ManifestRegex = regexp.MustCompile(`/v2/([\w.]+/[\w.]+)`)
)

type Downloader struct {
	s3c      *s3.Client
	inFlight map[string]struct{}
	inp      chan downloadWork

	sync.Mutex
}

func New(s3c *s3.Client) *Downloader {
	return &Downloader{
		s3c:      s3c,
		inFlight: map[string]struct{}{},
		inp:      make(chan downloadWork, 4),
	}
}

type downloadWork struct {
	bucket, key, pullURL, mediaType, authorizationHeader string
}

func (d downloadWork) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("bucket", d.bucket),
		slog.String("key", d.key),
		slog.String("pullURL", d.pullURL),
		slog.String("mediaType", d.mediaType),
		slog.Bool("hasAuthzHeader", d.authorizationHeader != ""),
	)
}

func (d *Downloader) Fetch(bucket, key, pullURL, mediaType, authorizationHeader string) {
	d.Lock()
	_, found := d.inFlight[pullURL]
	d.Unlock()

	if found {
		return
	}

	d.inp <- downloadWork{bucket, key, pullURL, mediaType, authorizationHeader}

	d.Lock()
	d.inFlight[pullURL] = struct{}{}
	d.Unlock()
}

func (d *Downloader) Work(ctx context.Context) {
	d.work(ctx)
}

func (d *Downloader) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("returning from downloader work thread")
			return
		case work, ok := <-d.inp:
			if !ok {
				return
			}

			lg := slog.With(
				"component", "downloader",
				"work", work,
			)

			if _, err := d.s3c.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: &work.bucket,
				Key:    &work.key,
			}); err == nil {
				lg.Debug("object already in bucket, skipping")
				continue
			}

			lg.Info("fetching")

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, work.pullURL, nil)
			if err != nil {
				lg.Error("can't make request", "err", err)
			}

			req.Header.Set("Authorization", work.authorizationHeader)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lg.Error("can't fetch from remote", "err", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				lg.Error("can't download, wrong status", "u", resp.Request.URL.String(), "wantStatus", http.StatusOK, "gotStatus", resp.StatusCode)
				continue
			}

			mt := resp.Header.Get("Content-Type")
			// NOTE(Xe): God is dead. The Ollama registry returns text/plain here when they should
			// really return application/json, or ideally application/vnd.docker.distribution.manifest.v2+json.
			// We have to treat JSON as if it's not JSON here. I hate it too.
			if mt == "text/plain; charset=utf-8" {
				if err := d.hackHandleManifests(&work, resp); err != nil {
					lg.Error("can't hackily handle manifests", "err", err)
				}
			}

			if _, err := d.s3c.PutObject(ctx, &s3.PutObjectInput{
				Bucket:             &work.bucket,
				Key:                &work.key,
				ContentType:        &work.mediaType,
				Body:               resp.Body,
				ContentLength:      &resp.ContentLength,
				ContentDisposition: aws.String(resp.Header.Get("Content-Disposition")),
			}); err != nil {
				lg.Error("can't put, retrying", "err", err)
				go d.Fetch(work.bucket, work.key, work.pullURL, work.mediaType, work.authorizationHeader)
				return
			}
		}
	}
}

func (d *Downloader) hackHandleManifests(work *downloadWork, resp *http.Response) error {
	rd := io.LimitReader(resp.Body, 4*4096) // at most 16k of json
	data, err := io.ReadAll(rd)
	if err != nil {
		return fmt.Errorf("can't read data: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("can't parse manifest: %w", err)
	}

	// cheeky stuff here, put data into a buffer, and then place that in
	// resp's body.
	buf := bytes.NewBuffer(data)
	resp.Body = io.NopCloser(buf)
	resp.ContentLength = int64(buf.Len())

	work.mediaType = manifest.MediaType

	var imageName string
	matches := ManifestRegex.FindStringSubmatch(work.pullURL)
	if len(matches) == 2 {
		imageName = matches[1]
	}

	urlBase := fmt.Sprintf("https://registry.ollama.ai/v2/%s", imageName)

	go func(manifest Manifest, urlBase string) {
		for _, layer := range manifest.Layers {
			d.Fetch(work.bucket, path.Join("blobs", layer.Digest), urlBase+"/"+path.Join("blobs", layer.Digest), layer.MediaType, work.authorizationHeader)
		}
	}(manifest, urlBase)

	return nil
}
