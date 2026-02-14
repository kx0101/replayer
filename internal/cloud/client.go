package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/kx0101/replayer/internal/models"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type UploadRequest struct {
	Environment string                  `json:"environment"`
	Targets     []string                `json:"targets"`
	Summary     models.Summary          `json:"summary"`
	Results     []models.MultiEnvResult `json:"results"`
	Labels      map[string]string       `json:"labels,omitempty"`
}

type UploadResponse struct {
	ID          string    `json:"id"`
	Environment string    `json:"environment"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewClient(baseURL, apiKey string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseURL: %w", err)
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("invalid scheme in baseURL")
	}

	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip != nil && ip.IsPrivate() {
		return nil, fmt.Errorf("baseURL cannot be private IP")
	}

	return &Client{
		baseURL: parsed.String(),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) Upload(req *UploadRequest) (*UploadResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/runs", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq) // #nosec G704: baseURL validated in constructor and not user-controlled
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	defer func() {
		err = resp.Body.Close()
		if err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("upload failed: %s - %s", resp.Status, string(respBody))
	}

	var uploadResp UploadResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &uploadResp, nil
}
