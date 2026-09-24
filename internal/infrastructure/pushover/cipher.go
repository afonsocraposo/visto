package pushover

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const ciphertextVersion = "v1"

type AESGCMCipher struct {
	aead cipher.AEAD
}

func NewAESGCMCipher(encodedKey string) (*AESGCMCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(key) != 32 {
		return nil, errors.New("secret encryption key must be base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create encryption cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create authenticated encryption: %w", err)
	}
	return &AESGCMCipher{aead: aead}, nil
}

func (cipher *AESGCMCipher) Encrypt(value string) (string, error) {
	nonce := make([]byte, cipher.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("create encryption nonce: %w", err)
	}
	ciphertext := cipher.aead.Seal(nonce, nonce, []byte(value), []byte(ciphertextVersion))
	return ciphertextVersion + "." + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func (cipher *AESGCMCipher) Decrypt(value string) (string, error) {
	version, encoded, ok := strings.Cut(value, ".")
	if !ok || version != ciphertextVersion {
		return "", errors.New("unsupported encrypted secret format")
	}
	data, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(data) <= cipher.aead.NonceSize() {
		return "", errors.New("invalid encrypted secret")
	}
	nonce, ciphertext := data[:cipher.aead.NonceSize()], data[cipher.aead.NonceSize():]
	plaintext, err := cipher.aead.Open(nil, nonce, ciphertext, []byte(ciphertextVersion))
	if err != nil {
		return "", errors.New("encrypted secret authentication failed")
	}
	return string(plaintext), nil
}
