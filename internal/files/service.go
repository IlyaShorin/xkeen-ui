package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ConfigFile struct {
	Name    string
	Size    int64
	ModTime time.Time
}

type Service struct {
	configDir string
	backupDir string
}

func NewService(configDir, backupDir string) *Service {
	return &Service{
		configDir: configDir,
		backupDir: backupDir,
	}
}

func (s *Service) ConfigDir() string {
	return s.configDir
}

func (s *Service) ListConfigs() ([]ConfigFile, error) {
	entries, err := os.ReadDir(s.configDir)
	if err != nil {
		return nil, fmt.Errorf("read config dir: %w", err)
	}

	files := make([]ConfigFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || !validName(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat config file %q: %w", entry.Name(), err)
		}

		if !info.Mode().IsRegular() {
			continue
		}

		files = append(files, ConfigFile{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	return files, nil
}

func (s *Service) ReadConfig(name string) (string, error) {
	path, err := s.resolve(name)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read config %q: %w", name, err)
	}

	return string(content), nil
}

func (s *Service) SaveConfig(name, content string) (string, error) {
	path, err := s.resolve(name)
	if err != nil {
		return "", err
	}

	backupPath, err := s.CreateBackup("save")
	if err != nil {
		return "", err
	}

	if err := writeAtomic(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write config %q: %w", name, err)
	}

	return backupPath, nil
}

func (s *Service) CreateBackup(kind string) (string, error) {
	files, err := s.ListConfigs()
	if err != nil {
		return "", err
	}

	timestamp := time.Now().UTC().Format("20060102-150405.000")
	backupRoot := filepath.Join(s.backupDir, timestamp+"-"+sanitize(kind))
	backupConfigsDir := filepath.Join(backupRoot, "configs")

	if err := os.MkdirAll(backupConfigsDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	for _, file := range files {
		sourcePath := filepath.Join(s.configDir, file.Name)
		targetPath := filepath.Join(backupConfigsDir, file.Name)
		if err := copyFile(sourcePath, targetPath, 0o644); err != nil {
			return "", fmt.Errorf("backup config %q: %w", file.Name, err)
		}
	}

	return backupRoot, nil
}

func (s *Service) RestoreLatest() (string, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return "", fmt.Errorf("read backup dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupConfigsDir := filepath.Join(s.backupDir, entry.Name(), "configs")
		info, err := os.Stat(backupConfigsDir)
		if err != nil || !info.IsDir() {
			continue
		}

		configEntries, err := os.ReadDir(backupConfigsDir)
		if err != nil {
			return "", fmt.Errorf("read backup configs: %w", err)
		}

		for _, configEntry := range configEntries {
			if configEntry.IsDir() || !validName(configEntry.Name()) {
				continue
			}

			sourcePath := filepath.Join(backupConfigsDir, configEntry.Name())
			targetPath := filepath.Join(s.configDir, configEntry.Name())
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return "", fmt.Errorf("restore read %q: %w", configEntry.Name(), err)
			}

			if err := writeAtomic(targetPath, content, 0o644); err != nil {
				return "", fmt.Errorf("restore write %q: %w", configEntry.Name(), err)
			}
		}

		return filepath.Join(s.backupDir, entry.Name()), nil
	}

	return "", errors.New("no backups found")
}

func (s *Service) resolve(name string) (string, error) {
	if !validName(name) {
		return "", fmt.Errorf("invalid config file name %q", name)
	}

	return filepath.Join(s.configDir, name), nil
}

func validName(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || !strings.HasSuffix(name, ".json") {
		return false
	}

	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '.', char == '-', char == '_':
		default:
			return false
		}
	}

	return true
}

func sanitize(kind string) string {
	if kind == "" {
		return "backup"
	}

	var builder strings.Builder
	for _, char := range kind {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char + ('a' - 'A'))
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}

	return strings.Trim(builder.String(), "-")
}

func writeAtomic(path string, content []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tempPath := file.Name()

	defer os.Remove(tempPath)

	if _, err := file.Write(content); err != nil {
		file.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := file.Chmod(mode); err != nil {
		file.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func copyFile(sourcePath, targetPath string, mode fs.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open target file: %w", err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}

	if err := target.Sync(); err != nil {
		return fmt.Errorf("sync target file: %w", err)
	}

	return nil
}
