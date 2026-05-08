package websearch

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ddgResultLinkRe    = regexp.MustCompile(`class="result__a"[^>]*href="([^"]+)"`)
	ddgResultTitleRe   = regexp.MustCompile(`class="result__a"[^>]*>([^<]+)<`)
	ddgResultSnippetRe = regexp.MustCompile(`class="result__snippet"[^>]*>([\s\S]*?)</a>`)
	ddgHTMLTagRe       = regexp.MustCompile(`<[^>]*>`)
)

func DuckDuckGo(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://html.duckduckgo.com/html/")
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	form := url.Values{}
	form.Set("q", query)
	form.Set("b", "")
	form.Set("kl", "")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildSearchHTTPError(resp.StatusCode, body)
	}
	htmlStr := string(body)
	links := ddgResultLinkRe.FindAllStringSubmatch(htmlStr, -1)
	titles := ddgResultTitleRe.FindAllStringSubmatch(htmlStr, -1)
	snippets := ddgResultSnippetRe.FindAllStringSubmatch(htmlStr, -1)
	n := len(links)
	if len(titles) < n {
		n = len(titles)
	}
	if count < n {
		n = count
	}
	results := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		rawURL := html.UnescapeString(links[i][1])
		realURL := extractDDGURL(rawURL)
		title := html.UnescapeString(strings.TrimSpace(titles[i][1]))
		snippet := ""
		if i < len(snippets) {
			snippet = html.UnescapeString(strings.TrimSpace(ddgHTMLTagRe.ReplaceAllString(snippets[i][1], "")))
		}
		if realURL == "" {
			continue
		}
		results = append(results, map[string]any{"title": title, "url": realURL, "description": snippet})
	}
	return map[string]any{"query": query, "results": results}, nil
}

func extractDDGURL(rawURL string) string {
	if strings.Contains(rawURL, "uddg=") {
		parsed, err := url.Parse(rawURL)
		if err == nil {
			if uddg := parsed.Query().Get("uddg"); uddg != "" {
				return uddg
			}
		}
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	return rawURL
}
