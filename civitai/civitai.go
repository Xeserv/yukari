package civitai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	cli   *http.Client
	token string
}

func New(token string) *Client {
	return &Client{
		cli:   http.DefaultClient,
		token: token,
	}
}

func (c *Client) Token() string { return c.token }

func (c *Client) FetchModel(ctx context.Context, model string) (*ModelResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://civitai.com/api/v1/models/%s", model), nil)
	if err != nil {
		return nil, fmt.Errorf("can't make request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.token)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't get response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("can't get response: %s", resp.Status)
	}

	var result ModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("can't decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) FetchModelVersion(ctx context.Context, modelVersion string) (*ModelVersionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://civitai.com/api/v1/model-versions/%s", modelVersion), nil)
	if err != nil {
		return nil, fmt.Errorf("can't make request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.token)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't get response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("can't get response: %s", resp.Status)
	}

	var result ModelVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("can't decode response: %w", err)
	}

	return &result, nil
}

type ModelVersionResponse struct {
	ID                   int       `json:"id"`
	ModelID              int       `json:"modelId"`
	Name                 string    `json:"name"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
	TrainedWords         []string  `json:"trainedWords"`
	BaseModel            string    `json:"baseModel"`
	EarlyAccessTimeFrame int       `json:"earlyAccessTimeFrame"`
	Description          any       `json:"description"`
	Stats                Stats     `json:"stats"`
	Model                Model     `json:"model"`
	Files                []Files   `json:"files"`
	Images               []Images  `json:"images"`
	DownloadURL          string    `json:"downloadUrl"`
}

type Stats struct {
	DownloadCount int     `json:"downloadCount"`
	RatingCount   int     `json:"ratingCount"`
	Rating        float64 `json:"rating"`
}

type Model struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Nsfw bool   `json:"nsfw"`
	Poi  bool   `json:"poi"`
}

type Metadata struct {
	Fp     string `json:"fp"`
	Size   string `json:"size"`
	Format string `json:"format"`
}

type Hashes struct {
	AutoV1 string `json:"AutoV1"`
	AutoV2 string `json:"AutoV2"`
	Sha256 string `json:"SHA256"`
	Crc32  string `json:"CRC32"`
	Blake3 string `json:"BLAKE3"`
}

type Files struct {
	Name              string    `json:"name"`
	ID                int       `json:"id"`
	SizeKB            float64   `json:"sizeKB"`
	Type              string    `json:"type"`
	Metadata          Metadata  `json:"metadata"`
	PickleScanResult  string    `json:"pickleScanResult"`
	PickleScanMessage string    `json:"pickleScanMessage"`
	VirusScanResult   string    `json:"virusScanResult"`
	ScannedAt         time.Time `json:"scannedAt"`
	Hashes            Hashes    `json:"hashes"`
	Primary           bool      `json:"primary"`
	DownloadURL       string    `json:"downloadUrl"`
}

type Images struct {
	URL    string `json:"url"`
	Nsfw   bool   `json:"nsfw"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Hash   string `json:"hash"`
	Meta   any    `json:"meta"`
}

type ModelResponse struct {
	ID                    int             `json:"id"`
	Name                  string          `json:"name"`
	Description           string          `json:"description"`
	Type                  string          `json:"type"`
	Poi                   bool            `json:"poi"`
	Nsfw                  bool            `json:"nsfw"`
	AllowNoCredit         bool            `json:"allowNoCredit"`
	AllowCommercialUse    any             `json:"allowCommercialUse"`
	AllowDerivatives      bool            `json:"allowDerivatives"`
	AllowDifferentLicense bool            `json:"allowDifferentLicense"`
	Stats                 Stats           `json:"stats"`
	Creator               Creator         `json:"creator"`
	Tags                  any             `json:"tags"`
	ModelVersions         []ModelVersions `json:"modelVersions"`
}

type Creator struct {
	Username string `json:"username"`
	Image    string `json:"image"`
}

type Tags struct {
	Name string `json:"name"`
}

type ModelVersions struct {
	ID                   int       `json:"id"`
	ModelID              int       `json:"modelId"`
	Name                 string    `json:"name"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
	TrainedWords         []string  `json:"trainedWords"`
	BaseModel            string    `json:"baseModel"`
	EarlyAccessTimeFrame int       `json:"earlyAccessTimeFrame"`
	Description          string    `json:"description"`
	Stats                Stats     `json:"stats"`
	Files                []Files   `json:"files"`
	Images               []Images  `json:"images"`
	DownloadURL          string    `json:"downloadUrl"`
}
