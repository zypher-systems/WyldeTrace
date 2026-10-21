package secretbox

import (
	"encoding/hex"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	b, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := b.Seal("super-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(sealed) {
		t.Fatalf("expected prefix: %s", sealed)
	}
	plain, err := b.Open(sealed)
	if err != nil || plain != "super-secret" {
		t.Fatalf("got %q err=%v", plain, err)
	}
	// Idempotent seal
	again, err := b.Seal(sealed)
	if err != nil || again != sealed {
		t.Fatalf("re-seal changed value")
	}
}

func TestParseKey(t *testing.T) {
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	k, err := ParseKey(hexKey)
	if err != nil || len(k) != 32 {
		t.Fatalf("hex: %v len=%d", err, len(k))
	}
}
