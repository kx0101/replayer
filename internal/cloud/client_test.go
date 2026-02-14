package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kx0101/replayer/internal/models"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient("http://localhost:8090", "test-api-key")
	if err != nil {
		t.Errorf("unexpected error creating client: %v", err)
	}

	if client.baseURL != "http://localhost:8090" {
		t.Errorf("expected baseURL http://localhost:8090, got %s", client.baseURL)
	}

	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey test-api-key, got %s", client.apiKey)
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestClient_Upload_Success(t *testing.T) {
	expectedID := "test-run-id"
	expectedEnv := "production"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		if r.URL.Path != "/api/v1/runs" {
			t.Errorf("expected path /api/v1/runs, got %s", r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Errorf("expected X-API-Key test-api-key, got %s", r.Header.Get("X-API-Key"))
		}

		var req UploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		if req.Environment != expectedEnv {
			t.Errorf("expected environment %s, got %s", expectedEnv, req.Environment)
		}

		resp := UploadResponse{
			ID:          expectedID,
			Environment: expectedEnv,
			CreatedAt:   time.Now(),
		}

		w.WriteHeader(http.StatusCreated)
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-api-key")
	if err != nil {
		t.Errorf("unexpected error creating client: %v", err)
	}

	req := &UploadRequest{
		Environment: expectedEnv,
		Targets:     []string{"staging.api", "prod.api"},
		Summary: models.Summary{
			TotalRequests: 100,
			Succeeded:     95,
			Failed:        5,
		},
		Results: []models.MultiEnvResult{},
		Labels:  map[string]string{"version": "v1.0.0"},
	}

	resp, err := client.Upload(req)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if resp.ID != expectedID {
		t.Errorf("expected ID %s, got %s", expectedID, resp.ID)
	}

	if resp.Environment != expectedEnv {
		t.Errorf("expected environment %s, got %s", expectedEnv, resp.Environment)
	}
}

func TestClient_Upload_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte("invalid api key"))
		if err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "invalid-key")
	if err != nil {
		t.Errorf("unexpected error creating client: %v", err)
	}

	req := &UploadRequest{
		Environment: "test",
		Targets:     []string{"api.test"},
	}

	_, err = client.Upload(req)
	if err == nil {
		t.Error("expected error for unauthorized request")
	}
}

func TestClient_Upload_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("internal server error"))
		if err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-api-key")
	if err != nil {
		t.Errorf("unexpected error creating client: %v", err)
	}

	req := &UploadRequest{
		Environment: "test",
		Targets:     []string{"api.test"},
	}

	_, err = client.Upload(req)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestClient_Upload_NetworkError(t *testing.T) {
	client, err := NewClient("http://localhost:99999", "test-api-key")
	if err != nil {
		t.Errorf("unexpected error creating client: %v", err)
	}

	req := &UploadRequest{
		Environment: "test",
		Targets:     []string{"api.test"},
	}

	_, err = client.Upload(req)
	if err == nil {
		t.Error("expected error for network failure")
	}
}

func TestClient_Upload_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte("not valid json"))
		if err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-api-key")
	if err != nil {
		t.Errorf("unexpected error creating client: %v", err)
	}

	req := &UploadRequest{
		Environment: "test",
		Targets:     []string{"api.test"},
	}

	_, err = client.Upload(req)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestUploadRequest_JSON(t *testing.T) {
	req := &UploadRequest{
		Environment: "production",
		Targets:     []string{"staging.api", "prod.api"},
		Summary: models.Summary{
			TotalRequests: 100,
			Succeeded:     95,
			Failed:        5,
		},
		Labels: map[string]string{
			"version": "v1.0.0",
			"branch":  "main",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded UploadRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.Environment != req.Environment {
		t.Errorf("expected environment %s, got %s", req.Environment, decoded.Environment)
	}

	if len(decoded.Targets) != len(req.Targets) {
		t.Errorf("expected %d targets, got %d", len(req.Targets), len(decoded.Targets))
	}

	if decoded.Labels["version"] != "v1.0.0" {
		t.Errorf("expected label version=v1.0.0, got %s", decoded.Labels["version"])
	}
}

func TestUploadRequest_EmptyLabels(t *testing.T) {
	req := &UploadRequest{
		Environment: "test",
		Targets:     []string{"api.test"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("failed to unmarshal into raw map: %v", err)
	}

	if _, exists := raw["labels"]; exists {
		t.Error("labels should be omitted when empty (omitempty)")
	}
}
