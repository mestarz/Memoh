package main

import "testing"

func TestIsBrowserArgMatchesRealBrowserExecutables(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{
		"chromium",
		"/usr/bin/chromium",
		"/usr/lib/chromium/chromium",
		"google-chrome-stable",
		"/opt/google/chrome/chrome",
	} {
		if !isBrowserArg(arg) {
			t.Fatalf("expected %q to be recognized as a browser executable", arg)
		}
	}
}

func TestIsBrowserArgRejectsShellCommandsContainingBrowserText(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{
		"sh -lc command -v chromium",
		"--remote-debugging-port=9222",
		"/tmp/memoh-display-prepare.sh",
	} {
		if isBrowserArg(arg) {
			t.Fatalf("expected %q not to be recognized as a browser executable", arg)
		}
	}
}

func TestChromiumBypassListNormalizesNoProxySyntax(t *testing.T) {
	t.Parallel()

	got := chromiumBypassList(
		"127.0.0.1, localhost,.example.com",
		"::1,*.foo.bar,127.0.0.1,10.0.0.0/8",
	)
	want := "127.0.0.1,localhost,*.example.com,::1,*.foo.bar,10.0.0.0/8"
	if got != want {
		t.Fatalf("chromiumBypassList mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestChromiumBypassListEmpty(t *testing.T) {
	t.Parallel()

	if got := chromiumBypassList("", "  ,  ,"); got != "" {
		t.Fatalf("expected empty bypass list, got %q", got)
	}
}

func TestResolveBrowserProxyPrecedence(t *testing.T) {
	t.Setenv("MEMOH_BROWSER_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", "http://up:8080")
	t.Setenv("http_proxy", "")
	if got := resolveBrowserProxy(); got != "http://up:8080" {
		t.Fatalf("HTTP_PROXY fallback: got %q", got)
	}
	t.Setenv("HTTPS_PROXY", "http://https:9090")
	if got := resolveBrowserProxy(); got != "http://https:9090" {
		t.Fatalf("HTTPS_PROXY priority: got %q", got)
	}
	t.Setenv("MEMOH_BROWSER_PROXY", "http://browser:7777")
	if got := resolveBrowserProxy(); got != "http://browser:7777" {
		t.Fatalf("MEMOH_BROWSER_PROXY override: got %q", got)
	}
}
