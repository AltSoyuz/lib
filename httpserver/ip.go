package httpserver

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

var (
	trustedProxyMu   sync.RWMutex
	trustedProxyNets = defaultTrustedProxyCIDRs()
)

func defaultTrustedProxyCIDRs() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
}

// SetTrustedProxyCIDRs configures which direct peers are allowed to supply
// trusted forwarded headers (X-Forwarded-For).
//
// Input format:
//   - comma-separated CIDRs ("10.0.0.0/8,192.168.1.0/24")
//   - or comma-separated IPs ("127.0.0.1,::1")
//   - empty string resets to loopback defaults
//   - "none" disables forwarded-header trust entirely
func SetTrustedProxyCIDRs(spec string) error {
	prefixes, err := parseTrustedProxyCIDRs(spec)
	if err != nil {
		return err
	}

	trustedProxyMu.Lock()
	trustedProxyNets = prefixes
	trustedProxyMu.Unlock()

	return nil
}

func parseTrustedProxyCIDRs(spec string) ([]netip.Prefix, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return defaultTrustedProxyCIDRs(), nil
	}
	if strings.EqualFold(trimmed, "none") {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, p := range parts {
		part := strings.TrimSpace(p)
		if part == "" {
			continue
		}

		if strings.Contains(part, "/") {
			prefix, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", part, err)
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP %q: %w", part, err)
		}

		bits := 32
		if addr.Is6() {
			bits = 128
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, bits))
	}

	return prefixes, nil
}

func isTrustedProxy(addr netip.Addr) bool {
	trustedProxyMu.RLock()
	defer trustedProxyMu.RUnlock()

	for _, p := range trustedProxyNets {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ClientIP returns the best-guess client IP address for the request.
//
// Security model:
//   - X-Forwarded-For is trusted only when the direct peer is in the
//     configured trusted-proxy CIDRs.
//   - otherwise we ignore forwarded headers and use RemoteAddr directly.
//
// When X-Forwarded-For is trusted, we strip trusted-proxy hops from the right
// and return the rightmost remaining (untrusted) address — i.e. the leftmost
// address not appended by a known proxy. This prevents both spoofed leftmost
// entries and intermediate-proxy IPs from being mistaken for the real client.
func ClientIP(r *http.Request) string {
	remote, ok := remoteIP(r.RemoteAddr)
	if !ok {
		return ""
	}

	if isTrustedProxy(remote) {
		if addr, ok := firstUntrustedForwardedIP(r.Header.Get("X-Forwarded-For")); ok {
			return addr.String()
		}
	}

	return remote.String()
}

// IsTrustedPeer reports whether the direct peer of r is in the configured
// trusted-proxy set.
func IsTrustedPeer(r *http.Request) bool {
	remote, ok := remoteIP(r.RemoteAddr)
	if !ok {
		return false
	}
	return isTrustedProxy(remote)
}

func remoteIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

// firstUntrustedForwardedIP scans the X-Forwarded-For list right-to-left,
// skipping entries whose IP falls within a trusted-proxy CIDR, and returns
// the rightmost remaining (untrusted) address.
//
// This handles multi-proxy chains correctly: trusted intermediate proxies are
// stripped so the result is the first address that was not injected by a known
// proxy — typically the real client IP.
func firstUntrustedForwardedIP(xff string) (netip.Addr, bool) {
	if xff == "" {
		return netip.Addr{}, false
	}

	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(parts[i])
		if ipStr == "" {
			continue
		}
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			continue
		}
		if !isTrustedProxy(addr) {
			return addr, true
		}
	}

	return netip.Addr{}, false
}
