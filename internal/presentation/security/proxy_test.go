package security

import (
	"net/http"
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

func TestProxyResolver_GivenPublicHTTPSOriginWithoutTrustedCIDRs_WhenProxyHeadersArrive_ThenUsesOnlyPeerForClientIPAndPublicURLForHTTPS(t *testing.T) {
	resolver, err := ParseTrustedProxies("")
	if err != nil {
		t.Fatalf("parse proxies: %v", err)
	}
	resolver = resolver.WithPublicURL("https://visto.example.com")
	request := httptest.NewRequest(http.MethodGet, "http://visto.internal", nil)
	request.RemoteAddr = "172.20.0.13:8080"
	request.Header.Set("X-Forwarded-For", "198.51.100.7")
	request.Header.Set("X-Forwarded-Proto", "https")

	if got := resolver.ClientIP(request); got != "172.20.0.13" {
		t.Fatalf("client IP = %q, want direct proxy peer when no proxy CIDR is configured", got)
	}
	if !resolver.IsHTTPS(request) {
		t.Fatal("expected configured HTTPS public URL to be used as the scheme fallback")
	}
	if resolver.HasTrustedProxies() {
		t.Fatal("expected proxy trust to remain unconfigured")
	}
}
