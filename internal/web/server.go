package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"xkeen-ui/internal/auth"
	"xkeen-ui/internal/commands"
	"xkeen-ui/internal/config"
	"xkeen-ui/internal/files"
	"xkeen-ui/internal/logview"
	webassets "xkeen-ui/web"
)

type ActionGroup struct {
	Name    string
	Actions []commands.ActionSpec
}

type ConfigFileView struct {
	Name         string
	SizeLabel    string
	ModTimeLabel string
}

type EventView struct {
	Title   string
	Summary string
	Output  string
	At      string
	Success bool
}

type PageData struct {
	Title          string
	AssetVersion   string
	Active         string
	Notice         string
	Error          string
	LastEvent      *EventView
	Status         *commands.Result
	StatusRunning  bool
	ActionGroups   []ActionGroup
	ConfigDir      string
	ConfigFiles    []ConfigFileView
	CurrentFile    string
	Content        string
	LogKinds       []string
	CurrentLogKind string
	LogContent     string
	MergedConfig   string
	CurrentUser    string
	LoginError     string
	LoginUsername  string
}

type Server struct {
	cfg          config.Config
	templates    *template.Template
	files        *files.Service
	commands     *commands.Service
	logs         *logview.Service
	events       *EventStore
	logger       *log.Logger
	sessions     *auth.SessionManager
	authUsername string
	passwordHash string
	assetVersion string
}

func NewServer(cfg config.Config, sessionManager *auth.SessionManager, fileService *files.Service, commandService *commands.Service, logService *logview.Service, logger *log.Logger) (*Server, error) {
	templates, err := template.New("").ParseFS(webassets.TemplatesFS(), "*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Server{
		cfg:          cfg,
		templates:    templates,
		files:        fileService,
		commands:     commandService,
		logs:         logService,
		events:       &EventStore{},
		logger:       logger,
		sessions:     sessionManager,
		authUsername: cfg.Username,
		passwordHash: cfg.PasswordHash,
		assetVersion: strconv.FormatInt(time.Now().UTC().Unix(), 10),
	}, nil
}

func (s *Server) Handler() http.Handler {
	root := http.NewServeMux()
	protected := http.NewServeMux()

	root.Handle("/static/", s.staticHandler())
	root.HandleFunc("/healthz", s.handleHealthz)
	root.HandleFunc("/login", s.handleLogin)
	root.HandleFunc("/logout", s.handleLogout)

	protected.HandleFunc("/", s.handleDashboard)
	protected.HandleFunc("/configs", s.handleConfigs)
	protected.HandleFunc("/configs/", s.handleConfigsSubtree)
	protected.HandleFunc("/logs", s.handleLogsIndex)
	protected.HandleFunc("/logs/", s.handleLogs)
	protected.HandleFunc("/actions/", s.handleAction)
	protected.HandleFunc("/fragments/status", s.handleStatusFragment)

	root.Handle("/", s.sessions.RequireAuth("/login")(protected))

	return root
}

func (s *Server) staticHandler() http.Handler {
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(webassets.StaticFS())))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.sessions.Username(r); ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		data := PageData{
			Title:        "Login",
			AssetVersion: s.assetVersion,
		}
		s.render(w, "login", data, http.StatusOK)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.renderLoginError(w, "invalid login form")
			return
		}

		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		if username != s.authUsername || !auth.VerifyPassword(password, s.passwordHash) {
			s.renderLoginError(w, "invalid credentials")
			return
		}

		if err := s.sessions.Set(w, username); err != nil {
			s.renderLoginError(w, "failed to create session")
			return
		}

		http.Redirect(w, r, safeRedirectTarget(r.FormValue("next"), "/"), http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := s.basePage("Dashboard", "dashboard", r)
	data.ActionGroups = groupActions(s.commands.AllowedActions())
	s.render(w, "dashboard", data, http.StatusOK)
}

func (s *Server) handleStatusFragment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.commands.Status()
	data := PageData{
		Status:        &status,
		StatusRunning: status.Success && status.ExitCode == 0 && !status.TimedOut,
	}

	s.renderFragment(w, "status_panel", data, http.StatusOK)
}

func (s *Server) handleConfigs(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/configs" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configFiles, err := s.files.ListConfigs()
	if err != nil {
		s.renderError(w, "configs", "Configs", "configs", err, http.StatusInternalServerError, r, nil)
		return
	}

	fileViews := make([]ConfigFileView, 0, len(configFiles))
	for _, file := range configFiles {
		fileViews = append(fileViews, ConfigFileView{
			Name:         file.Name,
			SizeLabel:    formatSize(file.Size),
			ModTimeLabel: file.ModTime.Local().Format("2006-01-02 15:04:05"),
		})
	}

	data := s.basePage("Configs", "configs", r)
	data.ConfigDir = s.files.ConfigDir()
	data.ConfigFiles = fileViews
	s.render(w, "configs", data, http.StatusOK)
}

func (s *Server) handleConfigsSubtree(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/configs/")

	switch {
	case rest == "":
		http.NotFound(w, r)
	case rest == "validate":
		s.handleValidate(w, r)
	case rest == "merged":
		s.handleMerged(w, r)
	case strings.HasSuffix(rest, "/save"):
		name := strings.TrimSuffix(rest, "/save")
		s.handleSaveConfig(w, r, name)
	default:
		s.handleConfigEditor(w, r, rest)
	}
}

func (s *Server) handleConfigEditor(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileName, ok := cleanConfigName(name)
	if !ok {
		http.NotFound(w, r)
		return
	}

	content, err := s.files.ReadConfig(fileName)
	if err != nil {
		s.renderError(w, "config_edit", "Config editor", "configs", err, http.StatusBadRequest, r, func(data *PageData) {
			data.CurrentFile = fileName
		})
		return
	}

	data := s.basePage("Edit "+fileName, "configs", r)
	data.CurrentFile = fileName
	data.Content = content
	s.render(w, "config_edit", data, http.StatusOK)
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderError(w, "config_edit", "Config editor", "configs", err, http.StatusBadRequest, r, nil)
		return
	}

	fileName, ok := cleanConfigName(name)
	if !ok {
		http.NotFound(w, r)
		return
	}

	backupPath, err := s.files.SaveConfig(fileName, r.FormValue("content"))
	if err != nil {
		s.renderError(w, "config_edit", "Config editor", "configs", err, http.StatusBadRequest, r, func(data *PageData) {
			data.CurrentFile = fileName
			data.Content = r.FormValue("content")
		})
		return
	}

	s.events.Set(eventFromSave(fileName, backupPath))
	http.Redirect(w, r, redirectTarget(r, "/configs/"+fileName), http.StatusSeeOther)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := s.commands.Validate()
	s.events.Set(eventFromResult(result))
	http.Redirect(w, r, redirectTarget(r, "/configs"), http.StatusSeeOther)
}

func (s *Server) handleMerged(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := s.commands.DumpMerged()
	if !result.Success {
		s.events.Set(eventFromResult(result))
	}

	data := s.basePage("Merged config", "merged", r)
	data.MergedConfig = joinOutput(result.Stdout, result.Stderr)
	if data.MergedConfig == "" {
		data.MergedConfig = "No output"
	}
	s.render(w, "merged", data, http.StatusOK)
}

func (s *Server) handleLogsIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/logs" {
		http.NotFound(w, r)
		return
	}

	kinds := s.logs.Kinds()
	if len(kinds) == 0 {
		s.renderError(w, "logs", "Logs", "logs", fmt.Errorf("no log files configured"), http.StatusInternalServerError, r, nil)
		return
	}

	http.Redirect(w, r, "/logs/"+kinds[0], http.StatusSeeOther)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	kind := strings.TrimPrefix(r.URL.Path, "/logs/")
	content, err := s.logs.Tail(kind)
	if err != nil {
		s.renderError(w, "logs", "Logs", "logs", err, http.StatusBadRequest, r, func(data *PageData) {
			data.LogKinds = s.logs.Kinds()
			data.CurrentLogKind = kind
		})
		return
	}

	data := s.basePage("Logs", "logs", r)
	data.LogKinds = s.logs.Kinds()
	data.CurrentLogKind = kind
	data.LogContent = content
	if data.LogContent == "" {
		data.LogContent = "No log content"
	}
	s.render(w, "logs", data, http.StatusOK)
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/actions/")
	result, err := s.commands.Run(name)
	if err != nil {
		s.renderError(w, "dashboard", "Dashboard", "dashboard", err, http.StatusNotFound, r, func(data *PageData) {
			data.ActionGroups = groupActions(s.commands.AllowedActions())
		})
		return
	}

	s.events.Set(eventFromResult(result))

	if strings.EqualFold(r.Header.Get("HX-Request"), "true") {
		w.Header().Set("HX-Trigger", `{"status-refresh":true}`)
		s.renderFragment(w, "event_panel", PageData{
			LastEvent: s.lastEventView(),
		}, http.StatusOK)
		return
	}

	http.Redirect(w, r, redirectTarget(r, "/"), http.StatusSeeOther)
}

func (s *Server) basePage(title, active string, r *http.Request) PageData {
	return PageData{
		Title:        title,
		AssetVersion: s.assetVersion,
		Active:       active,
		LastEvent:    s.lastEventView(),
		CurrentUser:  auth.UsernameFromContext(r.Context()),
	}
}

func (s *Server) lastEventView() *EventView {
	event := s.events.Get()
	if event == nil {
		return nil
	}

	return &EventView{
		Title:   event.Title,
		Summary: event.Summary,
		Output:  event.Output,
		At:      event.At.Local().Format(time.RFC3339),
		Success: event.Success,
	}
}

func (s *Server) renderLoginError(w http.ResponseWriter, message string) {
	data := PageData{
		Title:        "Login",
		AssetVersion: s.assetVersion,
		LoginError:   message,
	}
	s.render(w, "login", data, http.StatusUnauthorized)
}

func (s *Server) renderError(w http.ResponseWriter, templateName, title, active string, err error, status int, r *http.Request, enrich func(*PageData)) {
	s.logger.Printf("http error: %v", err)
	data := s.basePage(title, active, r)
	data.Error = err.Error()
	if enrich != nil {
		enrich(&data)
	}
	s.render(w, templateName, data, status)
}

func (s *Server) render(w http.ResponseWriter, templateName string, data PageData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, templateName, data); err != nil {
		s.logger.Printf("render template %s: %v", templateName, err)
	}
}

func (s *Server) renderFragment(w http.ResponseWriter, templateName string, data PageData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, templateName, data); err != nil {
		s.logger.Printf("render fragment %s: %v", templateName, err)
	}
}

func groupActions(specs []commands.ActionSpec) []ActionGroup {
	groups := map[string][]commands.ActionSpec{
		"xray":  {},
		"xkeen": {},
		"tests": {},
	}

	for _, spec := range specs {
		groups[spec.Group] = append(groups[spec.Group], spec)
	}

	order := []struct {
		Key  string
		Name string
	}{
		{Key: "xray", Name: "Xray control"},
		{Key: "xkeen", Name: "Xkeen tools"},
		{Key: "tests", Name: "Diagnostics"},
	}

	result := make([]ActionGroup, 0, len(order))
	for _, item := range order {
		actions := groups[item.Key]
		if len(actions) == 0 {
			continue
		}
		result = append(result, ActionGroup{Name: item.Name, Actions: actions})
	}

	return result
}

func redirectTarget(r *http.Request, fallback string) string {
	return safeRedirectTarget(r.FormValue("redirect"), fallback)
}

func safeRedirectTarget(target, fallback string) string {
	if target == "" {
		return fallback
	}

	parsed, err := url.Parse(target)
	if err != nil || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return fallback
	}

	return parsed.String()
}

func joinOutput(stdout, stderr string) string {
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n\nstderr:\n" + stderr
	}
}

func resultSummary(result commands.Result) string {
	status := "failed"
	if result.Success {
		status = "completed"
	}
	if result.TimedOut {
		status = "timed out"
	}

	return fmt.Sprintf("%s in %s (exit %d)", status, result.Duration.Round(time.Millisecond), result.ExitCode)
}

func formatSize(size int64) string {
	switch {
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(size)/(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func cleanConfigName(name string) (string, bool) {
	if name == "" || strings.Contains(name, "/") {
		return "", false
	}

	fileName := path.Base(name)
	if fileName != name {
		return "", false
	}

	return fileName, true
}
