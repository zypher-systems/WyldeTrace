package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const prefix = "wtenc:v1:"

// Box envelope-encrypts secrets with AES-256-GCM.
type Box struct {
	gcm cipher.AEAD
}

// ParseKey accepts 64 hex chars (32 bytes) or raw 32-byte / 44-char base64.
func ParseKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty key")
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, fmt.Errorf("SETTINGS_ENCRYPTION_KEY must be 32 bytes (64 hex chars or base64)")
}

func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, prefix)
}

func (b *Box) Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(out), nil
}

func (b *Box) Open(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil // legacy plaintext
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}

// SealRequired encrypts when box is non-nil; when box is nil returns plaintext (lab) or error if requireKey.
func SealRequired(b *Box, plaintext string, allowPlain bool) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if b == nil {
		if allowPlain {
			return plaintext, nil
		}
		return "", errors.New("SETTINGS_ENCRYPTION_KEY is required to store secrets")
	}
	return b.Seal(plaintext)
}

func OpenOptional(b *Box, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !IsEncrypted(value) {
		return value, nil
	}
	if b == nil {
		return "", errors.New("encrypted secret present but SETTINGS_ENCRYPTION_KEY is not configured")
	}
	return b.Open(value)
}
