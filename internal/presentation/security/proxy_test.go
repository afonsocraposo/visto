package security

import (
	"net/http/httptest"
	"testing"
)

func TestProxyResolver_GivenTrustedProxy_WhenForwardedClientAddressExists_ThenUsesTheClientAddress(t *testing.T) {
	resolver, err := ParseTrustedProxies("192.0.2.0/24, 2001:db8::/32")
	if err != nil {
		t.Fatalf("parse proxies: %v", err)
	}
	request := httptest.NewRequest("GET", "https://visto.example", nil)
	request.RemoteAddr = "192.0.2.10:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.7, 192.0.2.4")
	request.Header.Set("X-Forwarded-Proto", "https")
	if got := resolver.ClientIP(request); got != "198.51.100.7" {
		t.Fatalf("client IP = %q, want forwarded client", got)
	}
	if !resolver.IsHTTPS(request) {
		t.Fatal("expected trusted proxy HTTPS header to be accepted")
	}
}

func TestProxyResolver_GivenUntrustedPeer_WhenForwardedHeadersAreSpoofed_ThenUsesPeerAddress(t *testing.T) {
	resolver, err := ParseTrustedProxies("192.0.2.0/24")
	if err != nil {
		t.Fatalf("parse proxies: %v", err)
	}
	request := httptest.NewRequest("GET", "http://visto.example", nil)
	request.RemoteAddr = "198.51.100.7:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	request.Header.Set("X-Forwarded-Proto", "https")
	if got := resolver.ClientIP(request); got != "198.51.100.7" {
		t.Fatalf("client IP = %q, want peer address", got)
	}
	if resolver.IsHTTPS(request) {
		t.Fatal("untrusted peer spoofed HTTPS")
	}
}
