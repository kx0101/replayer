package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/models"
)

func TestNewSessionManager(t *testing.T) {
	tests := []struct {
		name         string
		secret       string
		secureCookie bool
		wantErr      bool
	}{
		{
			name:         "valid secret",
			secret:       "12345678901234567890123456789012",
			secureCookie: false,
			wantErr:      false,
		},
		{
			name:         "secret too short",
			secret:       "tooshort",
			secureCookie: false,
			wantErr:      true,
		},
		{
			name:         "long secret truncated",
			secret:       "12345678901234567890123456789012extra",
			secureCookie: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, err := NewSessionManager(tt.secret, tt.secureCookie)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSessionManager() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && sm == nil {
				t.Error("expected session manager, got nil")
			}
		})
	}
}

func TestSessionManager_CreateAndGetSession(t *testing.T) {
	sm, err := NewSessionManager("12345678901234567890123456789012", false)
	if err != nil {
		t.Fatalf("NewSessionManager failed: %v", err)
	}

	user := &models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
	}

	rec := httptest.NewRecorder()
	err = sm.CreateSession(rec, user)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie to be set")
	}

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("session cookie not found")
	}

	if !sessionCookie.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sessionCookie)

	session, err := sm.GetSession(req)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if session.UserID != user.ID {
		t.Errorf("got UserID %v, want %v", session.UserID, user.ID)
	}

	if session.Email != user.Email {
		t.Errorf("got Email %v, want %v", session.Email, user.Email)
	}
}

func TestSessionManager_GetSession_NoCookie(t *testing.T) {
	sm, err := NewSessionManager("12345678901234567890123456789012", false)
	if err != nil {
		t.Fatalf("NewSessionManager failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err = sm.GetSession(req)

	if err != ErrInvalidSession {
		t.Errorf("expected ErrInvalidSession, got %v", err)
	}
}

func TestSessionManager_GetSession_InvalidCookie(t *testing.T) {
	sm, err := NewSessionManager("12345678901234567890123456789012", false)
	if err != nil {
		t.Fatalf("NewSessionManager failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: "invalid-session-value",
	})

	_, err = sm.GetSession(req)
	if err != ErrInvalidSession {
		t.Errorf("expected ErrInvalidSession, got %v", err)
	}
}

func TestSessionManager_GetSession_DifferentKey(t *testing.T) {
	sm1, _ := NewSessionManager("12345678901234567890123456789012", false)
	sm2, _ := NewSessionManager("differentkey90123456789012345678", false)

	user := &models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
	}

	rec := httptest.NewRecorder()
	_ = sm1.CreateSession(rec, user)

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sessionCookie)

	_, err := sm2.GetSession(req)
	if err != ErrInvalidSession {
		t.Errorf("expected ErrInvalidSession with different key, got %v", err)
	}
}

func TestSessionManager_ClearSession(t *testing.T) {
	sm, err := NewSessionManager("12345678901234567890123456789012", false)
	if err != nil {
		t.Fatalf("NewSessionManager failed: %v", err)
	}

	rec := httptest.NewRecorder()
	sm.ClearSession(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie to be set")
	}

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("session cookie not found")
	}

	if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", sessionCookie.MaxAge)
	}
}

func TestSessionManager_SecureCookie(t *testing.T) {
	sm, _ := NewSessionManager("12345678901234567890123456789012", true)

	user := &models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
	}

	rec := httptest.NewRecorder()
	_ = sm.CreateSession(rec, user)

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if !sessionCookie.Secure {
		t.Error("expected Secure flag to be true")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	sm, _ := NewSessionManager("12345678901234567890123456789012", false)

	plaintext := []byte("test data")
	encrypted, err := sm.encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	decrypted, err := sm.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("got %s, want %s", decrypted, plaintext)
	}
}

func TestSessionExpiry(t *testing.T) {
	sm, _ := NewSessionManager("12345678901234567890123456789012", false)

	user := &models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
	}

	rec := httptest.NewRecorder()
	_ = sm.CreateSession(rec, user)

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sessionCookie)

	session, _ := sm.GetSession(req)

	expectedExpiry := time.Now().Add(7 * 24 * time.Hour)
	tolerance := time.Minute

	diff := session.ExpiresAt.Sub(expectedExpiry)
	if diff > tolerance || diff < -tolerance {
		t.Errorf("session expiry %v not within tolerance of expected %v", session.ExpiresAt, expectedExpiry)
	}
}
