package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"
)

type EmailSender struct {
	host     string
	port     int
	user     string
	password string
	from     string
	baseURL  string
}

func NewEmailSender(host string, port int, user, password, from, baseURL string) *EmailSender {
	return &EmailSender{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
		baseURL:  baseURL,
	}
}

func (e *EmailSender) SendVerificationEmail(to, token string) error {
	verifyURL := fmt.Sprintf("%s/verify?token=%s", e.baseURL, token)

	subject := "Verify your Replayer Cloud account"
	body := fmt.Sprintf(`Hello,

Please verify your email address by clicking the link below:

%s

If you didn't create an account, you can safely ignore this email.

Thanks,
Replayer Cloud`, verifyURL)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, to, subject, body)

	auth := smtp.PlainAuth("", e.user, e.password, e.host)
	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	return smtp.SendMail(addr, auth, e.from, []string{to}, []byte(msg))
}

func (e *EmailSender) IsConfigured() bool {
	return e.host != "" && e.from != ""
}

func GenerateVerifyToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
