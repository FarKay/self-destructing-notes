package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// parseKey turns the NOTES_KEY variable (32 random bytes, base64 encoded)
// into the key used to encrypt notes.
func parseKey(b64 string) ([]byte, error) {
	if b64 == "" {
		return nil, errors.New("NOTES_KEY is not set")
	}
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("NOTES_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("NOTES_KEY must decode to exactly 32 bytes")
	}
	return key, nil
}

// encrypt seals the text with AES-256-GCM. GCM both encrypts and detects
// tampering. The random nonce is stored in front of the ciphertext.
func encrypt(key []byte, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// a nonce must be unique for every encryption with the same key
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends the ciphertext to the nonce: result = nonce + ciphertext
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// decrypt reverses encrypt. It fails if the data was changed.
func decrypt(key []byte, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// newID makes an unguessable note ID: 16 random bytes from crypto/rand.
// math/rand must never be used for this, because its output is predictable.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}