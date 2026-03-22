package commands

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xkeen-ui/internal/config"
)

func TestRunAllowedAction(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := fakeConfig(t, dir)
	service := NewService(cfg, 3*time.Second, log.New(ioDiscard{}, "", 0))

	result, err := service.Run("xkeen-version")
	if err != nil {
		t.Fatalf("run action: %v", err)
	}

	if !result.Success || result.Stdout != "xkeen version 1.0" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunRejectsUnknownAction(t *testing.T) {
	t.Parallel()

	service := NewService(fakeConfig(t, t.TempDir()), 3*time.Second, log.New(ioDiscard{}, "", 0))
	if _, err := service.Run("unknown"); err == nil {
		t.Fatal("expected unknown action error")
	}
}

func TestValidateUsesXrayBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := fakeConfig(t, dir)
	service := NewService(cfg, 3*time.Second, log.New(ioDiscard{}, "", 0))

	result := service.Validate()
	if !result.Success {
		t.Fatalf("unexpected validate result: %#v", result)
	}
}

func TestRunTimesOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := fakeConfig(t, dir)
	if err := os.WriteFile(cfg.XKeenBin, []byte("#!/bin/sh\nsleep 2\n"), 0o755); err != nil {
		t.Fatalf("rewrite xkeen binary: %v", err)
	}

	service := NewService(cfg, 200*time.Millisecond, log.New(ioDiscard{}, "", 0))
	result, err := service.Run("xkeen-version")
	if err != nil {
		t.Fatalf("run action: %v", err)
	}

	if !result.TimedOut {
		t.Fatalf("expected timeout result: %#v", result)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(data []byte) (int, error) {
	return len(data), nil
}

func fakeConfig(t *testing.T, dir string) config.Config {
	t.Helper()

	xkeenPath := filepath.Join(dir, "xkeen")
	xrayPath := filepath.Join(dir, "xray")
	servicePath := filepath.Join(dir, "S24xray")
	configDir := filepath.Join(dir, "configs")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "08_outbounds.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := os.WriteFile(xkeenPath, []byte("#!/bin/sh\ncase \"$1\" in\n  -v) echo 'xkeen version 1.0' ;;\n  -cb) echo 'backup ok' ;;\n  -cbr) echo 'restore ok' ;;\n  -tc) echo 'connection ok' ;;\n  -tpx) echo 'ports ok' ;;\n  -tfx) echo 'xray files ok' ;;\n  -tfk) echo 'xkeen files ok' ;;\n  *) echo 'bad arg' >&2; exit 1 ;;\nesac\n"), 0o755); err != nil {
		t.Fatalf("write xkeen binary: %v", err)
	}

	if err := os.WriteFile(xrayPath, []byte("#!/bin/sh\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"-test\" ]; then\n  echo 'config ok'\n  exit 0\nfi\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"-dump\" ]; then\n  echo '{\"merged\":true}'\n  exit 0\nfi\necho 'bad xray arg' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write xray binary: %v", err)
	}

	if err := os.WriteFile(servicePath, []byte("#!/bin/sh\ncase \"$1\" in\n  start|restart|status) echo 'running'; exit 0 ;;\n  stop) echo 'stopped'; exit 0 ;;\n  *) exit 1 ;;\nesac\n"), 0o755); err != nil {
		t.Fatalf("write service binary: %v", err)
	}

	return config.Config{
		Listen:        "127.0.0.1:9081",
		Username:      "admin",
		PasswordHash:  "sha256$120000$salt$hash",
		AllowCIDRs:    []string{"127.0.0.1/32"},
		XKeenBin:      xkeenPath,
		XrayBin:       xrayPath,
		XrayService:   servicePath,
		XrayConfigDir: configDir,
		BackupDir:     filepath.Join(dir, "backups"),
		LogFiles: map[string]string{
			"xray_access": filepath.Join(dir, "xray-access.log"),
			"xray_error":  filepath.Join(dir, "xray-error.log"),
			"xkeen_info":  filepath.Join(dir, "xkeen-info.log"),
			"xkeen_error": filepath.Join(dir, "xkeen-error.log"),
		},
	}
}
