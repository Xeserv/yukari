package civitaiinvalidator

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata-community/yukari/civitai"
	"github.com/tigrisdata-community/yukari/internal/civitaiproxy"
	"github.com/tigrisdata-community/yukari/internal/download"
	"github.com/tigrisdata-community/yukari/tigris"
)

type Worker struct {
	s3c        *s3.Client
	bucketName string
	d          *download.Downloader
	c          *civitai.Client
}

func New(s3c *s3.Client, d *download.Downloader, c *civitai.Client, bucketName string) *Worker {
	return &Worker{s3c, bucketName, d, c}
}

func (w *Worker) Work(ctx context.Context, invalidatorPeriod, manifestLifetime time.Duration) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("returning from downloader work thread")
			return
		default:
			t := time.Now()
			t = t.Add(-1 * manifestLifetime)

			aWeekAgo := t.Format(time.RFC3339)
			q := fmt.Sprintf("`Content-Type` = \"application/vnd.civitai.model+json\" AND `Last-Modified` < %q", aWeekAgo)

			objects, err := w.s3c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket: &w.bucketName,
			}, tigris.WithQuery(q))
			if err != nil {
				slog.Error("can't list objects", "err", err)
			}

			for _, obj := range objects.Contents {
				slog.Debug("found old manifest, reprocessing", "key", *obj.Key, "lastModified", obj.LastModified.Format(time.RFC3339))

				modelIDStr := path.Base(*obj.Key)
				modelInfo, err := w.c.FetchModel(ctx, modelIDStr)
				if err != nil {
					slog.Error("can't get info for model", "id", modelIDStr, "err", err)
					continue
				}

				if err := civitaiproxy.PutModelMetadata(ctx, w.s3c, w.bucketName, modelInfo); err != nil {
					slog.Error("can't put model metadata", "err", err)
				}

				for _, version := range modelInfo.ModelVersions {
					for _, file := range version.Files {
						cacheKey := fmt.Sprintf("blobs/sha256:%s", strings.ToLower(file.Hashes.Sha256))

						u, err := url.Parse(fmt.Sprintf("https://civitai.com/api/download/models/%d", modelInfo.ID))
						if err != nil {
							slog.Error("can't parse model URL", "err", err)
						}

						q := u.Query()
						q.Set("type", file.Type)
						if file.Metadata.Format != "" {
							q.Set("format", file.Metadata.Format)
						}
						if file.Metadata.Size != "" {
							q.Set("size", file.Metadata.Size)
						}
						if file.Metadata.Fp != "" {
							q.Set("fp", file.Metadata.Fp)
						}
						u.RawQuery = q.Encode()

						w.d.Fetch(w.bucketName, cacheKey, u.String(), "application/octet-stream", "Bearer "+w.c.Token())
					}
				}
			}

			time.Sleep(invalidatorPeriod)
		}
	}
}
