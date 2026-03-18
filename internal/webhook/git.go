package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PushEvent represents a GitHub push event payload
type PushEvent struct {
	Ref     string   `json:"ref"`
	Commits []Commit `json:"commits"`
}

// Commit represents a commit in a push event
type Commit struct {
	ID       string   `json:"id"`
	Message  string   `json:"message"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// ParsePushEvent parses the payload of a GitHub push event
func ParsePushEvent(payload []byte) (*PushEvent, error) {
	var event PushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// GitClient handles interactions with the GitHub API
type GitClient struct {
	Token      string
	Owner      string
	Repository string
	Branch     string
	HTTPClient *http.Client
}

// NewGitClient creates a new GitClient
func NewGitClient(token, owner, repository, branch string) *GitClient {
	return &GitClient{
		Token:      token,
		Owner:      owner,
		Repository: repository,
		Branch:     branch,
		HTTPClient: &http.Client{},
	}
}

// FetchFileContent fetches the raw content of a file from the repository
func (c *GitClient) FetchFileContent(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", c.Owner, c.Repository, c.Branch, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "token "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch file %s: status %d", path, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// ListAllDataFiles lists all relevant YAML and MD files in the data directory
func (c *GitClient) ListAllDataFiles(ctx context.Context) ([]string, error) {
	// A naive implementation using GitHub Tree API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", c.Owner, c.Repository, c.Branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "token "+c.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tree: status %d", resp.StatusCode)
	}

	var treeResp struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&treeResp); err != nil {
		return nil, err
	}

	var files []string
	for _, item := range treeResp.Tree {
		if item.Type == "blob" && strings.HasPrefix(item.Path, "data/") {
			if strings.HasSuffix(item.Path, ".yml") || strings.HasSuffix(item.Path, ".yaml") || strings.HasSuffix(item.Path, ".md") {
				files = append(files, item.Path)
			}
		}
	}

	return files, nil
}

// FetchTarball downloads the entire repository as a tar.gz archive stream.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *GitClient) FetchTarball(ctx context.Context) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", c.Owner, c.Repository, c.Branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "token "+c.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to fetch tarball: status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
