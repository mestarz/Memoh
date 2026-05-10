// Package httpproxy provides utilities for constructing HTTP clients and
// transports that route traffic through a configurable HTTP proxy.
//
// It is intentionally dependency-light so it can be reused by model providers,
// OAuth flows and other outbound HTTP traffic that needs an opt-in proxy
// independent from process-wide environment variables.
package httpproxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Validate parses and normalizes a proxy URL string. It returns an error when
// the URL is non-empty but malformed. An empty input returns the empty string
// without error.
func Validate(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	u, err := parse(value)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// ApplyToTransport configures the given transport to route requests through
// the provided proxy URL. When proxyURL is empty the transport is left with
// the default ProxyFromEnvironment behavior.
func ApplyToTransport(transport *http.Transport, proxyURL string) error {
	if transport == nil {
		return errors.New("httpproxy: transport must not be nil")
	}
	value := strings.TrimSpace(proxyURL)
	if value == "" {
		transport.Proxy = http.ProxyFromEnvironment
		return nil
	}
	u, err := parse(value)
	if err != nil {
		return err
	}
	transport.Proxy = http.ProxyURL(u)
	return nil
}

func parse(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	u, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid http proxy url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" && u.Scheme != "socks5h" {
		return nil, fmt.Errorf("unsupported http proxy scheme: %s", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, errors.New("http proxy host is required")
	}
	return u, nil
}
