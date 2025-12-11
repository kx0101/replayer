package main

import (
	"os"
	"strings"
	"testing"
)

func TestReadEntries(t *testing.T) {
	t.Run("valid JSON lines file", func(t *testing.T) {
		content := `{"method":"GET","path":"/users","headers":{},"body":""}
{"method":"POST","path":"/users","headers":{"Content-Type":["application/json"]},"body":"eyJ0ZXN0IjoidmFsdWUifQ=="}
{"method":"DELETE","path":"/users/123","headers":{},"body":""}
`
		tmpfile := createTempFile(t, content)
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		args := &CliArgs{
			InputFile: tmpfile,
			Limit:     0,
		}

		entries, err := ReadEntries(args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 3 {
			t.Errorf("expected 3 entries, got %d", len(entries))
		}

		if entries[0].Method != "GET" {
			t.Errorf("expected method GET, got %s", entries[0].Method)
		}

		if entries[1].Method != "POST" {
			t.Errorf("expected method POST, got %s", entries[1].Method)
		}

		if entries[2].Method != "DELETE" {
			t.Errorf("expected method DELETE, got %s", entries[2].Method)
		}
	})

	t.Run("with limit", func(t *testing.T) {
		content := `{"method":"GET","path":"/1","headers":{},"body":""}
{"method":"GET","path":"/2","headers":{},"body":""}
{"method":"GET","path":"/3","headers":{},"body":""}
{"method":"GET","path":"/4","headers":{},"body":""}
{"method":"GET","path":"/5","headers":{},"body":""}
`
		tmpfile := createTempFile(t, content)
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		args := &CliArgs{
			InputFile: tmpfile,
			Limit:     3,
		}

		entries, err := ReadEntries(args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 3 {
			t.Errorf("expected 3 entries due to limit, got %d", len(entries))
		}
	})

	t.Run("invalid JSON line skipped", func(t *testing.T) {
		content := `{"method":"GET","path":"/valid1","headers":{},"body":""}
{invalid json line
{"method":"POST","path":"/valid2","headers":{},"body":""}
`
		tmpfile := createTempFile(t, content)
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		args := &CliArgs{
			InputFile: tmpfile,
			Limit:     0,
		}

		entries, err := ReadEntries(args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("expected 2 valid entries, got %d", len(entries))
		}

		if entries[0].Path != "/valid1" {
			t.Errorf("expected first entry path /valid1, got %s", entries[0].Path)
		}

		if entries[1].Path != "/valid2" {
			t.Errorf("expected second entry path /valid2, got %s", entries[1].Path)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpfile := createTempFile(t, "")
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		args := &CliArgs{
			InputFile: tmpfile,
			Limit:     0,
		}

		entries, err := ReadEntries(args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		args := &CliArgs{
			InputFile: "/nonexistent/file.json",
			Limit:     0,
		}

		_, err := ReadEntries(args)

		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("complex entries with headers and body", func(t *testing.T) {
		content := `{"method":"POST","path":"/api/users","headers":{"Content-Type":["application/json"],"Authorization":["Bearer token123"]},"body":"eyJuYW1lIjoiTGlha29zIiwiYWdlIjozMH0="}
`
		tmpfile := createTempFile(t, content)
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		args := &CliArgs{
			InputFile: tmpfile,
			Limit:     0,
		}

		entries, err := ReadEntries(args)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		entry := entries[0]

		if entry.Method != "POST" {
			t.Errorf("expected method POST, got %s", entry.Method)
		}

		if entry.Path != "/api/users" {
			t.Errorf("expected path /api/users, got %s", entry.Path)
		}

		if len(entry.Headers) != 2 {
			t.Errorf("expected 2 headers, got %d", len(entry.Headers))
		}

		if entry.Headers["Content-Type"][0] != "application/json" {
			t.Errorf("unexpected Content-Type: %s", entry.Headers["Content-Type"][0])
		}

		if entry.Body == "" {
			t.Error("expected body to be set")
		}
	})
}

func TestDryRun(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		content := `{"method":"GET","path":"/test1","headers":{},"body":""}
{"method":"POST","path":"/test2","headers":{},"body":""}
`
		tmpfile := createTempFile(t, content)
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		err := DryRun(tmpfile)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid path with parent directory", func(t *testing.T) {
		err := DryRun("../etc/passwd")

		if err == nil {
			t.Error("expected error for path with ..")
		}

		if !strings.Contains(err.Error(), "invalid input path") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := DryRun("/nonexistent/file.json")

		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("invalid JSON handled gracefully", func(t *testing.T) {
		content := `{"method":"GET","path":"/valid","headers":{},"body":""}
invalid json
{"method":"POST","path":"/valid2","headers":{},"body":""}
`
		tmpfile := createTempFile(t, content)
		defer func() {
			err := os.Remove(tmpfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}()

		err := DryRun(tmpfile)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestParseEntries(t *testing.T) {
	t.Run("basic parsing", func(t *testing.T) {
		content := `{"method":"GET","path":"/test","headers":{},"body":""}
`
		reader := strings.NewReader(content)

		entries, err := parseEntries(reader, 0, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		content := `{"method":"GET","path":"/1","headers":{},"body":""}
{"method":"GET","path":"/2","headers":{},"body":""}
{"method":"GET","path":"/3","headers":{},"body":""}
`
		reader := strings.NewReader(content)

		entries, err := parseEntries(reader, 2, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}
	})

	t.Run("dry run mode", func(t *testing.T) {
		content := `{"method":"GET","path":"/test","headers":{},"body":""}
`
		reader := strings.NewReader(content)

		entries, err := parseEntries(reader, 0, true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries in dry run, got %d", len(entries))
		}
	})

	t.Run("skip invalid JSON and continue", func(t *testing.T) {
		content := `{"method":"GET","path":"/valid1","headers":{},"body":""}
not valid json
{"method":"POST","path":"/valid2","headers":{},"body":""}
also invalid
{"method":"DELETE","path":"/valid3","headers":{},"body":""}
`
		reader := strings.NewReader(content)

		entries, err := parseEntries(reader, 0, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 3 {
			t.Errorf("expected 3 valid entries, got %d", len(entries))
		}

		if entries[0].Path != "/valid1" {
			t.Errorf("expected first path /valid1, got %s", entries[0].Path)
		}

		if entries[1].Path != "/valid2" {
			t.Errorf("expected second path /valid2, got %s", entries[1].Path)
		}

		if entries[2].Path != "/valid3" {
			t.Errorf("expected third path /valid3, got %s", entries[2].Path)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		reader := strings.NewReader("")

		entries, err := parseEntries(reader, 0, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "replayer_test_*.json")
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
