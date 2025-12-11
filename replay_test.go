package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	t.Run("single target success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			_, err := w.Write([]byte(`{"success":true}`))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}))
		defer server.Close()

		entries := []LogEntry{
			{Method: "GET", Path: "/test", Headers: make(map[string][]string)},
		}

		args := &CliArgs{
			Targets:     []string{server.URL[7:]}, // take out http://
			Concurrency: 1,
			Timeout:     5000,
		}

		results := Run(entries, args)

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}

		if results[0].Responses[server.URL[7:]].Status == nil {
			t.Error("expected status to be set")
		}

		if *results[0].Responses[server.URL[7:]].Status != 200 {
			t.Errorf("expected status 200, got %d", *results[0].Responses[server.URL[7:]].Status)
		}
	})

	t.Run("multiple targets comparison", func(t *testing.T) {
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"version":"v1"}`))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}))
		defer server1.Close()

		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"version":"v2"}`))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}))
		defer server2.Close()

		entries := []LogEntry{
			{Method: "GET", Path: "/test", Headers: make(map[string][]string)},
		}

		args := &CliArgs{
			Targets:     []string{server1.URL[7:], server2.URL[7:]},
			Concurrency: 2,
			Timeout:     5000,
			Compare:     true,
		}

		results := Run(entries, args)

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		if results[0].Diff == nil {
			t.Error("expected diff to be present")
		}

		if !results[0].Diff.BodyMismatch {
			t.Error("expected body mismatch")
		}
	})

	t.Run("rate limiting", func(t *testing.T) {
		requestTimes := make([]time.Time, 0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestTimes = append(requestTimes, time.Now())
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entries := []LogEntry{
			{Method: "GET", Path: "/test1", Headers: make(map[string][]string)},
			{Method: "GET", Path: "/test2", Headers: make(map[string][]string)},
			{Method: "GET", Path: "/test3", Headers: make(map[string][]string)},
		}

		args := &CliArgs{
			Targets:     []string{server.URL[7:]},
			Concurrency: 1,
			Timeout:     5000,
			RateLimit:   2,
		}

		start := time.Now()
		Run(entries, args)
		duration := time.Since(start)

		if duration < time.Second {
			t.Errorf("rate limiting not working: took %v", duration)
		}
	})

	t.Run("concurrent execution", func(t *testing.T) {
		activeRequests := 0
		maxConcurrent := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			activeRequests++

			if activeRequests > maxConcurrent {
				maxConcurrent = activeRequests
			}

			time.Sleep(100 * time.Millisecond)

			activeRequests--

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entries := make([]LogEntry, 10)
		for i := range entries {
			entries[i] = LogEntry{
				Method:  "GET",
				Path:    fmt.Sprintf("/test%d", i),
				Headers: make(map[string][]string),
			}
		}

		args := &CliArgs{
			Targets:     []string{server.URL[7:]},
			Concurrency: 5,
			Timeout:     5000,
		}

		Run(entries, args)

		if maxConcurrent > 5 {
			t.Errorf("exceeded concurrency limit: %d", maxConcurrent)
		}
	})

	t.Run("delay between requests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entries := []LogEntry{
			{Method: "GET", Path: "/test1", Headers: make(map[string][]string)},
			{Method: "GET", Path: "/test2", Headers: make(map[string][]string)},
		}

		args := &CliArgs{
			Targets:     []string{server.URL[7:]},
			Concurrency: 1,
			Timeout:     5000,
			Delay:       200,
		}

		start := time.Now()
		Run(entries, args)
		duration := time.Since(start)

		if duration < 200*time.Millisecond {
			t.Errorf("delay not working: took %v", duration)
		}
	})
}

func TestReplaySingle(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"result":"success"}`))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}))
		defer server.Close()

		entry := LogEntry{
			Method:  "GET",
			Path:    "/test",
			Headers: make(map[string][]string),
		}

		client := &http.Client{Timeout: 5 * time.Second}
		args := &CliArgs{}

		result := ReplaySingle(0, entry, client, server.URL[7:], args)

		if result.Status == nil {
			t.Fatal("expected status to be set")
		}

		if *result.Status != 200 {
			t.Errorf("expected status 200, got %d", *result.Status)
		}

		if result.Body == nil {
			t.Fatal("expected body to be set")
		}

		if *result.Body != `{"result":"success"}` {
			t.Errorf("unexpected body: %s", *result.Body)
		}
	})

	t.Run("request with body", func(t *testing.T) {
		receivedBody := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("unexpected error reading body: %v", err)
			}

			receivedBody = string(bodyBytes)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		payload := `{"test":"data"}`
		entry := LogEntry{
			Method:  "POST",
			Path:    "/test",
			Body:    base64.StdEncoding.EncodeToString([]byte(payload)),
			Headers: make(map[string][]string),
		}

		client := &http.Client{Timeout: 5 * time.Second}
		args := &CliArgs{}

		ReplaySingle(0, entry, client, server.URL[7:], args)

		if receivedBody != payload {
			t.Errorf("expected body %s, got %s", payload, receivedBody)
		}
	})

	t.Run("request timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entry := LogEntry{
			Method:  "GET",
			Path:    "/test",
			Headers: make(map[string][]string),
		}

		client := &http.Client{Timeout: 50 * time.Millisecond}
		args := &CliArgs{}

		result := ReplaySingle(0, entry, client, server.URL[7:], args)

		if result.Error == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestBuildRequest(t *testing.T) {
	t.Run("basic GET request", func(t *testing.T) {
		entry := LogEntry{
			Method:  "GET",
			Path:    "/users/123",
			Headers: make(map[string][]string),
		}

		args := &CliArgs{}
		req, err := BuildRequest(entry, "localhost:8080", args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.Method != "GET" {
			t.Errorf("expected method GET, got %s", req.Method)
		}

		if req.URL.String() != "http://localhost:8080/users/123" {
			t.Errorf("unexpected URL: %s", req.URL.String())
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		payload := `{"name":"test"}`
		entry := LogEntry{
			Method:  "POST",
			Path:    "/users",
			Body:    base64.StdEncoding.EncodeToString([]byte(payload)),
			Headers: make(map[string][]string),
		}

		args := &CliArgs{}
		req, err := BuildRequest(entry, "localhost:8080", args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", req.Header.Get("Content-Type"))
		}
	})

	t.Run("custom headers", func(t *testing.T) {
		entry := LogEntry{
			Method: "GET",
			Path:   "/test",
			Headers: map[string][]string{
				"X-Custom": {"value1"},
			},
		}

		args := &CliArgs{
			Headers: []string{"X-API-Key: abc123", "X-Version: 2.0"},
		}

		req, err := BuildRequest(entry, "localhost:8080", args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.Header.Get("X-Custom") != "value1" {
			t.Errorf("expected X-Custom header")
		}

		if req.Header.Get("X-API-Key") != "abc123" {
			t.Errorf("expected X-API-Key header")
		}

		if req.Header.Get("X-Version") != "2.0" {
			t.Errorf("expected X-Version header")
		}
	})

	t.Run("auth header", func(t *testing.T) {
		entry := LogEntry{
			Method:  "GET",
			Path:    "/test",
			Headers: make(map[string][]string),
		}

		args := &CliArgs{
			AuthHeader: "Bearer token123",
		}

		req, err := BuildRequest(entry, "localhost:8080", args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("expected Authorization header")
		}
	})

	t.Run("user agent default", func(t *testing.T) {
		entry := LogEntry{
			Method:  "GET",
			Path:    "/test",
			Headers: make(map[string][]string),
		}

		args := &CliArgs{}
		req, err := BuildRequest(entry, "localhost:8080", args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if req.Header.Get("User-Agent") != "go-http-replayer/1.0" {
			t.Errorf("expected default User-Agent, got %s", req.Header.Get("User-Agent"))
		}
	})
}

func TestCompareResponses(t *testing.T) {
	t.Run("identical responses", func(t *testing.T) {
		status := 200
		body := `{"result":"success"}`

		responses := map[string]ReplayResult{
			"target1": {Status: &status, Body: &body, LatencyMs: 10},
			"target2": {Status: &status, Body: &body, LatencyMs: 12},
		}

		diff := CompareResponses(responses, nil, false)

		if diff != nil {
			t.Error("expected no diff for identical responses")
		}
	})

	t.Run("status code mismatch", func(t *testing.T) {
		status1 := 200
		status2 := 404
		body := `{"result":"success"}`

		responses := map[string]ReplayResult{
			"target1": {Status: &status1, Body: &body, LatencyMs: 10},
			"target2": {Status: &status2, Body: &body, LatencyMs: 12},
		}

		diff := CompareResponses(responses, nil, false)

		if diff == nil {
			t.Fatal("expected diff for status mismatch")
		}

		if !diff.StatusMismatch {
			t.Error("expected StatusMismatch to be true")
		}
	})

	t.Run("body mismatch", func(t *testing.T) {
		status := 200
		body1 := `{"version":"v1"}`
		body2 := `{"version":"v2"}`

		responses := map[string]ReplayResult{
			"target1": {Status: &status, Body: &body1, LatencyMs: 10},
			"target2": {Status: &status, Body: &body2, LatencyMs: 12},
		}

		diff := CompareResponses(responses, nil, false)

		if diff == nil {
			t.Fatal("expected diff for body mismatch")
		}

		if !diff.BodyMismatch {
			t.Error("expected BodyMismatch to be true")
		}
	})

	t.Run("volatile fields ignored", func(t *testing.T) {
		status := 200
		body1 := `{"result":"success","timestamp":"2024-01-01T00:00:00Z"}`
		body2 := `{"result":"success","timestamp":"2024-01-01T00:01:00Z"}`

		responses := map[string]ReplayResult{
			"target1": {Status: &status, Body: &body1, LatencyMs: 10},
			"target2": {Status: &status, Body: &body2, LatencyMs: 12},
		}

		config := DefaultVolatileConfig()
		diff := CompareResponses(responses, config, false)

		if diff != nil {
			t.Error("expected no diff when only volatile fields differ")
		}
	})

	t.Run("volatile fields with stable diff", func(t *testing.T) {
		status := 200
		body1 := `{"result":"success","timestamp":"2024-01-01T00:00:00Z"}`
		body2 := `{"result":"failure","timestamp":"2024-01-01T00:01:00Z"}`

		responses := map[string]ReplayResult{
			"target1": {Status: &status, Body: &body1, LatencyMs: 10},
			"target2": {Status: &status, Body: &body2, LatencyMs: 12},
		}

		config := DefaultVolatileConfig()
		diff := CompareResponses(responses, config, false)

		if diff == nil {
			t.Fatal("expected diff for stable field changes")
		}

		if !diff.BodyMismatch {
			t.Error("expected BodyMismatch to be true")
		}

		if diff.VolatileOnly {
			t.Error("expected VolatileOnly to be false")
		}
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten!", 12, "exactly ten!"},
		{"this is a very long string that should be truncated", 20, "this is a very long ..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := Truncate(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}

func TestWrapError(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		err := fmt.Errorf("connection refused")
		result := WrapError(5, err, 100)

		if result.Index != 5 {
			t.Errorf("expected index 5, got %d", result.Index)
		}

		if result.Error == nil {
			t.Fatal("expected error to be set")
		}

		if *result.Error != "connection refused" {
			t.Errorf("unexpected error message: %s", *result.Error)
		}

		if result.LatencyMs != 100 {
			t.Errorf("expected latency 100, got %d", result.LatencyMs)
		}
	})

	t.Run("without error", func(t *testing.T) {
		result := WrapError(0, nil, 0)

		if result.Error != nil {
			t.Error("expected no error")
		}
	})
}
