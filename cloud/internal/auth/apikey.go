package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	apiKeyLength = 32
	prefixLength = 8
)

func GenerateAPIKey() (fullKey, keyHash, keyPrefix string, err error) {
	bytes := make([]byte, apiKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", fmt.Errorf("generating random bytes: %w", err)
	}

	fullKey = "rp_" + hex.EncodeToString(bytes)
	keyHash = HashAPIKey(fullKey)
	keyPrefix = fullKey[:prefixLength+3]

	return fullKey, keyHash, keyPrefix, nil
}

func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
