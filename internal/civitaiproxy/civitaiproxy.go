package civitaiproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata-community/yukari/civitai"
	"github.com/tigrisdata-community/yukari/internal/download"
	"within.website/x/web"
)

func New(d *download.Downloader, c *civitai.Client, s3c *s3.Client, bucketName string) *Server {
	return &Server{
		d:          d,
		c:          c,
		s3c:        s3c,
		psc:        s3.NewPresignClient(s3c),
		bucketName: bucketName,
	}
}

type Server struct {
	d          *download.Downloader
	c          *civitai.Client
	s3c        *s3.Client
	psc        *s3.PresignClient
	bucketName string
}

// /civitai/download/{modelVersion}
func (s *Server) ModelVersion(w http.ResponseWriter, r *http.Request) {
	modelVersion := r.PathValue("modelVersion")

	dlType := r.FormValue("type")
	format := r.FormValue("format")
	size := r.FormValue("size")
	fp := r.FormValue("fp")

	usePrimary := (dlType == "" && format == "" && size == "" && fp == "")

	lg := slog.With(
		"modelVersion", modelVersion,
		"type", dlType,
		"format", format,
		"size", size,
		"fp", fp,
	)

	if modelVersion == "" {
		http.Error(w, "invalid request, need {modelVersion}", http.StatusBadRequest)
		return
	}

	modelVersionData, err := s.getModelVersion(r.Context(), modelVersion)
	if err != nil {
		lg.Error("can't fetch model version info", "modelVersion", modelVersion, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	modelInfo, err := s.getModel(r.Context(), strconv.Itoa(modelVersionData.ModelID))
	if err != nil {
		lg.Error("can't fetch model info", "model", modelVersionData.ModelID, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.putModelMetadata(r.Context(), modelInfo); err != nil {
		lg.Error("can't store model metadata into s3", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(modelVersionData.Files) == 0 {
		http.Error(w, "[unexpected] model version has no files???", http.StatusBadRequest)
		return
	}

	var targetFile civitai.Files

	for _, file := range modelVersionData.Files {
		if usePrimary && file.Primary {
			targetFile = file
		}
	}

	for _, file := range modelVersionData.Files {
		if file.Metadata.Fp == fp {
			targetFile = file
			break
		}
	}

	u, err := url.Parse(fmt.Sprintf("https://civitai.com/api/download/models/%d", modelVersionData.ID))
	if err != nil {
		panic(err)
	}

	cacheKey := fmt.Sprintf("blobs/sha256:%s", strings.ToLower(targetFile.Hashes.Sha256))
	lg = lg.With("cacheKey", cacheKey)

	if _, err := s.s3c.HeadObject(r.Context(), &s3.HeadObjectInput{
		Bucket: &s.bucketName,
		Key:    &cacheKey,
	}); err == nil {
		lg.Debug("object in bucket")

		req, err := s.psc.PresignGetObject(r.Context(), &s3.GetObjectInput{
			Bucket: &s.bucketName,
			Key:    &cacheKey,
		})
		if err != nil {
			lg.Error("can't get presigned url", "err", err)
			http.Error(w, "can't make presigned url, sorry :(", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, req.URL, http.StatusTemporaryRedirect)
		return
	}

	q := u.Query()
	q.Set("type", targetFile.Type)
	if targetFile.Metadata.Format != "" {
		q.Set("format", targetFile.Metadata.Format)
	}
	if targetFile.Metadata.Size != "" {
		q.Set("size", targetFile.Metadata.Size)
	}
	if targetFile.Metadata.Fp != "" {
		q.Set("fp", targetFile.Metadata.Fp)
	}
	u.RawQuery = q.Encode()

	s.d.Fetch(s.bucketName, cacheKey, u.String(), "application/octet-stream", "Bearer "+s.c.Token())

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		panic(err)
	}

	redirectURL, err := getRedirectURLFor(req)
	if err != nil {
		slog.Error("can't get redirect url for model download", "url", u.String(), "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	lg.Debug("redirecting", "to", redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (s *Server) putModelMetadata(ctx context.Context, modelInfo *civitai.ModelResponse) error {
	return PutModelMetadata(ctx, s.s3c, s.bucketName, modelInfo)
}

func (s *Server) getModel(ctx context.Context, model string) (*civitai.ModelResponse, error) {
	cacheKey := fmt.Sprintf("civitai/models/%s", model)

	_, err := s.s3c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucketName,
		Key:    &cacheKey,
	})

	if err != nil {
		modelInfo, err := s.c.FetchModel(ctx, model)
		if err != nil {
			return nil, err
		}

		var data bytes.Buffer
		if err := json.NewEncoder(&data).Encode(modelInfo); err != nil {
			return nil, err
		}

		if _, err := s.s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      &s.bucketName,
			Key:         &cacheKey,
			Body:        &data,
			ContentType: aws.String("application/vnd.civitai.model+json"),
		}); err != nil {
			return nil, err
		}

		return modelInfo, nil
	}

	resp, err := s.s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucketName,
		Key:    &cacheKey,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result civitai.ModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Server) getModelVersion(ctx context.Context, modelVersion string) (*civitai.ModelVersionResponse, error) {
	cacheKey := fmt.Sprintf("civitai/model-versions/%s", modelVersion)

	_, err := s.s3c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucketName,
		Key:    &cacheKey,
	})

	if err != nil {
		modelVersionInfo, err := s.c.FetchModelVersion(ctx, modelVersion)
		if err != nil {
			return nil, err
		}

		var data bytes.Buffer
		if err := json.NewEncoder(&data).Encode(modelVersionInfo); err != nil {
			return nil, err
		}

		if _, err := s.s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      &s.bucketName,
			Key:         &cacheKey,
			Body:        &data,
			ContentType: aws.String("application/vnd.civitai.model-version+json"),
		}); err != nil {
			return nil, err
		}

		return modelVersionInfo, nil
	}

	resp, err := s.s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucketName,
		Key:    &cacheKey,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result civitai.ModelVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func PutModelMetadata(ctx context.Context, s3c *s3.Client, bucketName string, modelInfo *civitai.ModelResponse) error {
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(modelInfo); err != nil {
		return fmt.Errorf("can't encode model metadata: %w", err)
	}

	key := fmt.Sprintf("civitai/models/%d", modelInfo.ID)

	if _, err := s3c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucketName,
		Key:         &key,
		ContentType: aws.String("application/vnd.civitai.model+json"),
		Body:        &data,
	}); err != nil {
		return fmt.Errorf("can't write model metadata to s3: %w", err)
	}

	return nil
}

func getRedirectURLFor(req *http.Request) (string, error) {
	cli := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusTemporaryRedirect {
		return "", web.NewError(http.StatusTemporaryRedirect, resp)
	}

	return resp.Header.Get("Location"), nil
}
