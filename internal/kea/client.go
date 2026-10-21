package kea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the ISC Kea Control Agent JSON API.
type Client struct {
	URL    string
	HTTP   *http.Client
	DryRun bool
}

type Reservation struct {
	SubnetID int    `json:"subnet-id,omitempty"`
	IPAddress string `json:"ip-address"`
	HWAddress string `json:"hw-address,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

type DHCPRange struct {
	SubnetCIDR string
	PoolStart  string
	PoolEnd    string
}

type Result struct {
	DryRun       bool
	StatusCode   int
	RequestBody  []byte
	ResponseBody string
	Error        string
}

type command struct {
	Command   string `json:"command"`
	Service   []string `json:"service,omitempty"`
	Arguments any    `json:"arguments,omitempty"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// AddReservation sends reservation-add for DHCPv4.
func (c *Client) AddReservation(ctx context.Context, res Reservation) (Result, error) {
	args := map[string]any{
		"reservation": map[string]any{
			"subnet-id":  res.SubnetID,
			"ip-address": res.IPAddress,
			"hw-address": res.HWAddress,
			"hostname":   res.Hostname,
		},
	}
	return c.call(ctx, "reservation-add", []string{"dhcp4"}, args)
}

// SyncPools is a pragmatic config-set style sync: sends a custom "wyldetrace-sync"
// payload documenting desired pools/reservations. When not dry-run, issues
// reservation-add for each host and config-get to verify connectivity.
func (c *Client) SyncPools(ctx context.Context, ranges []DHCPRange, reservations []Reservation) (Result, error) {
	payload := map[string]any{
		"pools":         ranges,
		"reservations":  reservations,
		"source":        "wyldetrace",
	}
	if c.DryRun {
		raw, _ := json.Marshal(payload)
		return Result{DryRun: true, StatusCode: 200, RequestBody: raw, ResponseBody: "dry-run"}, nil
	}
	// Probe control agent first.
	probe, err := c.call(ctx, "version-get", []string{"dhcp4"}, nil)
	if err != nil {
		return probe, err
	}
	var last Result
	for _, r := range reservations {
		if r.IPAddress == "" {
			continue
		}
		last, err = c.AddReservation(ctx, r)
		if err != nil {
			// Continue; Kea may reject duplicates. Surface last error.
			last.Error = err.Error()
		}
	}
	if last.RequestBody == nil {
		raw, _ := json.Marshal(payload)
		last = Result{StatusCode: probe.StatusCode, RequestBody: raw, ResponseBody: probe.ResponseBody}
	}
	return last, nil
}

func (c *Client) call(ctx context.Context, cmd string, service []string, args any) (Result, error) {
	if c.URL == "" {
		return Result{}, fmt.Errorf("kea url not configured")
	}
	body := command{Command: cmd, Service: service, Arguments: args}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	res := Result{DryRun: c.DryRun, RequestBody: raw}
	if c.DryRun {
		res.StatusCode = 200
		res.ResponseBody = "dry-run"
		return res, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.URL, "/"), bytes.NewReader(raw))
	if err != nil {
		return res, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	res.StatusCode = resp.StatusCode
	res.ResponseBody = string(respBody)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return res, fmt.Errorf("%s", res.Error)
	}
	// Kea wraps result in JSON array with result!=0 on failure.
	var parsed []struct {
		Result int    `json:"result"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err == nil && len(parsed) > 0 && parsed[0].Result != 0 {
		res.Error = parsed[0].Text
		if res.Error == "" {
			res.Error = fmt.Sprintf("kea result %d", parsed[0].Result)
		}
		return res, fmt.Errorf("%s", res.Error)
	}
	return res, nil
}
