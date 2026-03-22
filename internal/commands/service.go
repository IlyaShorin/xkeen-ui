package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"xkeen-ui/internal/config"
)

type ActionSpec struct {
	Name        string
	Title       string
	Description string
	Group       string
	Path        string
	Args        []string
}

type Result struct {
	Name      string
	Title     string
	Command   string
	Stdout    string
	Stderr    string
	StartedAt time.Time
	Duration  time.Duration
	ExitCode  int
	Success   bool
	TimedOut  bool
}

type Service struct {
	timeout       time.Duration
	logger        *log.Logger
	xrayBin       string
	xrayConfigDir string
	actions       map[string]ActionSpec
	ordered       []ActionSpec
}

func NewService(cfg config.Config, timeout time.Duration, logger *log.Logger) *Service {
	specs := []ActionSpec{
		{Name: "xray-start", Title: "Start xray", Description: "Run the init.d start action.", Group: "xray", Path: cfg.XrayService, Args: []string{"start"}},
		{Name: "xray-stop", Title: "Stop xray", Description: "Run the init.d stop action.", Group: "xray", Path: cfg.XrayService, Args: []string{"stop"}},
		{Name: "xray-restart", Title: "Restart xray", Description: "Run the init.d restart action.", Group: "xray", Path: cfg.XrayService, Args: []string{"restart"}},
		{Name: "xray-status", Title: "Status", Description: "Ask the init.d service for current status.", Group: "xray", Path: cfg.XrayService, Args: []string{"status"}},
		{Name: "xkeen-backup-configs", Title: "Backup configs", Description: "Run xkeen -cb.", Group: "xkeen", Path: cfg.XKeenBin, Args: []string{"-cb"}},
		{Name: "xkeen-restore-configs", Title: "Restore configs", Description: "Run xkeen -cbr.", Group: "xkeen", Path: cfg.XKeenBin, Args: []string{"-cbr"}},
		{Name: "xkeen-test-connection", Title: "Test connection", Description: "Run xkeen -tc.", Group: "tests", Path: cfg.XKeenBin, Args: []string{"-tc"}},
		{Name: "xkeen-test-ports", Title: "Test xray ports", Description: "Run xkeen -tpx.", Group: "tests", Path: cfg.XKeenBin, Args: []string{"-tpx"}},
		{Name: "xkeen-test-xray-files", Title: "Check xray files", Description: "Run xkeen -tfx.", Group: "tests", Path: cfg.XKeenBin, Args: []string{"-tfx"}},
		{Name: "xkeen-test-xkeen-files", Title: "Check xkeen files", Description: "Run xkeen -tfk.", Group: "tests", Path: cfg.XKeenBin, Args: []string{"-tfk"}},
		{Name: "xkeen-version", Title: "xkeen version", Description: "Run xkeen -v.", Group: "xkeen", Path: cfg.XKeenBin, Args: []string{"-v"}},
	}

	actions := make(map[string]ActionSpec, len(specs))
	for _, spec := range specs {
		actions[spec.Name] = spec
	}

	return &Service{
		timeout:       timeout,
		logger:        logger,
		xrayBin:       cfg.XrayBin,
		xrayConfigDir: cfg.XrayConfigDir,
		actions:       actions,
		ordered:       specs,
	}
}

func (s *Service) AllowedActions() []ActionSpec {
	cloned := make([]ActionSpec, len(s.ordered))
	copy(cloned, s.ordered)
	return cloned
}

func (s *Service) Run(name string) (Result, error) {
	spec, ok := s.actions[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown action %q", name)
	}

	return s.run(spec.Name, spec.Title, spec.Path, spec.Args...), nil
}

func (s *Service) Status() Result {
	result, _ := s.Run("xray-status")
	return result
}

func (s *Service) Validate() Result {
	return s.run("validate", "Validate xray config", s.xrayBin, "run", "-test", "-confdir", s.xrayConfigDir)
}

func (s *Service) DumpMerged() Result {
	return s.run("dump", "Dump merged xray config", s.xrayBin, "run", "-dump", "-confdir", s.xrayConfigDir)
}

func (s *Service) run(name, title, path string, args ...string) Result {
	startedAt := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	commandText := strings.Join(append([]string{path}, args...), " ")
	command := exec.CommandContext(ctx, path, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	duration := time.Since(startedAt)

	result := Result{
		Name:      name,
		Title:     title,
		Command:   commandText,
		Stdout:    strings.TrimSpace(stdout.String()),
		Stderr:    strings.TrimSpace(stderr.String()),
		StartedAt: startedAt,
		Duration:  duration,
		ExitCode:  0,
		Success:   runErr == nil,
		TimedOut:  errors.Is(ctx.Err(), context.DeadlineExceeded),
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			if result.Stderr == "" {
				result.Stderr = runErr.Error()
			} else {
				result.Stderr += "\n" + runErr.Error()
			}
		}
	}

	if result.TimedOut {
		result.ExitCode = -1
		if result.Stderr == "" {
			result.Stderr = "command timed out"
		}
	}

	s.logger.Printf("command=%s success=%t exit=%d duration=%s", result.Command, result.Success, result.ExitCode, result.Duration)

	return result
}
