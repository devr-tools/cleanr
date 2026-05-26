package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type headerAuthFunc func(http.Header)

type jsonAPIClient struct {
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	applyAuth  headerAuthFunc
}

func newJSONAPIClient(baseURL string, headers map[string]string, timeoutMS int, applyAuth headerAuthFunc) *jsonAPIClient {
	timeout := 10 * time.Second
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	return &jsonAPIClient{
		baseURL:    baseURL,
		headers:    headers,
		httpClient: &http.Client{Timeout: timeout},
		applyAuth:  applyAuth,
	}
}

func (c *jsonAPIClient) getJSON(ctx context.Context, resource string, query url.Values, out any) error {
	return c.doJSON(ctx, http.MethodGet, resource, query, nil, out)
}

func (c *jsonAPIClient) postJSON(ctx context.Context, resource string, payload, out any) error {
	return c.doJSON(ctx, http.MethodPost, resource, nil, payload, out)
}

func (c *jsonAPIClient) doJSON(ctx context.Context, method, resource string, query url.Values, payload, out any) error {
	endpoint := c.baseURL + resource
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.applyAuth != nil {
		c.applyAuth(req.Header)
	}
	applyHeaders(req.Header, c.headers)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(compactHTTPError(resp.StatusCode, data))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}
