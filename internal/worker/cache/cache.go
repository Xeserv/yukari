package cache

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func QueueFileForDownload(filePath string) {
	toDoFilePath := filePath + ".todo"
	_, err := os.Stat(toDoFilePath)
	fileExists := !errors.Is(err, os.ErrNotExist)
	if fileExists {
		// Queue file already exists. Do nothing
		return
	}
	_ = os.MkdirAll(path.Dir(toDoFilePath), 0755)
	_, _ = os.Create(toDoFilePath)
}

func Run(cacheDir string, originUrl *url.URL) {
	lg := slog.With(
		"component", "cacheWorker",
		"origin_url", originUrl.String(),
	)

	for {
		// Wall all files recursively in cacheDir
		err := filepath.Walk(cacheDir, func(fPath string, info os.FileInfo, err error) error {
			if err != nil {
				lg.Error("can't visit cache", "err", err, "path", fPath)
				return err
			}

			// Check if file is a .todo file
			if filepath.Ext(fPath) != ".todo" {
				return nil
			}

			urlPath := strings.TrimPrefix(fPath, path.Clean(cacheDir))
			urlPath = strings.TrimSuffix(urlPath, ".todo")

			fullURL := *originUrl
			fullURL.Path = urlPath

			expectedSHA256Sum := ""
			// Check if urlPath has sha256: in its name
			split := strings.Split(urlPath, "sha256:")
			if len(split) == 2 {
				expectedSHA256Sum = split[1]
			}
			err = downloadFile(fullURL.String(), fPath, expectedSHA256Sum)
			if err != nil {
				return fmt.Errorf("failed to download file %s: %w", fullURL.String(), err)
			}

			// Move .todo file
			donePath := strings.TrimSuffix(fPath, ".todo")
			err = os.Rename(fPath, donePath)
			if err != nil {
				return fmt.Errorf("failed to rename %s to %s: %w", fPath, donePath, err)
			}

			lg.Info("downloaded file")

			return nil
		})
		if err != nil {
			lg.Error("failed to walk cache dir", "err", err)
		}

		time.Sleep(10 * time.Second)
	}
}

func downloadFile(url string, downloadPath, expectedSHA256 string) error {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get %s from origin: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			// Delete .todo file
			os.Remove(downloadPath)
		}
		return fmt.Errorf("failed to get %s from origin. Status code %d. Body: %s", url, resp.StatusCode, string(b))
	}

	// Create file
	err = os.MkdirAll(path.Dir(downloadPath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path.Dir(downloadPath), err)
	}
	f, err := os.Create(downloadPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", downloadPath, err)
	}
	defer f.Close()

	h := sha256.New()

	multiWriter := io.MultiWriter(f, h)

	// Copy response to file and to response writer
	_, err = io.Copy(multiWriter, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response to file %s: %w", downloadPath, err)
	}

	fileSHA256Sum := fmt.Sprintf("%x", h.Sum(nil))
	if expectedSHA256 != "" && fileSHA256Sum != expectedSHA256 {
		return fmt.Errorf("SHA256 sum of downloaded file %q does not match expected sum %q",
			fileSHA256Sum, expectedSHA256)
	}

	return nil
}
