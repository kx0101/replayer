package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	fullKey, keyHash, keyPrefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if !strings.HasPrefix(fullKey, "rp_") {
		t.Errorf("fullKey should start with 'rp_', got %s", fullKey)
	}

	expectedPrefixLen := 3 + prefixLength
	if len(keyPrefix) != expectedPrefixLen {
		t.Errorf("keyPrefix length should be %d, got %d", expectedPrefixLen, len(keyPrefix))
	}

	if !strings.HasPrefix(fullKey, keyPrefix) {
		t.Errorf("fullKey should start with keyPrefix")
	}

	if keyHash == "" {
		t.Error("keyHash should not be empty")
	}

	if keyHash == fullKey {
		t.Error("keyHash should not equal fullKey")
	}

	if HashAPIKey(fullKey) != keyHash {
		t.Error("HashAPIKey(fullKey) should equal keyHash")
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	keys := make(map[string]bool)

	for range 100 {
		fullKey, _, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		if keys[fullKey] {
			t.Errorf("duplicate key generated: %s", fullKey)
		}
		keys[fullKey] = true
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "rp_testkey1234567890abcdef"
	hash1 := HashAPIKey(key)
	hash2 := HashAPIKey(key)

	if hash1 != hash2 {
		t.Error("same key should produce same hash")
	}

	otherKey := "rp_otherkey1234567890abcdef"
	otherHash := HashAPIKey(otherKey)

	if hash1 == otherHash {
		t.Error("different keys should produce different hashes")
	}

	if len(hash1) != 64 {
		t.Errorf("hash length should be 64, got %d", len(hash1))
	}

	for _, c := range hash1 {
		if ((c < '0' || c > '9') && (c < 'a' || c > 'f')) {
			t.Errorf("hash contains non-hex character: %c", c)
		}
	}
}

func TestAPIKeyFormat(t *testing.T) {
	fullKey, _, _, _ := GenerateAPIKey()

	expectedLen := 3 + (apiKeyLength * 2)
	if len(fullKey) != expectedLen {
		t.Errorf("fullKey length should be %d, got %d", expectedLen, len(fullKey))
	}

	hexPart := fullKey[3:]
	for _, c := range hexPart {
		if ((c < '0' || c > '9') && (c < 'a' || c > 'f')) {
			t.Errorf("key contains non-hex character: %c", c)
		}
	}
}
