package models

import (
	"net/http"
	"time"

	"github.com/memohai/memoh/internal/httpproxy"
)

const (
	DefaultProviderRequestTimeout      = 2 * time.Minute
	DefaultProviderProbeTimeout        = 60 * time.Second
	DefaultProviderTLSHandshakeTimeout = 30 * time.Second
)

var defaultProviderTransport = newDefaultProviderTransport()

// NewProviderHTTPClient returns an HTTP client for model/provider traffic.
// When timeout is zero or negative, the caller is expected to enforce limits
// via context deadlines, which keeps streaming responses unbounded by the
// client's global timeout while still using the relaxed TLS handshake window.
func NewProviderHTTPClient(timeout time.Duration) *http.Client {
	client := &http.Client{Transport: defaultProviderTransport}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return client
}

// NewProviderHTTPClientWithProxy returns a provider HTTP client whose
// transport routes traffic through proxyURL when proxyURL is non-empty.
// An invalid proxyURL falls back to the default (env-based) transport.
func NewProviderHTTPClientWithProxy(timeout time.Duration, proxyURL string) *http.Client {
	if proxyURL == "" {
		return NewProviderHTTPClient(timeout)
	}
	transport := defaultProviderTransport.Clone()
	if err := httpproxy.ApplyToTransport(transport, proxyURL); err != nil {
		return NewProviderHTTPClient(timeout)
	}
	client := &http.Client{Transport: transport}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return client
}

func newDefaultProviderTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{TLSHandshakeTimeout: DefaultProviderTLSHandshakeTimeout}
	}

	transport := base.Clone()
	if transport.TLSHandshakeTimeout < DefaultProviderTLSHandshakeTimeout {
		transport.TLSHandshakeTimeout = DefaultProviderTLSHandshakeTimeout
	}
	return transport
}
