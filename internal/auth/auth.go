package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultIterations = 120000
	sessionCookieName = "xkeen_ui_session"
)

type contextKey string

const usernameContextKey contextKey = "username"

type NetworkAuthorizer struct {
	allowed []*net.IPNet
}

type SessionManager struct {
	cookieName string
	secret     []byte
	ttl        time.Duration
}

func NewNetworkAuthorizer(cidrs []string) (*NetworkAuthorizer, error) {
	allowed := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return nil, fmt.Errorf("parse cidr %q: %w", cidr, err)
		}
		allowed = append(allowed, network)
	}

	return &NetworkAuthorizer{allowed: allowed}, nil
}

func NewSessionManager(seed string, ttl time.Duration) (*SessionManager, error) {
	if strings.TrimSpace(seed) == "" {
		return nil, errors.New("session seed is required")
	}
	if ttl <= 0 {
		return nil, errors.New("session ttl must be positive")
	}

	secret := sha256.Sum256([]byte(seed))
	return &SessionManager{
		cookieName: sessionCookieName,
		secret:     append([]byte(nil), secret[:]...),
		ttl:        ttl,
	}, nil
}

func (a *NetworkAuthorizer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := remoteIP(r.RemoteAddr)
		if ip == nil || !a.isAllowed(ip) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *SessionManager) RequireAuth(loginPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, ok := s.Username(r)
			if !ok {
				if strings.EqualFold(r.Header.Get("HX-Request"), "true") {
					w.Header().Set("HX-Redirect", loginPath)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}

				http.Redirect(w, r, loginPath, http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), usernameContextKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (s *SessionManager) Set(w http.ResponseWriter, username string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("username is required")
	}

	expiresAt := time.Now().UTC().Add(s.ttl)
	payload := username + "\n" + strconv.FormatInt(expiresAt.Unix(), 10)
	signature := s.sign(payload)
	value := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)

	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(s.ttl.Seconds()),
	})

	return nil
}

func (s *SessionManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	})
}

func (s *SessionManager) Username(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(s.cookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return "", false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}

	payload := string(payloadBytes)
	if !hmac.Equal(signature, s.sign(payload)) {
		return "", false
	}

	fields := strings.Split(payload, "\n")
	if len(fields) != 2 {
		return "", false
	}

	expiresAtUnix, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return "", false
	}

	if time.Now().UTC().Unix() > expiresAtUnix {
		return "", false
	}

	username := strings.TrimSpace(fields[0])
	if username == "" {
		return "", false
	}

	return username, true
}

func UsernameFromContext(ctx context.Context) string {
	value, _ := ctx.Value(usernameContextKey).(string)
	return value
}

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	digest := derive(password, salt, defaultIterations)
	return fmt.Sprintf("sha256$%d$%s$%s", defaultIterations, hex.EncodeToString(salt), hex.EncodeToString(digest)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}

	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}

	expected, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	actual := derive(password, salt, iterations)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func derive(password string, salt []byte, iterations int) []byte {
	passwordBytes := []byte(password)
	input := make([]byte, 0, len(salt)+len(passwordBytes)+sha256.Size)
	input = append(input, salt...)
	input = append(input, passwordBytes...)
	sum := sha256.Sum256(input)

	for i := 1; i < iterations; i++ {
		input = input[:0]
		input = append(input, sum[:]...)
		input = append(input, salt...)
		input = append(input, passwordBytes...)
		sum = sha256.Sum256(input)
	}

	out := make([]byte, sha256.Size)
	copy(out, sum[:])
	return out
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	return net.ParseIP(strings.TrimSpace(host))
}

func (a *NetworkAuthorizer) isAllowed(ip net.IP) bool {
	for _, network := range a.allowed {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func (s *SessionManager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
