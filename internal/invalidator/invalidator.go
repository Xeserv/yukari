package invalidator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Xeserv/yukari/internal/download"
	"github.com/Xeserv/yukari/tigris"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Worker struct {
	s3c        *s3.Client
	bucketName string
	d          *download.Downloader
}

func New(s3c *s3.Client, bucketName string) *Worker {
	d := download.New(s3c)
	return &Worker{s3c, bucketName, d}
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
			q := fmt.Sprintf("`Content-Type` = \"application/vnd.docker.distribution.manifest.v2+json\" AND `Last-Modified` < %q", aWeekAgo)

			objects, err := w.s3c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket: &w.bucketName,
			}, tigris.WithQuery(q))
			if err != nil {
				slog.Error("can't list objects", "err", err)
			}

			for _, obj := range objects.Contents {
				slog.Debug("found old manifest, reprocessing", "key", *obj.Key, "lastModified", obj.LastModified.Format(time.RFC3339))

				manifestURL := fmt.Sprintf("https://registry.ollama.ai/%s", *obj.Key)

				w.d.Fetch(w.bucketName, *obj.Key, manifestURL, "application/vnd.docker.distribution.manifest.v2+json")
			}

			time.Sleep(invalidatorPeriod)
		}
	}
}
