package crypto

import (
	"encoding/base64"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c := NewCodec()
	line, err := c.Encrypt("hunter2", "hello world")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := c.Decrypt("hunter2", line)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if pt != "hello world" {
		t.Fatalf("got %q, want %q", pt, "hello world")
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	c := NewCodec()
	line, err := c.Encrypt("correct-password", "secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := c.Decrypt("wrong-password", line); err != ErrDecryptFailed {
		t.Fatalf("got err=%v, want ErrDecryptFailed", err)
	}
}

func TestDecryptMalformedLine(t *testing.T) {
	c := NewCodec()
	cases := []string{"", "not-base64!!!", base64.StdEncoding.EncodeToString([]byte("short"))}
	for _, line := range cases {
		if _, err := c.Decrypt("any", line); err != ErrDecryptFailed {
			t.Errorf("line %q: got err=%v, want ErrDecryptFailed", line, err)
		}
	}
}

func TestSaltReusedWithinSessionSamePassword(t *testing.T) {
	c := NewCodec()
	line1, _ := c.Encrypt("pw", "first")
	line2, _ := c.Encrypt("pw", "second")

	salt1 := decodeSalt(t, line1)
	salt2 := decodeSalt(t, line2)
	if string(salt1) != string(salt2) {
		t.Fatalf("salts differ across blocks with the same password in one run: %x vs %x", salt1, salt2)
	}

	line3, _ := c.Encrypt("other-pw", "third")
	salt3 := decodeSalt(t, line3)
	if string(salt3) == string(salt1) {
		t.Fatalf("different passwords should not share a salt")
	}
}

func TestNonceUniquePerBlock(t *testing.T) {
	c := NewCodec()
	line1, _ := c.Encrypt("pw", "same text")
	line2, _ := c.Encrypt("pw", "same text")
	if line1 == line2 {
		t.Fatalf("two blocks with identical plaintext+password must not produce identical ciphertext (nonce reuse)")
	}
}

func decodeSalt(t *testing.T, line string) []byte {
	t.Helper()
	payload, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload) < saltLen {
		t.Fatalf("payload too short")
	}
	return payload[:saltLen]
}
