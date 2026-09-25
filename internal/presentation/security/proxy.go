// Package security provides small shared HTTP security controls.
package security

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// ProxyResolver trusts forwarded headers only from explicitly configured proxy
// address ranges.
type ProxyResolver struct {
	prefixes  []netip.Prefix
	publicURL string
}

func ParseTrustedProxies(raw string) (ProxyResolver, error) {
	resolver := ProxyResolver{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return ProxyResolver{}, fmt.Errorf("invalid trusted proxy CIDR %q", item)
		}
		resolver.prefixes = append(resolver.prefixes, prefix.Masked())
	}
	return resolver, nil
}

func (resolver ProxyResolver) IsTrusted(remote string) bool {
	ip, ok := remoteAddress(remote)
	if !ok {
		return false
	}
	for _, prefix := range resolver.prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// HasTrustedProxies reports whether forwarded headers can be trusted for a
// request from a configured proxy.
func (resolver ProxyResolver) HasTrustedProxies() bool {
	return len(resolver.prefixes) > 0
}

// WithPublicURL configures the canonical public origin. It supplies a secure
// scheme fallback when TLS terminates before Visto and proxy CIDRs are omitted.
func (resolver ProxyResolver) WithPublicURL(publicURL string) ProxyResolver {
	resolver.publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	return resolver
}

// PublicOrigin returns the configured canonical origin, if one is set.
func (resolver ProxyResolver) PublicOrigin() (string, bool) {
	parsed, err := url.Parse(resolver.publicURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	return parsed.Scheme + "://" + parsed.Host, true
}

// ClientIP returns the peer address unless that peer is a configured proxy. In
// that case it selects the first untrusted address from the right of XFF.
func (resolver ProxyResolver) ClientIP(request *http.Request) string {
	peer, ok := remoteAddress(request.RemoteAddr)
	if !ok {
		return request.RemoteAddr
	}
	if !resolver.IsTrusted(request.RemoteAddr) {
		return peer.String()
	}
	chain := []netip.Addr{}
	for _, item := range strings.Split(request.Header.Get("X-Forwarded-For"), ",") {
		if ip, err := netip.ParseAddr(strings.TrimSpace(item)); err == nil {
			chain = append(chain, ip.Unmap())
		}
	}
	for index := len(chain) - 1; index >= 0; index-- {
		ip := chain[index]
		trusted := false
		for _, prefix := range resolver.prefixes {
			if prefix.Contains(ip) {
				trusted = true
				break
			}
		}
		if !trusted {
			return ip.String()
		}
	}
	return peer.String()
}

func (resolver ProxyResolver) IsHTTPS(request *http.Request) bool {
	if request.TLS != nil || resolver.IsTrusted(request.RemoteAddr) && strings.EqualFold(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	origin, ok := resolver.PublicOrigin()
	if !ok {
		return false
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Scheme, "https")
}

func remoteAddress(raw string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}
