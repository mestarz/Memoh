package websearch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"
)

func Sogou(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	host := firstNonEmpty(stringValue(cfg["base_url"]), "wsa.tencentcloudapi.com")
	secretID := stringValue(cfg["secret_id"])
	secretKey := stringValue(cfg["secret_key"])
	if secretID == "" || secretKey == "" {
		return nil, errors.New("sogou search requires Tencent Cloud SecretId and SecretKey")
	}
	action := "SearchPro"
	version := "2025-05-08"
	service := "wsa"
	payload, _ := json.Marshal(map[string]any{"Query": query, "Mode": 0})
	now := time.Now().UTC()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	date := now.Format("2006-01-02")
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\n", "application/json", host)
	signedHeaders := "content-type;host"
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", "POST", "/", "", canonicalHeaders, signedHeaders, hashedPayload)
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%s\n%s\n%s", timestamp, credentialScope, sha256Hex([]byte(canonicalRequest)))
	secretDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))
	authorization := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", secretID, credentialScope, signedHeaders, signature)
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", timestamp)
	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildSearchHTTPError(resp.StatusCode, body)
	}
	var rawResp struct {
		Response struct {
			Error *struct{ Code, Message string } `json:"Error,omitempty"`
			Pages []json.RawMessage               `json:"Pages"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, errors.New("invalid search response")
	}
	if rawResp.Response.Error != nil {
		return nil, fmt.Errorf("sogou search failed: %s", rawResp.Response.Error.Message)
	}
	type sogouPage struct {
		Title, URL, Passage string
		Score               float64 `json:"scour"`
	}
	var pages []sogouPage
	for _, raw := range rawResp.Response.Pages {
		var rawStr string
		if err := json.Unmarshal(raw, &rawStr); err == nil {
			var page sogouPage
			if json.Unmarshal([]byte(rawStr), &page) == nil {
				pages = append(pages, page)
			}
		} else {
			var page sogouPage
			if json.Unmarshal(raw, &page) == nil {
				pages = append(pages, page)
			}
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Score > pages[j].Score })
	results := make([]map[string]any, 0)
	for i, page := range pages {
		if i >= count {
			break
		}
		results = append(results, map[string]any{"title": page.Title, "url": page.URL, "description": page.Passage})
	}
	return map[string]any{"query": query, "results": results}, nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
