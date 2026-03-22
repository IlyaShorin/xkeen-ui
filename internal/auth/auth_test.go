package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestNetworkAuthorizerAllowsCIDR(t *testing.T) {
	t.Parallel()

	authorizer, err := NewNetworkAuthorizer([]string{"192.168.0.0/16"})
	if err != nil {
		t.Fatalf("new network authorizer: %v", err)
	}

	handler := authorizer.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.168.1.10:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestNetworkAuthorizerDeniesCIDR(t *testing.T) {
	t.Parallel()

	authorizer, err := NewNetworkAuthorizer([]string{"192.168.0.0/16"})
	if err != nil {
		t.Fatalf("new network authorizer: %v", err)
	}

	handler := authorizer.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "8.8.8.8:53"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestSessionManagerSetAndUsername(t *testing.T) {
	t.Parallel()

	manager, err := NewSessionManager("seed", time.Hour)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	response := httptest.NewRecorder()
	if err := manager.Set(response, "admin"); err != nil {
		t.Fatalf("set session: %v", err)
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookies[0])

	username, ok := manager.Username(request)
	if !ok || username != "admin" {
		t.Fatalf("unexpected session username: %q, %t", username, ok)
	}
}

func TestSessionManagerRejectsExpired(t *testing.T) {
	t.Parallel()

	manager, err := NewSessionManager("seed", time.Hour)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	payload := "admin\n" + strconv.FormatInt(time.Now().UTC().Add(-time.Hour).Unix(), 10)
	signature := manager.sign(payload)
	value := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})

	if _, ok := manager.Username(request); ok {
		t.Fatal("expected expired session")
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if !VerifyPassword("secret", hash) {
		t.Fatal("expected password verification to pass")
	}

	if VerifyPassword("nope", hash) {
		t.Fatal("expected password verification to fail")
	}
}
