package supabase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

type StorageClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewStorageClient(baseURL, apiKey string) *StorageClient {
	return &StorageClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *StorageClient) UploadObject(ctx context.Context, bucket, objectPath, contentType string, payload []byte) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("storage bucket is required")
	}
	if strings.TrimSpace(objectPath) == "" {
		return fmt.Errorf("storage object path is required")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	cleanObjectPath := strings.TrimLeft(strings.ReplaceAll(objectPath, "\\", "/"), "/")
	endpoint := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, bucket, cleanObjectPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build storage upload request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("x-upsert", "true")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute storage upload request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	rawBody, _ := io.ReadAll(res.Body)
	bodyText := string(rawBody)
	if strings.Contains(bodyText, "row-level security") || strings.Contains(bodyText, `"statusCode":"403"`) {
		return fmt.Errorf(
			"storage upload failed bucket=%s object=%s status=%d body=%s (causa probable: SUPABASE_SERVICE_ROLE_KEY es anon o inválida; la service_role bypass RLS)",
			bucket,
			cleanObjectPath,
			res.StatusCode,
			bodyText,
		)
	}
	return fmt.Errorf("storage upload failed bucket=%s object=%s status=%d body=%s", bucket, cleanObjectPath, res.StatusCode, bodyText)
}

func (c *StorageClient) PublicURL(bucket, objectPath string) string {
	cleanObjectPath := strings.TrimLeft(strings.ReplaceAll(objectPath, "\\", "/"), "/")
	segments := strings.Split(cleanObjectPath, "/")
	for i, segment := range segments {
		segments[i] = pathEscapeSegment(segment)
	}
	encodedPath := strings.Join(segments, "/")
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", c.baseURL, bucket, encodedPath)
}

func pathEscapeSegment(value string) string {
	return strings.ReplaceAll(value, " ", "%20")
}

func ContentTypeFromExtension(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
