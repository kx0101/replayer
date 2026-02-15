package replay

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
)

func TestRun(t *testing.T) {
	t.Run("single target success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			defer func() {
				_, err := w.Write([]byte(`{"success":true}`))
				if err != nil {
					t.Fatalf("error writing the response")
				}
			}()
		}))
		defer server.Close()

		entries := []models.LogEntry{{Method: "GET", Path: "/", Headers: map[string][]string{}}}
		args := &cli.CliArgs{Targets: []string{server.Listener.Addr().String()}, Concurrency: 1, Timeout: 5000}

		results := Run(entries, args)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		status := results[0].Responses[server.Listener.Addr().String()].Status
		if status == nil || *status != 200 {
			t.Fatalf("expected status 200, got %v", status)
		}
	})

	t.Run("multiple targets comparison", func(t *testing.T) {
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			defer func() {
				_, err := w.Write([]byte(`{"version":"v1"}`))
				if err != nil {
					t.Fatalf("error writing the response")
				}
			}()
		}))
		defer server1.Close()
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			defer func() {
				_, err := w.Write([]byte(`{"version":"v2"}`))
				if err != nil {
					t.Fatalf("error writing the response")
				}
			}()
		}))
		defer server2.Close()

		entries := []models.LogEntry{{Method: "GET", Path: "/", Headers: map[string][]string{}}}
		args := &cli.CliArgs{
			Targets:     []string{server1.Listener.Addr().String(), server2.Listener.Addr().String()},
			Concurrency: 2,
			Timeout:     5000,
			Compare:     true,
		}

		results := Run(entries, args)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		if results[0].Diff == nil || !results[0].Diff.BodyMismatch {
			t.Fatal("expected body mismatch diff to be present")
		}
	})

	t.Run("rate limiting", func(t *testing.T) {
		var requests []time.Time
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, time.Now())
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entries := []models.LogEntry{
			{Method: "GET", Path: "/", Headers: map[string][]string{}},
			{Method: "GET", Path: "/", Headers: map[string][]string{}},
			{Method: "GET", Path: "/", Headers: map[string][]string{}},
		}
		args := &cli.CliArgs{Targets: []string{server.Listener.Addr().String()}, Concurrency: 1, Timeout: 5000, RateLimit: 2}

		start := time.Now()
		Run(entries, args)
		if time.Since(start) < 1*time.Second {
			t.Error("expected rate limiting to slow requests")
		}
	})

	t.Run("concurrent execution", func(t *testing.T) {
		active, max := 0, 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			active++
			if active > max {
				max = active
			}
			time.Sleep(50 * time.Millisecond)
			active--
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entries := make([]models.LogEntry, 10)
		for i := range entries {
			entries[i] = models.LogEntry{Method: "GET", Path: "/", Headers: map[string][]string{}}
		}

		args := &cli.CliArgs{Targets: []string{server.Listener.Addr().String()}, Concurrency: 5, Timeout: 5000}

		Run(entries, args)

		if max > 5 {
			t.Errorf("expected max concurrency 5, got %d", max)
		}
	})

	t.Run("delay between requests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		entries := []models.LogEntry{
			{Method: "GET", Path: "/", Headers: map[string][]string{}},
			{Method: "GET", Path: "/", Headers: map[string][]string{}},
		}
		args := &cli.CliArgs{Targets: []string{server.Listener.Addr().String()}, Concurrency: 1, Timeout: 5000, Delay: 200}

		start := time.Now()
		Run(entries, args)

		if time.Since(start) < 200*time.Millisecond {
			t.Error("expected delay between requests")
		}
	})
}

func TestReplaySingle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			defer func() {
				_, err := w.Write([]byte(`{"ok":true}`))
				if err != nil {
					t.Fatalf("error writing the response")
				}
			}()
		}))
		defer server.Close()

		entry := models.LogEntry{Method: "GET", Path: "/", Headers: map[string][]string{}}
		res := ReplaySingle(0, entry, &http.Client{Timeout: 5 * time.Second}, server.Listener.Addr().String(), &cli.CliArgs{})

		if res.Status == nil || *res.Status != 200 {
			t.Fatal("expected 200 status")
		}
	})

	t.Run("body sent", func(t *testing.T) {
		var bodyReceived string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			bodyReceived = string(b)
		}))
		defer server.Close()

		payload := `{"a":1}`
		entry := models.LogEntry{
			Method:  "POST",
			Path:    "/",
			Body:    base64.StdEncoding.EncodeToString([]byte(payload)),
			Headers: map[string][]string{},
		}

		ReplaySingle(0, entry, &http.Client{Timeout: 5 * time.Second}, server.Listener.Addr().String(), &cli.CliArgs{})

		if bodyReceived != payload {
			t.Fatalf("expected %q, got %q", payload, bodyReceived)
		}
	})
}

func TestBuildRequest(t *testing.T) {
	entry := models.LogEntry{
		Method:  "POST",
		Path:    "/x",
		Body:    base64.StdEncoding.EncodeToString([]byte(`{"ok":1}`)),
		Headers: map[string][]string{},
	}
	req, err := BuildRequest(entry, "localhost:8080", &cli.CliArgs{})
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != "POST" {
		t.Error("expected POST method")
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type application/json")
	}
}
