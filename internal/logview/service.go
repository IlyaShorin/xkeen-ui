package logview

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type Service struct {
	files    map[string]string
	maxLines int
	maxBytes int64
}

func NewService(files map[string]string, maxLines int, maxBytes int64) *Service {
	cloned := make(map[string]string, len(files))
	for key, value := range files {
		cloned[key] = value
	}

	return &Service{
		files:    cloned,
		maxLines: maxLines,
		maxBytes: maxBytes,
	}
}

func (s *Service) Kinds() []string {
	kinds := make([]string, 0, len(s.files))
	for kind := range s.files {
		kinds = append(kinds, kind)
	}

	sort.Strings(kinds)
	return kinds
}

func (s *Service) Tail(kind string) (string, error) {
	path, ok := s.files[kind]
	if !ok {
		return "", fmt.Errorf("unknown log kind %q", kind)
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "Log file not found: " + path, nil
		}
		return "", fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat log file: %w", err)
	}

	size := info.Size()
	if size == 0 {
		return "", nil
	}

	readSize := size
	if readSize > s.maxBytes {
		readSize = s.maxBytes
	}

	buffer := make([]byte, readSize)
	if _, err := file.ReadAt(buffer, size-readSize); err != nil {
		return "", fmt.Errorf("read log tail: %w", err)
	}

	content := strings.TrimRight(string(buffer), "\n")
	if content == "" {
		return "", nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) > s.maxLines {
		lines = lines[len(lines)-s.maxLines:]
	}

	return strings.Join(lines, "\n"), nil
}
