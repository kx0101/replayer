package main

import (
	"errors"
	"testing"
)

func TestExecute_ParseNginx(t *testing.T) {
	called := false
	convertNginxLogs = func(input, output, format string) error {
		called = true
		if input != "in.log" || output != "out.log" || format != "nginx" {
			t.Errorf("unexpected args: %s %s %s", input, output, format)
		}

		return nil
	}

	args := &CliArgs{
		ParseNginx:  "out.log",
		InputFile:   "in.log",
		NginxFormat: "nginx",
	}

	code := execute(args)
	if code != ExitOK {
		t.Errorf("expected ExitOK, got %v", code)
	}

	if !called {
		t.Errorf("convertNginxLogs was not called")
	}
}

func TestExecute_DryRun(t *testing.T) {
	called := false
	dryRun = func(file string) error {
		called = true
		if file != "input.txt" {
			t.Errorf("unexpected file: %s", file)
		}

		return nil
	}

	args := &CliArgs{
		DryRun:    true,
		InputFile: "input.txt",
	}

	code := execute(args)
	if code != ExitOK {
		t.Errorf("expected ExitOK, got %v", code)
	}

	if !called {
		t.Errorf("dryRun was not called")
	}
}

func TestExecute_Capture(t *testing.T) {
	called := false
	startReverseProxy = func(cfg *CaptureConfig) error {
		called = true
		if cfg.ListenAddr != "localhost:8080" || cfg.Upstream != "upstream:80" {
			t.Errorf("unexpected config: %+v", cfg)
		}

		return nil
	}

	args := &CliArgs{
		CaptureMode:   true,
		ListenAddr:    "localhost:8080",
		Upstream:      "upstream:80",
		CaptureOut:    "capture.mp4",
		CaptureStream: true,
	}

	code := execute(args)
	if code != ExitOK {
		t.Errorf("expected ExitOK, got %v", code)
	}

	if !called {
		t.Errorf("startReverseProxy was not called")
	}
}

func TestExecute_ReplayMode_RunError(t *testing.T) {
	readEntries = func(_args *CliArgs) ([]LogEntry, error) {
		return nil, errors.New("read error")
	}

	args := &CliArgs{}
	code := execute(args)
	if code != ExitRuntime {
		t.Errorf("expected ExitRuntime, got %v", code)
	}
}

func TestHandleError(t *testing.T) {
	code := handleError("some error", errors.New("oops"))
	if code != ExitRuntime {
		t.Errorf("expected ExitRuntime, got %v", code)
	}
}
