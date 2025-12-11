package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNginxParser_ParseLine(t *testing.T) {
	t.Run("combined log format", func(t *testing.T) {
		parser := NewNginxParser("combined")
		line := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/users/123 HTTP/1.1" 200 1234 "http://example.com/home" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`

		entry, err := parser.parseLine(line)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if entry.Method != "GET" {
			t.Errorf("expected method GET, got %s", entry.Method)
		}

		if entry.Path != "/api/users/123" {
			t.Errorf("expected path /api/users/123, got %s", entry.Path)
		}

		if len(entry.Headers) == 0 {
			t.Error("expected headers to be set")
		}

		if entry.Headers["User-Agent"][0] != "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" {
			t.Errorf("unexpected User-Agent: %s", entry.Headers["User-Agent"][0])
		}
	})

	t.Run("common log format", func(t *testing.T) {
		parser := NewNginxParser("common")
		line := `192.168.1.1 - - [10/Dec/2024:14:23:45 +0000] "POST /api/login HTTP/1.1" 201 567`

		entry, err := parser.parseLine(line)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if entry.Method != "POST" {
			t.Errorf("expected method POST, got %s", entry.Method)
		}

		if entry.Path != "/api/login" {
			t.Errorf("expected path /api/login, got %s", entry.Path)
		}
	})

	t.Run("path with query string", func(t *testing.T) {
		parser := NewNginxParser("combined")
		line := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /search?q=test&page=1 HTTP/1.1" 200 1234 "-" "curl/7.68.0"`

		entry, err := parser.parseLine(line)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if entry.Path != "/search" {
			t.Errorf("expected path /search (without query), got %s", entry.Path)
		}
	})

	t.Run("different HTTP methods", func(t *testing.T) {
		parser := NewNginxParser("combined")
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

		for _, method := range methods {
			line := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "` + method + ` /api/test HTTP/1.1" 200 100 "-" "test"`

			entry, err := parser.parseLine(line)

			if err != nil {
				t.Fatalf("unexpected error for method %s: %v", method, err)
			}

			if entry.Method != method {
				t.Errorf("expected method %s, got %s", method, entry.Method)
			}
		}
	})

	t.Run("missing referrer and user agent", func(t *testing.T) {
		parser := NewNginxParser("combined")
		line := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/test HTTP/1.1" 200 100 "-" "-"`

		entry, err := parser.parseLine(line)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, exists := entry.Headers["User-Agent"]; exists {
			t.Error("User-Agent should not exist for '-' value")
		}

		if _, exists := entry.Headers["Referrer"]; exists {
			t.Error("Referrer should not exist for '-' value")
		}
	})

	t.Run("invalid log format", func(t *testing.T) {
		parser := NewNginxParser("combined")
		line := `this is not a valid nginx log line`

		_, err := parser.parseLine(line)

		if err == nil {
			t.Error("expected error for invalid log format")
		}

		if !strings.Contains(err.Error(), "does not match nginx log format") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestNginxParser_ParseFile(t *testing.T) {
	t.Run("valid log file", func(t *testing.T) {
		content := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/users HTTP/1.1" 200 1234 "http://example.com" "Mozilla/5.0"
192.168.1.1 - - [10/Dec/2024:14:23:46 +0000] "POST /api/users HTTP/1.1" 201 567 "-" "curl/7.68.0"
10.0.0.1 - - [10/Dec/2024:14:23:47 +0000] "DELETE /api/users/123 HTTP/1.1" 204 0 "-" "-"
`
		inputFile := createNginxTempFile(t, content)
		defer func() {
			err := os.Remove(inputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		outputFile := inputFile + ".out"
		defer func() {
			err := os.Remove(outputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		parser := NewNginxParser("combined")
		err := parser.ParseFile(inputFile, outputFile)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")

		if len(lines) != 3 {
			t.Errorf("expected 3 lines in output, got %d", len(lines))
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		if entry.Method != "GET" {
			t.Errorf("expected method GET, got %s", entry.Method)
		}

		if entry.Path != "/api/users" {
			t.Errorf("expected path /api/users, got %s", entry.Path)
		}
	})

	t.Run("file with invalid lines", func(t *testing.T) {
		content := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /valid1 HTTP/1.1" 200 100 "-" "test"
invalid line that should be skipped
192.168.1.1 - - [10/Dec/2024:14:23:46 +0000] "POST /valid2 HTTP/1.1" 201 200 "-" "curl"
another invalid line
`
		inputFile := createNginxTempFile(t, content)
		defer func() {
			err := os.Remove(inputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		outputFile := inputFile + ".out"
		defer func() {
			err := os.Remove(outputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		parser := NewNginxParser("combined")
		err := parser.ParseFile(inputFile, outputFile)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")

		if len(lines) != 2 {
			t.Errorf("expected 2 valid lines, got %d", len(lines))
		}
	})

	t.Run("empty lines skipped", func(t *testing.T) {
		content := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /test1 HTTP/1.1" 200 100 "-" "test"

192.168.1.1 - - [10/Dec/2024:14:23:46 +0000] "GET /test2 HTTP/1.1" 200 200 "-" "curl"
   
`
		inputFile := createNginxTempFile(t, content)
		defer func() {
			err := os.Remove(inputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		outputFile := inputFile + ".out"
		defer func() {
			err := os.Remove(outputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		parser := NewNginxParser("combined")
		err := parser.ParseFile(inputFile, outputFile)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")

		if len(lines) != 2 {
			t.Errorf("expected 2 lines (empty lines skipped), got %d", len(lines))
		}
	})

	t.Run("input file not found", func(t *testing.T) {
		parser := NewNginxParser("combined")
		err := parser.ParseFile("/nonexistent/input.log", "/tmp/output.json")

		if err == nil {
			t.Error("expected error for nonexistent input file")
		}
	})

	t.Run("path traversal protection input", func(t *testing.T) {
		parser := NewNginxParser("combined")
		err := parser.ParseFile("../etc/passwd", "/tmp/output.json")

		if err == nil {
			t.Error("expected error for path with ..")
		}

		if !strings.Contains(err.Error(), "invalid output path") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("path traversal protection output", func(t *testing.T) {
		content := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /test HTTP/1.1" 200 100 "-" "test"`
		inputFile := createNginxTempFile(t, content)
		defer func() {
			err := os.Remove(inputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		parser := NewNginxParser("combined")
		err := parser.ParseFile(inputFile, "../tmp/output.json")

		if err == nil {
			t.Error("expected error for output path with ..")
		}

		if !strings.Contains(err.Error(), "invalid output path") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestNewNginxParser(t *testing.T) {
	t.Run("explicit combined format", func(t *testing.T) {
		parser := NewNginxParser("combined")

		if parser.format != "combined" {
			t.Errorf("expected format combined, got %s", parser.format)
		}
	})

	t.Run("explicit common format", func(t *testing.T) {
		parser := NewNginxParser("common")

		if parser.format != "common" {
			t.Errorf("expected format common, got %s", parser.format)
		}
	})

	t.Run("default format", func(t *testing.T) {
		parser := NewNginxParser("")

		if parser.format != "combined" {
			t.Errorf("expected default format combined, got %s", parser.format)
		}
	})
}

func TestConvertNginxLogs(t *testing.T) {
	t.Run("successful conversion", func(t *testing.T) {
		content := `127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/test HTTP/1.1" 200 100 "-" "test"`
		inputFile := createNginxTempFile(t, content)
		defer func() {
			err := os.Remove(inputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		outputFile := inputFile + ".out"
		defer func() {
			err := os.Remove(outputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		err := ConvertNginxLogs(inputFile, outputFile, "combined")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(outputFile); os.IsNotExist(err) {
			t.Error("output file was not created")
		}
	})

	t.Run("common format conversion", func(t *testing.T) {
		content := `192.168.1.1 - - [10/Dec/2024:14:23:45 +0000] "POST /api/data HTTP/1.1" 201 567`
		inputFile := createNginxTempFile(t, content)
		defer func() {
			err := os.Remove(inputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		outputFile := inputFile + ".out"
		defer func() {
			err := os.Remove(outputFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		err := ConvertNginxLogs(inputFile, outputFile, "common")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output: %v", err)
		}

		var entry LogEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatalf("failed to parse output: %v", err)
		}

		if entry.Method != "POST" {
			t.Errorf("expected method POST, got %s", entry.Method)
		}

		if entry.Path != "/api/data" {
			t.Errorf("expected path /api/data, got %s", entry.Path)
		}
	})
}

func createNginxTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "nginx_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	return tmpfile.Name()
}
