package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Listen        string
	Username      string
	PasswordHash  string
	AllowCIDRs    []string
	XKeenBin      string
	XrayBin       string
	XrayService   string
	XrayConfigDir string
	BackupDir     string
	LogFiles      map[string]string
}

func Default() Config {
	return Config{
		Listen:        "0.0.0.0:9081",
		AllowCIDRs:    []string{"127.0.0.1/32", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"},
		XKeenBin:      "/opt/sbin/xkeen",
		XrayBin:       "/opt/sbin/xray",
		XrayService:   "/opt/etc/init.d/S24xray",
		XrayConfigDir: "/opt/etc/xray/configs",
		BackupDir:     "/opt/backups/xkeen-ui",
		LogFiles: map[string]string{
			"xkeen_ui_service": "/opt/var/log/xkeen-ui/service.log",
			"xray_access":      "/opt/var/log/xray/access.log",
			"xray_error":       "/opt/var/log/xray/error.log",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if err := parseYAML(string(content), &cfg); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Listen) == "":
		return errors.New("listen is required")
	case strings.TrimSpace(c.Username) == "":
		return errors.New("username is required")
	case strings.TrimSpace(c.PasswordHash) == "":
		return errors.New("password_hash is required")
	case strings.TrimSpace(c.XKeenBin) == "":
		return errors.New("xkeen_bin is required")
	case strings.TrimSpace(c.XrayBin) == "":
		return errors.New("xray_bin is required")
	case strings.TrimSpace(c.XrayService) == "":
		return errors.New("xray_service is required")
	case strings.TrimSpace(c.XrayConfigDir) == "":
		return errors.New("xray_config_dir is required")
	case strings.TrimSpace(c.BackupDir) == "":
		return errors.New("backup_dir is required")
	}

	if len(c.AllowCIDRs) == 0 {
		return errors.New("allow_cidrs must not be empty")
	}

	if len(c.LogFiles) == 0 {
		return errors.New("log_files must not be empty")
	}

	for kind, path := range c.LogFiles {
		if strings.TrimSpace(kind) == "" {
			return errors.New("log_files keys must not be empty")
		}

		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("log_files.%s must not be empty", kind)
		}
	}

	return nil
}

func parseYAML(input string, cfg *Config) error {
	scanner := bufio.NewScanner(strings.NewReader(input))
	section := ""
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		raw := strings.TrimRight(scanner.Text(), " \t")
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if indent == 0 {
			section = ""
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				return fmt.Errorf("config line %d: expected key:value", lineNo)
			}

			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if value == "" {
				switch key {
				case "allow_cidrs", "log_files":
					if key == "allow_cidrs" {
						cfg.AllowCIDRs = nil
					}
					section = key
				default:
					return fmt.Errorf("config line %d: unsupported empty section %q", lineNo, key)
				}
				continue
			}

			switch key {
			case "listen":
				cfg.Listen = parseScalar(value)
			case "username":
				cfg.Username = parseScalar(value)
			case "password_hash":
				cfg.PasswordHash = parseScalar(value)
			case "xkeen_bin":
				cfg.XKeenBin = parseScalar(value)
			case "xray_bin":
				cfg.XrayBin = parseScalar(value)
			case "xray_service":
				cfg.XrayService = parseScalar(value)
			case "xray_config_dir":
				cfg.XrayConfigDir = parseScalar(value)
			case "backup_dir":
				cfg.BackupDir = parseScalar(value)
			default:
				return fmt.Errorf("config line %d: unknown key %q", lineNo, key)
			}

			continue
		}

		switch section {
		case "allow_cidrs":
			child := strings.TrimSpace(raw)
			if !strings.HasPrefix(child, "- ") {
				return fmt.Errorf("config line %d: expected list item", lineNo)
			}
			cfg.AllowCIDRs = append(cfg.AllowCIDRs, parseScalar(strings.TrimSpace(strings.TrimPrefix(child, "- "))))
		case "log_files":
			child := strings.TrimSpace(raw)
			key, value, ok := strings.Cut(child, ":")
			if !ok {
				return fmt.Errorf("config line %d: expected map entry", lineNo)
			}
			cfg.LogFiles[strings.TrimSpace(key)] = parseScalar(strings.TrimSpace(value))
		default:
			return fmt.Errorf("config line %d: unexpected indentation", lineNo)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan config: %w", err)
	}

	return nil
}

func parseScalar(value string) string {
	value = strings.TrimSpace(value)

	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}

	return value
}
