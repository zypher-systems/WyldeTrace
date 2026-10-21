package handlers

import (
	"strings"
	"testing"
)

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty", raw: "", wantErr: "invalid url"},
		{name: "no_scheme", raw: "example.com/hook", wantErr: "invalid url"},
		{name: "ftp", raw: "ftp://example.com/hook", wantErr: "scheme"},
		// validateWebhookURL allows http (lab-friendly); private/loopback still blocked.
		{name: "http_public_ok", raw: "http://203.0.113.10/hook", wantErr: ""},
		{name: "localhost", raw: "https://localhost/hook", wantErr: "not allowed"},
		{name: "localhost_suffix", raw: "https://app.localhost/hook", wantErr: "not allowed"},
		{name: "loopback_v4", raw: "https://127.0.0.1/hook", wantErr: "not allowed"},
		{name: "loopback_v6", raw: "https://[::1]/hook", wantErr: "not allowed"},
		{name: "unspecified", raw: "https://0.0.0.0/hook", wantErr: "not allowed"},
		{name: "private_10", raw: "https://10.1.2.3/hook", wantErr: "not allowed"},
		{name: "private_192", raw: "https://192.168.1.1/hook", wantErr: "not allowed"},
		{name: "link_local_meta", raw: "https://169.254.169.254/latest", wantErr: "not allowed"},
		{name: "gcp_metadata_name", raw: "https://metadata.google.internal/", wantErr: "not allowed"},
		{name: "aws_metadata_name", raw: "https://metadata.aws.internal/", wantErr: "not allowed"},
		{name: "public_ip_ok", raw: "https://203.0.113.50/hooks/wt", wantErr: ""},
		{name: "public_ip_path", raw: "https://198.51.100.20:8443/a", wantErr: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebhookURL(tc.raw)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateOutboundURL_allowHTTPAndPrivate(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		allowHTTP    bool
		allowPrivate bool
		wantErr      string
	}{
		{
			name: "http_lab_public", raw: "http://203.0.113.9/kea",
			allowHTTP: true, allowPrivate: true, wantErr: "",
		},
		{
			name: "http_denied", raw: "http://203.0.113.9/kea",
			allowHTTP: false, allowPrivate: true, wantErr: "https",
		},
		{
			name: "private_allowed", raw: "https://10.0.0.5/api",
			allowHTTP: true, allowPrivate: true, wantErr: "",
		},
		{
			name: "private_denied", raw: "https://10.0.0.5/api",
			allowHTTP: true, allowPrivate: false, wantErr: "not allowed",
		},
		{
			name: "loopback_still_blocked", raw: "https://127.0.0.1/api",
			allowHTTP: true, allowPrivate: true, wantErr: "not allowed",
		},
		{
			name: "metadata_still_blocked", raw: "https://169.254.169.254/",
			allowHTTP: true, allowPrivate: true, wantErr: "not allowed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOutboundURL(tc.raw, tc.allowHTTP, tc.allowPrivate)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want err containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateSNMPTarget(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty", raw: "", wantErr: "required"},
		{name: "private_ok", raw: "10.0.0.1", wantErr: ""},
		{name: "private_with_port", raw: "10.0.0.1:161", wantErr: ""},
		{name: "loopback", raw: "127.0.0.1", wantErr: "not allowed"},
		{name: "localhost", raw: "localhost", wantErr: "not allowed"},
		{name: "metadata", raw: "169.254.169.254", wantErr: "not allowed"},
		{name: "public_ok", raw: "203.0.113.7", wantErr: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSNMPTarget(tc.raw)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want err containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAPI_validateIntegrationAndWebhook(t *testing.T) {
	lab := &API{AllowPlainSecrets: true}
	if err := lab.validateIntegrationURL("http://10.1.1.1/kea"); err != nil {
		t.Fatalf("lab integration private http: %v", err)
	}
	if err := lab.validateWebhookOutbound("http://203.0.113.1/hook"); err != nil {
		t.Fatalf("lab webhook http public: %v", err)
	}
	if err := lab.validateWebhookOutbound("http://10.1.1.1/hook"); err == nil {
		t.Fatal("lab webhook must still block private")
	}

	strict := &API{AllowPlainSecrets: false}
	if err := strict.validateIntegrationURL("http://10.1.1.1/kea"); err == nil {
		t.Fatal("strict integration must require https")
	}
	if err := strict.validateIntegrationURL("https://10.1.1.1/kea"); err != nil {
		t.Fatalf("strict integration https private ok: %v", err)
	}
}
