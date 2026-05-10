package httpproxy

import (
	"net/http"
	"testing"
)

func TestValidateNormalizesScheme(t *testing.T) {
	t.Parallel()
	got, err := Validate("127.0.0.1:7890")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
}

func TestValidateRejectsBadScheme(t *testing.T) {
	t.Parallel()
	if _, err := Validate("ftp://example.com"); err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestApplyToTransportEmptyFallsBackToEnv(t *testing.T) {
	t.Parallel()
	tr := &http.Transport{}
	if err := ApplyToTransport(tr, ""); err != nil {
		t.Fatalf("ApplyToTransport() error = %v", err)
	}
	if tr.Proxy == nil {
		t.Fatal("expected fallback Proxy to be set")
	}
}

func TestApplyToTransportSetsProxy(t *testing.T) {
	t.Parallel()
	tr := &http.Transport{}
	if err := ApplyToTransport(tr, "http://proxy.local:3128"); err != nil {
		t.Fatalf("ApplyToTransport() error = %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	u, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy func error = %v", err)
	}
	if u == nil || u.Host != "proxy.local:3128" {
		t.Fatalf("unexpected proxy URL: %v", u)
	}
}
