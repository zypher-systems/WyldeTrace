package apitoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const PrefixTag = "wt"

func Generate() (plaintext string, prefix string, hash string, err error) {
	prefixBytes := make([]byte, 4)
	secretBytes := make([]byte, 24)
	if _, err = rand.Read(prefixBytes); err != nil {
		return "", "", "", err
	}
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", "", err
	}
	prefix = hex.EncodeToString(prefixBytes)
	secret := hex.EncodeToString(secretBytes)
	plaintext = fmt.Sprintf("%s_%s_%s", PrefixTag, prefix, secret)
	sum := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(sum[:])
	return plaintext, prefix, hash, nil
}

func Parse(raw string) (prefix string, ok bool) {
	parts := strings.Split(raw, "_")
	if len(parts) != 3 || parts[0] != PrefixTag || len(parts[1]) < 4 {
		return "", false
	}
	return parts[1], true
}

func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
