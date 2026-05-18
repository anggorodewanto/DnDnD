package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenEncryptor_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := NewTokenEncryptor(key)
	require.NoError(t, err)

	token := "ya29.a0AfH6SMBx-test-access-token"
	ciphertext, err := enc.Encrypt(token)
	require.NoError(t, err)
	assert.NotEqual(t, token, ciphertext, "ciphertext should differ from plaintext")

	decrypted, err := enc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, token, decrypted)
}

func TestTokenEncryptor_NilKey_Passthrough(t *testing.T) {
	enc, err := NewTokenEncryptor(nil)
	require.NoError(t, err)

	token := "plaintext-token"
	encrypted, err := enc.Encrypt(token)
	require.NoError(t, err)
	assert.Equal(t, token, encrypted, "nil-key encryptor should passthrough")

	decrypted, err := enc.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, token, decrypted)
}

func TestTokenEncryptor_DecryptPlaintext_BackwardCompat(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	enc, err := NewTokenEncryptor(key)
	require.NoError(t, err)

	// A plaintext token (not base64-encoded ciphertext) should be returned as-is
	plaintext := "old-unencrypted-token"
	decrypted, err := enc.Decrypt(plaintext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted, "should gracefully handle pre-encryption tokens")
}

func TestTokenEncryptor_InvalidKeyLength(t *testing.T) {
	_, err := NewTokenEncryptor([]byte("too-short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}
