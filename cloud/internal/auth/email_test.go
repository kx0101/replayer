package auth

import (
	"strings"
	"testing"
)

func TestNewEmailSender(t *testing.T) {
	sender := NewEmailSender(
		"smtp.example.com",
		587,
		"user@example.com",
		"password",
		"noreply@example.com",
		"https://replayer.example.com",
	)

	if sender == nil {
		t.Error("expected sender, got nil")
	}

	if sender != nil && sender.host != "smtp.example.com" {
		t.Errorf("expected host smtp.example.com, got %s", sender.host)
	}

	if sender.port != 587 {
		t.Errorf("expected port 587, got %d", sender.port)
	}
}

func TestEmailSender_IsConfigured(t *testing.T) {
	tests := []struct {
		name string
		host string
		from string
		want bool
	}{
		{
			name: "fully configured",
			host: "smtp.example.com",
			from: "noreply@example.com",
			want: true,
		},
		{
			name: "missing host",
			host: "",
			from: "noreply@example.com",
			want: false,
		},
		{
			name: "missing from",
			host: "smtp.example.com",
			from: "",
			want: false,
		},
		{
			name: "both missing",
			host: "",
			from: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := NewEmailSender(tt.host, 587, "", "", tt.from, "")
			got := sender.IsConfigured()
			if got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateVerifyToken(t *testing.T) {
	token1, err := GenerateVerifyToken()
	if err != nil {
		t.Fatalf("GenerateVerifyToken failed: %v", err)
	}

	if len(token1) != 64 {
		t.Errorf("token length should be 64, got %d", len(token1))
	}

	for _, c := range token1 {
		if ((c < '0' || c > '9') && (c < 'a' || c > 'f')) {
			t.Errorf("token contains non-hex character: %c", c)
		}
	}

	token2, _ := GenerateVerifyToken()
	if token1 == token2 {
		t.Error("tokens should be unique")
	}
}

func TestGenerateVerifyToken_Unique(t *testing.T) {
	tokens := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, err := GenerateVerifyToken()
		if err != nil {
			t.Fatalf("GenerateVerifyToken failed: %v", err)
		}

		if tokens[token] {
			t.Errorf("duplicate token generated: %s", token)
		}

		tokens[token] = true
	}
}

func TestEmailMessageFormat(t *testing.T) {
	sender := NewEmailSender(
		"smtp.example.com",
		587,
		"user@example.com",
		"password",
		"noreply@example.com",
		"https://replayer.example.com",
	)

	token := "abc123"
	expectedURL := "https://replayer.example.com/verify?token=abc123"

	if !strings.Contains(sender.baseURL+"/verify?token="+token, expectedURL) {
		t.Error("verify URL not constructed correctly")
	}
}
