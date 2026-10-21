package handlers

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// validateWebhookURL validates user-controlled webhook destinations (strict SSRF).
func validateWebhookURL(raw string) error {
	return validateOutboundURL(raw, true, false)
}

// validateOutboundURL checks scheme, host, and resolved addresses.
// allowHTTP: permit http (lab / WYLDTRACE_DEV). Otherwise https only.
// allowPrivate: permit RFC1918 (Kea/PowerDNS/SNMP on enterprise networks). Webhooks keep this false.
func validateOutboundURL(raw string, allowHTTP, allowPrivate bool) error {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
		// ok
	case "http":
		if !allowHTTP {
			return fmt.Errorf("url scheme must be https")
		}
	default:
		return fmt.Errorf("url scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid url host")
	}
	return validateOutboundHost(host, allowPrivate)
}

func validateOutboundHost(host string, allowPrivate bool) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" || lower == "metadata" ||
		lower == "metadata.aws.internal" {
		return fmt.Errorf("url host not allowed")
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip, allowPrivate) {
			return fmt.Errorf("url host not allowed")
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("url host dns lookup failed")
	}
	if len(ips) == 0 {
		return fmt.Errorf("url host dns lookup failed")
	}
	for _, ip := range ips {
		if isBlockedIP(ip, allowPrivate) {
			return fmt.Errorf("url host not allowed")
		}
	}
	return nil
}

// validateSNMPTarget validates a host or host:port used for SNMP enrichment.
// Private RFC1918 targets are allowed (switches live on site networks); loopback/metadata are not.
func validateSNMPTarget(raw string) error {
	host := strings.TrimSpace(raw)
	if host == "" {
		return fmt.Errorf("target required")
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return validateOutboundHost(host, true)
}

func isBlockedIP(ip net.IP, allowPrivate bool) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if !allowPrivate && ip.IsPrivate() {
		return true
	}
	// Cloud metadata / link-local services.
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}

func (a *API) validateIntegrationURL(raw string) error {
	// Integrations may target private Kea/PowerDNS; still require https outside lab.
	return validateOutboundURL(raw, a.AllowPlainSecrets, true)
}

func (a *API) validateWebhookOutbound(raw string) error {
	return validateOutboundURL(raw, a.AllowPlainSecrets, false)
}
