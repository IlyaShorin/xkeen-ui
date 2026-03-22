package logview

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTailReturnsLastLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "xray.log")
	content := "1\n2\n3\n4\n5\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	service := NewService(map[string]string{"xray_error": path}, 3, 1024)
	tail, err := service.Tail("xray_error")
	if err != nil {
		t.Fatalf("tail log: %v", err)
	}

	if tail != "3\n4\n5" {
		t.Fatalf("unexpected tail: %q", tail)
	}
}

func TestTailRejectsUnknownKind(t *testing.T) {
	t.Parallel()

	service := NewService(map[string]string{}, 3, 1024)
	if _, err := service.Tail("missing"); err == nil {
		t.Fatal("expected tail error")
	}
}

func TestTailReturnsFriendlyMessageForMissingFile(t *testing.T) {
	t.Parallel()

	service := NewService(map[string]string{"xkeen_error": "/tmp/definitely-missing-log-file.log"}, 3, 1024)
	tail, err := service.Tail("xkeen_error")
	if err != nil {
		t.Fatalf("unexpected tail error: %v", err)
	}

	if tail == "" {
		t.Fatal("expected friendly missing-file message")
	}
}
