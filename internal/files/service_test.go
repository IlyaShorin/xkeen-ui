package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListConfigsFiltersNames(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	backupDir := t.TempDir()
	service := NewService(configDir, backupDir)

	files := map[string]string{
		"10_routing.json": "{}",
		"notes.txt":       "skip",
		"bad%.json":       "skip",
	}

	for name, content := range files {
		path := filepath.Join(configDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	configs, err := service.ListConfigs()
	if err != nil {
		t.Fatalf("list configs: %v", err)
	}

	if len(configs) != 1 || configs[0].Name != "10_routing.json" {
		t.Fatalf("unexpected configs: %#v", configs)
	}
}

func TestSaveConfigCreatesBackup(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	backupDir := t.TempDir()
	service := NewService(configDir, backupDir)

	configPath := filepath.Join(configDir, "08_outbounds.json")
	if err := os.WriteFile(configPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	backupPath, err := service.SaveConfig("08_outbounds.json", "new")
	if err != nil {
		t.Fatalf("save config: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if string(content) != "new" {
		t.Fatalf("unexpected config content: %s", string(content))
	}

	backupContent, err := os.ReadFile(filepath.Join(backupPath, "configs", "08_outbounds.json"))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}

	if string(backupContent) != "old" {
		t.Fatalf("unexpected backup content: %s", string(backupContent))
	}
}

func TestSaveConfigRejectsTraversal(t *testing.T) {
	t.Parallel()

	service := NewService(t.TempDir(), t.TempDir())
	if _, err := service.SaveConfig("../secrets", "x"); err == nil {
		t.Fatal("expected invalid file name error")
	}
}

func TestRestoreLatest(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	backupDir := t.TempDir()
	service := NewService(configDir, backupDir)

	configPath := filepath.Join(configDir, "10_routing.json")
	if err := os.WriteFile(configPath, []byte("first"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	firstBackup, err := service.CreateBackup("manual")
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}

	if err := os.WriteFile(configPath, []byte("second"), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}

	restoredPath, err := service.RestoreLatest()
	if err != nil {
		t.Fatalf("restore latest: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}

	if string(content) != "first" {
		t.Fatalf("unexpected restored content: %s", string(content))
	}

	if !strings.HasPrefix(restoredPath, firstBackup) {
		t.Fatalf("unexpected restored backup path: %s", restoredPath)
	}
}
