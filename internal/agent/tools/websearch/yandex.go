package websearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type xmlInnerText string

func (t *xmlInnerText) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var buf strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case xml.CharData:
			buf.Write(v)
		case xml.StartElement:
			var inner xmlInnerText
			if err := d.DecodeElement(&inner, &v); err != nil {
				return err
			}
			buf.WriteString(string(inner))
		case xml.EndElement:
			*t = xmlInnerText(buf.String())
			return nil
		}
	}
	*t = xmlInnerText(buf.String())
	return nil
}

type yandexResponse struct {
	XMLName xml.Name      `xml:"response"`
	Results yandexResults `xml:"results"`
}

type yandexResults struct {
	Grouping yandexGrouping `xml:"grouping"`
}

type yandexGrouping struct {
	Groups []yandexGroup `xml:"group"`
}

type yandexGroup struct {
	Doc yandexDoc `xml:"doc"`
}

type yandexDoc struct {
	URL      xmlInnerText   `xml:"url"`
	Title    xmlInnerText   `xml:"title"`
	Passages yandexPassages `xml:"passages"`
}

type yandexPassages struct {
	Passage []xmlInnerText `xml:"passage"`
}

func parseYandexXML(data []byte) ([]map[string]any, error) {
	var resp yandexResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, len(resp.Results.Grouping.Groups))
	for _, group := range resp.Results.Grouping.Groups {
		snippet := ""
		if len(group.Doc.Passages.Passage) > 0 {
			snippet = string(group.Doc.Passages.Passage[0])
		}
		results = append(results, map[string]any{"title": string(group.Doc.Title), "url": string(group.Doc.URL), "description": snippet})
	}
	return results, nil
}

func Yandex(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://searchapi.api.cloud.yandex.net/v2/web/search")
	apiKey := stringValue(cfg["api_key"])
	if apiKey == "" {
		return nil, errors.New("yandex API key is required")
	}
	searchType := firstNonEmpty(stringValue(cfg["search_type"]), "SEARCH_TYPE_RU")
	payload, _ := json.Marshal(map[string]any{
		"query":     map[string]any{"queryText": query, "searchType": searchType},
		"groupSpec": map[string]any{"groupMode": "GROUP_MODE_DEEP", "groupsOnPage": count, "docsInGroup": 1},
	})
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Api-Key "+apiKey)
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
		RawData string `json:"rawData"`
	}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, errors.New("invalid search response")
	}
	xmlData, err := base64.StdEncoding.DecodeString(rawResp.RawData)
	if err != nil {
		return nil, errors.New("failed to decode Yandex response")
	}
	results, err := parseYandexXML(xmlData)
	if err != nil {
		return nil, errors.New("failed to parse Yandex XML response")
	}
	return map[string]any{"query": query, "results": results}, nil
}
