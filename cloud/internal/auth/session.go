package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/kx0101/replayer-cloud/internal/models"
)

const (
	sessionCookieName = "session"
	sessionDuration   = 7 * 24 * time.Hour
)

var (
	ErrInvalidSession = errors.New("invalid session")
	ErrExpiredSession = errors.New("session expired")
)

type SessionManager struct {
	key          []byte
	secureCookie bool
}

func NewSessionManager(secret string, secureCookie bool) (*SessionManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("session secret must be at least 32 characters")
	}
	return &SessionManager{
		key:          []byte(secret)[:32],
		secureCookie: secureCookie,
	}, nil
}

func (sm *SessionManager) CreateSession(w http.ResponseWriter, user *models.User) error {
	session := models.SessionData{
		UserID:    user.ID,
		Email:     user.Email,
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	encrypted, err := sm.encrypt(data)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encrypted,
		Path:     "/",
		HttpOnly: true,
		Secure:   sm.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	return nil
}

func (sm *SessionManager) GetSession(r *http.Request) (*models.SessionData, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, ErrInvalidSession
	}

	data, err := sm.decrypt(cookie.Value)
	if err != nil {
		return nil, ErrInvalidSession
	}

	var session models.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, ErrInvalidSession
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrExpiredSession
	}

	return &session, nil
}

func (sm *SessionManager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   sm.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (sm *SessionManager) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(sm.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (sm *SessionManager) decrypt(encoded string) ([]byte, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(sm.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, nil)
}
