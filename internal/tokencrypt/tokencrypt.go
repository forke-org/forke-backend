// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

// Package tokencrypt decrypts GitHub tokens stored by forke-dashboard and
// forke-marketing: "v1:" + base64(iv[12] || ciphertext || tag[16]),
// AES-256-GCM with SANDBOX_TOKEN_KEY (base64, 32 bytes). Values without the
// prefix are legacy plaintext and returned as-is.
package tokencrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

const prefix = "v1:"

func Decrypt(stored string) (string, error) {
	if !strings.HasPrefix(stored, prefix) {
		return stored, nil
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("SANDBOX_TOKEN_KEY"))
	if err != nil || len(key) != 32 {
		return "", errors.New("SANDBOX_TOKEN_KEY must be 32 bytes, base64 encoded")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil || len(raw) < 12+16 {
		return "", errors.New("malformed encrypted token")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, raw[:12], raw[12:], nil)
	if err != nil {
		return "", errors.New("could not decrypt token")
	}
	return string(plain), nil
}
