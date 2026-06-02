package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type githubConfig struct {
	Enabled          bool   `json:"enabled"`
	GithubOwner      string `json:"github_owner"`
	GithubRepo       string `json:"github_repo"`
	ReleaseAssetName string `json:"release_asset_name"`
	GithubToken      string `json:"github_token"`
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name string `json:"name"`
}

func fetchLatestRelease(ctx context.Context, owner, repo, token string) (githubRelease, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return githubRelease{}, fmt.Errorf("github_owner y github_repo son requeridos")
	}

	uri := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sycronizafhir-updater")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("github releases/latest respondio %d", res.StatusCode)
	}

	var release githubRelease
	if err = json.NewDecoder(res.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, fmt.Errorf("release sin tag_name")
	}
	return release, nil
}
