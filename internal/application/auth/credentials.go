package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

func validateAccount(email, displayName, password string) (string, string, error) {
	email, displayName, err := validateIdentity(email, displayName)
	if err != nil {
		return "", "", err
	}
	if len(password) < 12 {
		return "", "", fmt.Errorf("password must be at least 12 characters")
	}
	return email, displayName, nil
}

func validateIdentity(email, displayName string) (string, string, error) {
	email, displayName = strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(displayName)
	separator := strings.LastIndexByte(email, '@')
	if separator < 1 || separator == len(email)-1 || !strings.Contains(email[separator+1:], ".") || strings.ContainsAny(email, " \t\n") || len(email) > 254 {
		return "", "", fmt.Errorf("enter a valid email address")
	}
	if displayName == "" || len(displayName) > 80 {
		return "", "", fmt.Errorf("display name must be 1–80 characters")
	}
	return email, displayName, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return "$visto$argon2id$v=19$m=65536,t=3,p=4$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 7 || parts[1] != "visto" || parts[2] != "argon2id" {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[5])
	expected, keyErr := base64.RawStdEncoding.DecodeString(parts[6])
	if saltErr != nil || keyErr != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, uint32(len(expected)))
	return subtleCompare(actual, expected)
}

func subtleCompare(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var result byte
	for i := range left {
		result |= left[i] ^ right[i]
	}
	return result == 0
}
