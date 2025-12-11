package main

import (
	"testing"
)

func TestApply(t *testing.T) {
	t.Run("no filters applied", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
			{Method: "DELETE", Path: "/users/123"},
		}

		args := &CliArgs{
			FilterMethod: "",
			FilterPath:   "",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 3 {
			t.Errorf("expected 3 entries (no filtering), got %d", len(filtered))
		}
	})

	t.Run("filter by method GET", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
			{Method: "GET", Path: "/posts"},
			{Method: "DELETE", Path: "/users/123"},
			{Method: "GET", Path: "/comments"},
		}

		args := &CliArgs{
			FilterMethod: "GET",
			FilterPath:   "",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 3 {
			t.Errorf("expected 3 GET entries, got %d", len(filtered))
		}

		for _, entry := range filtered {
			if entry.Method != "GET" {
				t.Errorf("expected only GET methods, found %s", entry.Method)
			}
		}
	})

	t.Run("filter by method POST", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
			{Method: "POST", Path: "/posts"},
			{Method: "DELETE", Path: "/users/123"},
		}

		args := &CliArgs{
			FilterMethod: "POST",
			FilterPath:   "",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 2 {
			t.Errorf("expected 2 POST entries, got %d", len(filtered))
		}

		for _, entry := range filtered {
			if entry.Method != "POST" {
				t.Errorf("expected only POST methods, found %s", entry.Method)
			}
		}
	})

	t.Run("filter by path substring", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/api/users"},
			{Method: "POST", Path: "/api/users"},
			{Method: "GET", Path: "/api/posts"},
			{Method: "DELETE", Path: "/api/users/123"},
			{Method: "GET", Path: "/health"},
		}

		args := &CliArgs{
			FilterMethod: "",
			FilterPath:   "/api/users",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 3 {
			t.Errorf("expected 3 entries with /api/users, got %d", len(filtered))
		}

		for _, entry := range filtered {
			if entry.Path != "/api/users" && entry.Path != "/api/users/123" {
				t.Errorf("unexpected path: %s", entry.Path)
			}
		}
	})

	t.Run("filter by both method and path", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/api/users"},
			{Method: "POST", Path: "/api/users"},
			{Method: "GET", Path: "/api/posts"},
			{Method: "DELETE", Path: "/api/users/123"},
			{Method: "POST", Path: "/api/users/456"},
		}

		args := &CliArgs{
			FilterMethod: "POST",
			FilterPath:   "/api/users",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 2 {
			t.Errorf("expected 2 entries (POST + /api/users), got %d", len(filtered))
		}

		for _, entry := range filtered {
			if entry.Method != "POST" {
				t.Errorf("expected only POST methods, found %s", entry.Method)
			}

			if entry.Path != "/api/users" && entry.Path != "/api/users/456" {
				t.Errorf("unexpected path: %s", entry.Path)
			}
		}
	})

	t.Run("filter matches nothing", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/posts"},
		}

		args := &CliArgs{
			FilterMethod: "DELETE",
			FilterPath:   "",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 0 {
			t.Errorf("expected 0 entries, got %d", len(filtered))
		}
	})

	t.Run("case insensitive method filtering", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "get", Path: "/posts"},
			{Method: "Get", Path: "/comments"},
		}

		args := &CliArgs{
			FilterMethod: "GET",
			FilterPath:   "",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 3 {
			t.Errorf("expected 3 entries (case insensitive), got %d", len(filtered))
		}
	})

	t.Run("partial path matching", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/api/v1/users"},
			{Method: "GET", Path: "/api/v2/users"},
			{Method: "GET", Path: "/users"},
			{Method: "GET", Path: "/api/posts"},
		}

		args := &CliArgs{
			FilterMethod: "",
			FilterPath:   "users",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 3 {
			t.Errorf("expected 3 entries containing 'users', got %d", len(filtered))
		}

		for _, entry := range filtered {
			if entry.Path != "/api/v1/users" && entry.Path != "/api/v2/users" && entry.Path != "/users" {
				t.Errorf("unexpected path without 'users': %s", entry.Path)
			}
		}
	})

	t.Run("empty entries list", func(t *testing.T) {
		entries := []LogEntry{}

		args := &CliArgs{
			FilterMethod: "GET",
			FilterPath:   "/test",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 0 {
			t.Errorf("expected 0 entries, got %d", len(filtered))
		}
	})

	t.Run("filter specific endpoint", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/api/checkout"},
			{Method: "POST", Path: "/api/checkout"},
			{Method: "GET", Path: "/api/cart"},
			{Method: "POST", Path: "/api/orders"},
			{Method: "GET", Path: "/api/checkout/success"},
		}

		args := &CliArgs{
			FilterMethod: "POST",
			FilterPath:   "/api/checkout",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 1 {
			t.Errorf("expected 1 entry, got %d", len(filtered))
		}

		if filtered[0].Method != "POST" || filtered[0].Path != "/api/checkout" {
			t.Errorf("unexpected filtered entry: %+v", filtered[0])
		}
	})

	t.Run("filter with special characters in path", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/api/users?page=1"},
			{Method: "GET", Path: "/api/users?page=2"},
			{Method: "GET", Path: "/api/posts"},
		}

		args := &CliArgs{
			FilterMethod: "",
			FilterPath:   "?page=",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 2 {
			t.Errorf("expected 2 entries with query params, got %d", len(filtered))
		}
	})

	t.Run("all entries filtered out", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "GET", Path: "/posts"},
			{Method: "GET", Path: "/comments"},
		}

		args := &CliArgs{
			FilterMethod: "POST",
			FilterPath:   "/nonexistent",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 0 {
			t.Errorf("expected all entries filtered out, got %d", len(filtered))
		}
	})

	t.Run("method filter with lowercase input", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
		}

		args := &CliArgs{
			FilterMethod: "get",
			FilterPath:   "",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 1 {
			t.Errorf("expected 1 entry (case insensitive), got %d", len(filtered))
		}

		if filtered[0].Method != "GET" {
			t.Errorf("expected GET method, got %s", filtered[0].Method)
		}
	})

	t.Run("complex filtering scenario", func(t *testing.T) {
		entries := []LogEntry{
			{Method: "GET", Path: "/api/v1/users/123"},
			{Method: "POST", Path: "/api/v1/users"},
			{Method: "GET", Path: "/api/v2/users/456"},
			{Method: "PUT", Path: "/api/v1/users/123"},
			{Method: "GET", Path: "/api/v1/posts"},
			{Method: "DELETE", Path: "/api/v1/users/123"},
			{Method: "GET", Path: "/health"},
		}

		args := &CliArgs{
			FilterMethod: "GET",
			FilterPath:   "/api/v1/users",
		}

		filtered := Apply(entries, args)

		if len(filtered) != 1 {
			t.Errorf("expected 1 entry (GET + /api/v1/users), got %d", len(filtered))
		}

		if filtered[0].Path != "/api/v1/users/123" {
			t.Errorf("unexpected path: %s", filtered[0].Path)
		}
	})
}
