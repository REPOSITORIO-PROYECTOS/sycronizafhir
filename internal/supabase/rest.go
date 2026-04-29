package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RestClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewRestClient(baseURL, apiKey string) *RestClient {
	return &RestClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *RestClient) Upsert(ctx context.Context, table string, payload interface{}) error {
	return c.UpsertWithConflict(ctx, table, payload, nil)
}

func (c *RestClient) UpsertWithConflict(ctx context.Context, table string, payload interface{}, conflictColumns []string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal upsert payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/v1/%s", c.baseURL, table)
	if len(conflictColumns) > 0 {
		conflict := url.QueryEscape(strings.Join(conflictColumns, ","))
		endpoint = fmt.Sprintf("%s?on_conflict=%s", endpoint, conflict)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build upsert request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=minimal")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute upsert request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	rawBody, _ := io.ReadAll(res.Body)
	return fmt.Errorf("supabase upsert failed table=%s status=%d body=%s", table, res.StatusCode, string(rawBody))
}
