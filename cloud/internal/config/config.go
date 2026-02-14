package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL   string
	ListenAddr    string
	SessionSecret string `json:"-"`
	SecureCookies bool
	BaseURL       string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string `json:"-"`
	SMTPFrom     string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required (minimum 32 characters)")
	}
	if len(sessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8090"
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8090"
	}

	secureCookies := os.Getenv("SECURE_COOKIES") == "true"

	smtpPort := 587
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			smtpPort = n
		}
	}

	return &Config{
		DatabaseURL:   dbURL,
		ListenAddr:    listenAddr,
		SessionSecret: sessionSecret,
		SecureCookies: secureCookies,
		BaseURL:       baseURL,
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      smtpPort,
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:      os.Getenv("SMTP_FROM"),
	}, nil
}
