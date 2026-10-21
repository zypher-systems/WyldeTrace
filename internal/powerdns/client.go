package powerdns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// Client talks to the PowerDNS Authoritative HTTP API (v1).
type Client struct {
	BaseURL  string
	APIKey   string
	ServerID string
	HTTP     *http.Client
	DryRun   bool
}

type RecordChange struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	TTL        int    `json:"ttl"`
	ChangeType string `json:"changetype"` // REPLACE or DELETE
	Disabled   bool   `json:"disabled,omitempty"`
}

type rrset struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	TTL        int           `json:"ttl"`
	ChangeType string        `json:"changetype"`
	Records    []rrsetRecord `json:"records,omitempty"`
}

type rrsetRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

type patchBody struct {
	RRSets []rrset `json:"rrsets"`
}

// Result describes a (possibly dry-run) API call.
type Result struct {
	DryRun       bool
	StatusCode   int
	RequestBody  []byte
	ResponseBody string
	Error        string
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c *Client) server() string {
	if c.ServerID == "" {
		return "localhost"
	}
	return c.ServerID
}

// UpsertForward creates/replaces A or AAAA for hostname → address.
func (c *Client) UpsertForward(ctx context.Context, hostname string, addr netip.Addr, ttl int) (Result, error) {
	hostname = ensureFQDN(hostname)
	if hostname == "" || !addr.IsValid() {
		return Result{}, fmt.Errorf("hostname and address required")
	}
	typ := "A"
	if addr.Is6() {
		typ = "AAAA"
	}
	if ttl <= 0 {
		ttl = 300
	}
	return c.patch(ctx, zoneFromName(hostname), []rrset{{
		Name: hostname, Type: typ, TTL: ttl, ChangeType: "REPLACE",
		Records: []rrsetRecord{{Content: addr.String()}},
	}})
}

// UpsertPTR creates/replaces PTR for address → hostname.
func (c *Client) UpsertPTR(ctx context.Context, addr netip.Addr, hostname string, ttl int) (Result, error) {
	hostname = ensureFQDN(hostname)
	if hostname == "" || !addr.IsValid() {
		return Result{}, fmt.Errorf("hostname and address required")
	}
	ptrName, zone, err := ptrNameAndZone(addr)
	if err != nil {
		return Result{}, err
	}
	if ttl <= 0 {
		ttl = 300
	}
	return c.patch(ctx, zone, []rrset{{
		Name: ptrName, Type: "PTR", TTL: ttl, ChangeType: "REPLACE",
		Records: []rrsetRecord{{Content: hostname}},
	}})
}

// DeleteForward removes A/AAAA for hostname.
func (c *Client) DeleteForward(ctx context.Context, hostname string, addr netip.Addr) (Result, error) {
	hostname = ensureFQDN(hostname)
	if hostname == "" {
		return Result{}, fmt.Errorf("hostname required")
	}
	typ := "A"
	if addr.IsValid() && addr.Is6() {
		typ = "AAAA"
	}
	return c.patch(ctx, zoneFromName(hostname), []rrset{{
		Name: hostname, Type: typ, ChangeType: "DELETE",
	}})
}

// DeletePTR removes PTR for address.
func (c *Client) DeletePTR(ctx context.Context, addr netip.Addr) (Result, error) {
	if !addr.IsValid() {
		return Result{}, fmt.Errorf("address required")
	}
	ptrName, zone, err := ptrNameAndZone(addr)
	if err != nil {
		return Result{}, err
	}
	return c.patch(ctx, zone, []rrset{{
		Name: ptrName, Type: "PTR", ChangeType: "DELETE",
	}})
}

func (c *Client) patch(ctx context.Context, zone string, sets []rrset) (Result, error) {
	if c.BaseURL == "" {
		return Result{}, fmt.Errorf("powerdns url not configured")
	}
	body := patchBody{RRSets: sets}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	res := Result{DryRun: c.DryRun, RequestBody: raw}
	if c.DryRun {
		res.StatusCode = 204
		res.ResponseBody = "dry-run"
		return res, nil
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/api/v1/servers/" + c.server() + "/zones/" + zone
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(raw))
	if err != nil {
		return res, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	res.StatusCode = resp.StatusCode
	res.ResponseBody = string(respBody)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(res.ResponseBody))
		return res, fmt.Errorf("%s", res.Error)
	}
	return res, nil
}

func ensureFQDN(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return name
}

func zoneFromName(fqdn string) string {
	fqdn = ensureFQDN(fqdn)
	parts := strings.Split(strings.TrimSuffix(fqdn, "."), ".")
	if len(parts) < 2 {
		return fqdn
	}
	return strings.Join(parts[1:], ".") + "."
}

func ptrNameAndZone(addr netip.Addr) (name, zone string, err error) {
	if addr.Is4() {
		ip := addr.As4()
		name = fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa.", ip[3], ip[2], ip[1], ip[0])
		zone = fmt.Sprintf("%d.%d.%d.in-addr.arpa.", ip[2], ip[1], ip[0])
		return name, zone, nil
	}
	if addr.Is6() {
		// Full nibble reverse; zone = /64 reverse (first 16 nibbles from right → last 16 of expanded).
		expanded := expandIPv6(addr)
		var b strings.Builder
		for i := len(expanded) - 1; i >= 0; i-- {
			b.WriteByte(expanded[i])
			b.WriteByte('.')
		}
		b.WriteString("ip6.arpa.")
		name = b.String()
		// Use /64 zone (16 hex chars from left of address = rightmost 16 nibbles of reverse omitted).
		zoneParts := strings.Split(strings.TrimSuffix(name, "ip6.arpa."), ".")
		if len(zoneParts) < 17 {
			return name, name, nil
		}
		zone = strings.Join(zoneParts[16:], ".") + "ip6.arpa."
		return name, zone, nil
	}
	return "", "", fmt.Errorf("invalid address")
}

func expandIPv6(addr netip.Addr) string {
	ip := net.IP(addr.AsSlice()).To16()
	var b strings.Builder
	for i := 0; i < 16; i++ {
		b.WriteString(fmt.Sprintf("%02x", ip[i]))
	}
	return b.String()
}
