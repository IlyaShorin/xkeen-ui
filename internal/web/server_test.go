package web

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xkeen-ui/internal/auth"
	"xkeen-ui/internal/commands"
	"xkeen-ui/internal/config"
	"xkeen-ui/internal/files"
	"xkeen-ui/internal/logview"
)

func TestLoginAndDashboard(t *testing.T) {
	t.Parallel()

	handler, _ := testHandler(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected dashboard status: %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), "Keenetic companion") {
		t.Fatalf("unexpected dashboard body: %s", response.Body.String())
	}
}

func TestUnauthorizedRedirectsToLogin(t *testing.T) {
	t.Parallel()

	handler, _ := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: %d", response.Code)
	}

	if response.Header().Get("Location") != "/login" {
		t.Fatalf("unexpected location: %s", response.Header().Get("Location"))
	}
}

func TestSaveValidateFailAndManualRestart(t *testing.T) {
	handler, statusPath := testHandler(t)
	cookie := loginCookie(t, handler)

	saveRequest := httptest.NewRequest(http.MethodPost, "/configs/08_outbounds.json/save", strings.NewReader("content=FAIL&redirect=/configs/08_outbounds.json"))
	saveRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saveRequest.RemoteAddr = "127.0.0.1:12345"
	saveRequest.AddCookie(cookie)
	saveResponse := httptest.NewRecorder()
	handler.ServeHTTP(saveResponse, saveRequest)

	if saveResponse.Code != http.StatusSeeOther {
		t.Fatalf("unexpected save status: %d", saveResponse.Code)
	}

	validateRequest := httptest.NewRequest(http.MethodPost, "/configs/validate", strings.NewReader("redirect=/configs/08_outbounds.json"))
	validateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validateRequest.RemoteAddr = "127.0.0.1:12345"
	validateRequest.AddCookie(cookie)
	validateResponse := httptest.NewRecorder()
	handler.ServeHTTP(validateResponse, validateRequest)

	if validateResponse.Code != http.StatusSeeOther {
		t.Fatalf("unexpected validate status: %d", validateResponse.Code)
	}

	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Fatalf("xray status file should not exist after validate only")
	}

	restartRequest := httptest.NewRequest(http.MethodPost, "/actions/xray-restart", strings.NewReader("redirect=/"))
	restartRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	restartRequest.RemoteAddr = "127.0.0.1:12345"
	restartRequest.AddCookie(cookie)
	restartResponse := httptest.NewRecorder()
	handler.ServeHTTP(restartResponse, restartRequest)

	if restartResponse.Code != http.StatusSeeOther {
		t.Fatalf("unexpected restart status: %d", restartResponse.Code)
	}

	statusContent, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}

	if strings.TrimSpace(string(statusContent)) != "running" {
		t.Fatalf("unexpected status file content: %s", string(statusContent))
	}
}

func TestMergedPreview(t *testing.T) {
	t.Parallel()

	handler, _ := testHandler(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/configs/merged", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected merged status: %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), "merged") {
		t.Fatalf("unexpected merged body: %s", response.Body.String())
	}
}

func TestValidateSuccessThenRestart(t *testing.T) {
	t.Parallel()

	handler, statusPath := testHandler(t)
	cookie := loginCookie(t, handler)

	saveRequest := httptest.NewRequest(http.MethodPost, "/configs/08_outbounds.json/save", strings.NewReader("content=%7B%7D&redirect=/configs/08_outbounds.json"))
	saveRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saveRequest.RemoteAddr = "127.0.0.1:12345"
	saveRequest.AddCookie(cookie)
	saveResponse := httptest.NewRecorder()
	handler.ServeHTTP(saveResponse, saveRequest)

	if saveResponse.Code != http.StatusSeeOther {
		t.Fatalf("unexpected save status: %d", saveResponse.Code)
	}

	validateRequest := httptest.NewRequest(http.MethodPost, "/configs/validate", strings.NewReader("redirect=/configs/08_outbounds.json"))
	validateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validateRequest.RemoteAddr = "127.0.0.1:12345"
	validateRequest.AddCookie(cookie)
	validateResponse := httptest.NewRecorder()
	handler.ServeHTTP(validateResponse, validateRequest)

	if validateResponse.Code != http.StatusSeeOther {
		t.Fatalf("unexpected validate status: %d", validateResponse.Code)
	}

	restartRequest := httptest.NewRequest(http.MethodPost, "/actions/xray-restart", strings.NewReader("redirect=/"))
	restartRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	restartRequest.RemoteAddr = "127.0.0.1:12345"
	restartRequest.AddCookie(cookie)
	restartResponse := httptest.NewRecorder()
	handler.ServeHTTP(restartResponse, restartRequest)

	if restartResponse.Code != http.StatusSeeOther {
		t.Fatalf("unexpected restart status: %d", restartResponse.Code)
	}

	statusContent, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}

	if strings.TrimSpace(string(statusContent)) != "running" {
		t.Fatalf("unexpected status file content: %s", string(statusContent))
	}
}

func TestStatusFragment(t *testing.T) {
	t.Parallel()

	handler, _ := testHandler(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/fragments/status", nil)
	request.Header.Set("HX-Request", "true")
	request.RemoteAddr = "127.0.0.1:12345"
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected fragment status: %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), "status-panel") {
		t.Fatalf("unexpected fragment body: %s", response.Body.String())
	}
}

func TestActionHtmxRefreshesStatus(t *testing.T) {
	t.Parallel()

	handler, _ := testHandler(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodPost, "/actions/xray-restart", strings.NewReader(""))
	request.Header.Set("HX-Request", "true")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = "127.0.0.1:12345"
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected HTMX action status: %d", response.Code)
	}

	if !strings.Contains(response.Header().Get("HX-Trigger"), "status-refresh") {
		t.Fatalf("missing HX-Trigger header: %#v", response.Header())
	}

	if !strings.Contains(response.Body.String(), "Restart xray") {
		t.Fatalf("unexpected HTMX action body: %s", response.Body.String())
	}
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()

	form := url.Values{
		"username": []string{"admin"},
		"password": []string{"secret"},
		"next":     []string{"/"},
	}

	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("unexpected login status: %d", response.Code)
	}

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "xkeen_ui_session" {
			return cookie
		}
	}

	t.Fatal("session cookie not found")
	return nil
}

func testHandler(t *testing.T) (http.Handler, string) {
	t.Helper()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "configs")
	backupDir := filepath.Join(dir, "backups")
	statusPath := filepath.Join(dir, "status")
	xkeenPath := filepath.Join(dir, "xkeen")
	xrayPath := filepath.Join(dir, "xray")
	servicePath := filepath.Join(dir, "S24xray")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "08_outbounds.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := os.WriteFile(xkeenPath, []byte("#!/bin/sh\ncase \"$1\" in\n  -cb) echo 'backup ok' ;;\n  -cbr) echo 'restore ok' ;;\n  -tc) echo 'connection ok' ;;\n  -tpx) echo 'ports ok' ;;\n  -tfx) echo 'xray files ok' ;;\n  -tfk) echo 'xkeen files ok' ;;\n  -v) echo 'xkeen version 1.0' ;;\n  *) echo 'unknown' >&2; exit 1 ;;\nesac\n"), 0o755); err != nil {
		t.Fatalf("write xkeen: %v", err)
	}

	if err := os.WriteFile(xrayPath, []byte("#!/bin/sh\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"-test\" ]; then\n  if grep -q FAIL \"$4\"/08_outbounds.json; then\n    echo 'invalid config' >&2\n    exit 1\n  fi\n  echo 'config ok'\n  exit 0\nfi\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"-dump\" ]; then\n  echo '{\"merged\": true}'\n  exit 0\nfi\necho 'bad args' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write xray: %v", err)
	}

	if err := os.WriteFile(servicePath, []byte("#!/bin/sh\nSTATUS_FILE=\""+statusPath+"\"\ncase \"$1\" in\n  start) echo 'running' > \"$STATUS_FILE\"; echo 'running'; exit 0 ;;\n  restart) echo 'running' > \"$STATUS_FILE\"; echo 'running'; exit 0 ;;\n  stop) echo 'stopped' > \"$STATUS_FILE\"; echo 'stopped'; exit 0 ;;\n  status)\n    if [ -f \"$STATUS_FILE\" ] && grep -q running \"$STATUS_FILE\"; then\n      echo 'running'\n      exit 0\n    fi\n    echo 'stopped'\n    exit 1\n    ;;\n  *) exit 1 ;;\nesac\n"), 0o755); err != nil {
		t.Fatalf("write xray service: %v", err)
	}

	hash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := config.Config{
		Listen:        "127.0.0.1:9081",
		Username:      "admin",
		PasswordHash:  hash,
		AllowCIDRs:    []string{"127.0.0.1/32"},
		XKeenBin:      xkeenPath,
		XrayBin:       xrayPath,
		XrayService:   servicePath,
		XrayConfigDir: configDir,
		BackupDir:     backupDir,
		LogFiles: map[string]string{
			"xray_access": filepath.Join(dir, "xray-access.log"),
			"xray_error":  filepath.Join(dir, "xray-error.log"),
			"xkeen_info":  filepath.Join(dir, "xkeen-info.log"),
			"xkeen_error": filepath.Join(dir, "xkeen-error.log"),
		},
	}

	fileService := files.NewService(configDir, backupDir)
	commandService := commands.NewService(cfg, time.Second, log.New(ioDiscard{}, "", 0))
	logService := logview.NewService(cfg.LogFiles, 50, 64*1024)
	sessionManager, err := auth.NewSessionManager(cfg.Username+"\n"+cfg.PasswordHash, 12*time.Hour)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	server, err := NewServer(cfg, sessionManager, fileService, commandService, logService, log.New(ioDiscard{}, "", 0))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	authz, err := auth.NewNetworkAuthorizer([]string{"127.0.0.1/32"})
	if err != nil {
		t.Fatalf("new network authorizer: %v", err)
	}

	return authz.Middleware(server.Handler()), statusPath
}

type ioDiscard struct{}

func (ioDiscard) Write(data []byte) (int, error) {
	return len(data), nil
}
