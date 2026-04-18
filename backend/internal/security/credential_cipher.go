package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const encryptedPrefix = "enc:v1:"

// CredentialCipher encrypts and decrypts sensitive credential fields.
type CredentialCipher struct {
	aead cipher.AEAD
}

// NewCredentialCipherFromBase64 creates a cipher from a base64-encoded 32-byte key.
func NewCredentialCipherFromBase64(encodedKey string) (*CredentialCipher, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil {
		return nil, fmt.Errorf("decode credentials encryption key: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("credentials encryption key must decode to 32 bytes, got %d", len(decoded))
	}

	block, err := aes.NewCipher(decoded)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &CredentialCipher{aead: aead}, nil
}

// IsEncrypted reports whether the value matches the encrypted value prefix.
func (c *CredentialCipher) IsEncrypted(v string) bool {
	return strings.HasPrefix(v, encryptedPrefix)
}

// Encrypt returns an encrypted representation of plain text.
func (c *CredentialCipher) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if c.IsEncrypted(plain) {
		return plain, nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := c.aead.Seal(nil, nonce, []byte(plain), nil)
	combined := append(nonce, sealed...)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt returns plain text from an encrypted value.
func (c *CredentialCipher) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !c.IsEncrypted(value) {
		return value, nil
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(data) < c.aead.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := data[:c.aead.NonceSize()]
	ciphertext := data[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plain), nil
}
