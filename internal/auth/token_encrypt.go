package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// TokenEncryptor encrypts and decrypts OAuth tokens at rest using AES-GCM.
// If key is nil, operates as a no-op passthrough (for local dev).
type TokenEncryptor struct {
	gcm cipher.AEAD
}

// NewTokenEncryptor creates an encryptor from a 32-byte AES-256 key.
// Pass nil for a no-op encryptor (tokens stored as plaintext).
func NewTokenEncryptor(key []byte) (*TokenEncryptor, error) {
	if len(key) == 0 {
		return &TokenEncryptor{}, nil
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("token encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return &TokenEncryptor{gcm: gcm}, nil
}

// Encrypt encrypts a plaintext token. Returns base64-encoded ciphertext.
func (e *TokenEncryptor) Encrypt(plaintext string) (string, error) {
	if e.gcm == nil {
		return plaintext, nil
	}
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext back to plaintext.
func (e *TokenEncryptor) Decrypt(encoded string) (string, error) {
	if e.gcm == nil {
		return encoded, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Likely a plaintext token from before encryption was enabled.
		return encoded, nil
	}
	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		// Too short to be encrypted — return as-is (backward compat).
		return encoded, nil
	}
	plaintext, err := e.gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		// Decryption failed — likely a plaintext token. Return as-is.
		return encoded, nil
	}
	return string(plaintext), nil
}
