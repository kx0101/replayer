package main

import (
	"errors"
	"testing"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
	"github.com/kx0101/replayer/internal/proxy"
)

func TestExecute_ParseNginx(t *testing.T) {
	called := false
	convertNginxLogsFn = func(input, output, format string) error {
		called = true
		if input != "in.log" || output != "out.log" || format != "nginx" {
			t.Errorf("unexpected args: %s %s %s", input, output, format)
		}

		return nil
	}

	args := &cli.CliArgs{
		ParseNginx:  "out.log",
		InputFile:   "in.log",
		NginxFormat: "nginx",
	}

	code := execute(args)
	if code != cli.ExitOK {
		t.Errorf("expected ExitOK, got %v", code)
	}

	if !called {
		t.Errorf("convertNginxLogsFn was not called")
	}
}

func TestExecute_DryRun(t *testing.T) {
	called := false
	dryRunFn = func(file string) error {
		called = true
		if file != "input.txt" {
			t.Errorf("unexpected file: %s", file)
		}

		return nil
	}

	args := &cli.CliArgs{
		DryRun:    true,
		InputFile: "input.txt",
	}

	code := execute(args)
	if code != cli.ExitOK {
		t.Errorf("expected ExitOK, got %v", code)
	}

	if !called {
		t.Errorf("dryRunFn was not called")
	}
}

func TestExecute_Capture(t *testing.T) {
	called := false
	startReverseProxyFn = func(cfg *proxy.CaptureConfig) error {
		called = true
		if cfg.ListenAddr != "localhost:8080" || cfg.Upstream != "upstream:80" {
			t.Errorf("unexpected config: %+v", cfg)
		}

		return nil
	}

	args := &cli.CliArgs{
		CaptureMode:   true,
		ListenAddr:    "localhost:8080",
		Upstream:      "upstream:80",
		CaptureOut:    "capture.mp4",
		CaptureStream: true,
	}

	code := execute(args)
	if code != cli.ExitOK {
		t.Errorf("expected ExitOK, got %v", code)
	}

	if !called {
		t.Errorf("startReverseProxyFn was not called")
	}
}

func TestExecute_ReplayMode_RunError(t *testing.T) {
	readEntriesFn = func(_args *cli.CliArgs) ([]models.LogEntry, error) {
		return nil, errors.New("read error")
	}

	args := &cli.CliArgs{}
	code := execute(args)
	if code != cli.ExitRuntime {
		t.Errorf("expected ExitRuntime, got %v", code)
	}
}

func TestHandleError(t *testing.T) {
	code := handleError("some error", errors.New("oops"))
	if code != cli.ExitRuntime {
		t.Errorf("expected ExitRuntime, got %v", code)
	}
}
