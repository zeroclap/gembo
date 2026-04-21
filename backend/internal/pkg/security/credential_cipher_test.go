package security

import (
	"encoding/base64"
	"testing"
)

func TestCredentialCipher_EncryptDecrypt(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	c, err := NewCredentialCipherFromBase64(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	enc, err := c.Encrypt("secret-value")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "secret-value" {
		t.Fatal("encrypted value should differ from plaintext")
	}

	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != "secret-value" {
		t.Fatalf("expected decrypted plaintext to match, got %q", dec)
	}
}

func TestCredentialCipher_DeterministicPrefixAndPassthrough(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	c, err := NewCredentialCipherFromBase64(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	plain := "already-plain"
	dec, err := c.Decrypt(plain)
	if err != nil {
		t.Fatalf("decrypt plain: %v", err)
	}
	if dec != plain {
		t.Fatalf("expected plain passthrough, got %q", dec)
	}

	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !c.IsEncrypted(enc) {
		t.Fatalf("expected encrypted prefix on value: %q", enc)
	}
}

func TestCredentialCipher_InvalidKeyLength(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("short-key"))
	if _, err := NewCredentialCipherFromBase64(key); err == nil {
		t.Fatal("expected key length error")
	}
}
