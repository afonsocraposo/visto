package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
)

func normalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{ReadScope, WriteScope, OfflineScope}, nil
	}
	seen := map[string]bool{}
	for _, scope := range scopes {
		if scope != ReadScope && scope != WriteScope && scope != OfflineScope {
			return nil, ErrInvalidRequest
		}
		seen[scope] = true
	}
	if !seen[ReadScope] && !seen[WriteScope] {
		return nil, ErrInvalidRequest
	}
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result, nil
}

func validRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func validChallenge(challenge string) bool {
	if len(challenge) < 43 || len(challenge) > 128 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(decoded) == sha256.Size
}

func validVerifier(verifier string) bool { return verifierPattern.MatchString(verifier) }

func pkceChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func verifyPKCE(challenge, verifier string) bool {
	computed := pkceChallenge(verifier)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

func contains(values []string, item string) bool {
	for _, value := range values {
		if value == item {
			return true
		}
	}
	return false
}
