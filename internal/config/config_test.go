package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
listen: 127.0.0.1:9999
username: admin
password_hash: sha256$120000$salt$hash
allow_cidrs:
  - 127.0.0.1/32
xkeen_bin: /xkeen
xray_bin: /xray
xray_service: /service
xray_config_dir: /configs
backup_dir: /backups
log_files:
  xkeen_ui_service: /logs/service.log
  xray_access: /logs/xray-access.log
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Listen != "127.0.0.1:9999" {
		t.Fatalf("unexpected listen: %s", cfg.Listen)
	}

	if len(cfg.AllowCIDRs) != 1 || cfg.AllowCIDRs[0] != "127.0.0.1/32" {
		t.Fatalf("unexpected allow cidrs: %#v", cfg.AllowCIDRs)
	}

	if cfg.LogFiles["xkeen_ui_service"] != "/logs/service.log" {
		t.Fatalf("unexpected log file map: %#v", cfg.LogFiles)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("unknown: value\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load to fail")
	}
}
