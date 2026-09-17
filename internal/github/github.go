// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client handles GitHub REST API interactions
type Client struct {
	HTTPClient *http.Client
}

// NewClient returns an initialized GitHub API client
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PRInfo represents essential metadata about a pull request
type PRInfo struct {
	Number            int    `json:"number"`
	Title             string `json:"title"`
	Body              string `json:"body"`
	HeadSHA           string `json:"head_sha"`
	DeveloperUsername string `json:"developer_username"`
	HTMLURL           string `json:"html_url"`
}

// PRFile represents a file altered within a pull request
type PRFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

// FetchPRInfo queries GitHub for PR title, body, and head SHA
func (c *Client) FetchPRInfo(ctx context.Context, repoFullName string, prNumber int, token string) (*PRInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", repoFullName, prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Forke-Review-Engine/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(b))
	}

	var raw struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			SHA string `json:"sha"`
		} `json:"head"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub PR JSON: %w", err)
	}

	return &PRInfo{
		Number:            raw.Number,
		Title:             raw.Title,
		Body:              raw.Body,
		HeadSHA:           raw.Head.SHA,
		DeveloperUsername: raw.User.Login,
		HTMLURL:           raw.HTMLURL,
	}, nil
}

// FetchPRFiles queries GitHub for the list of modified files in a PR
func (c *Client) FetchPRFiles(ctx context.Context, repoFullName string, prNumber int, token string) ([]PRFile, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/files?per_page=100", repoFullName, prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Forke-Review-Engine/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github PR files returned %d: %s", resp.StatusCode, string(b))
	}

	var files []PRFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode PR files JSON: %w", err)
	}

	return files, nil
}

// BuildGitDiff concatenates individual file patches into a unified git diff format
func BuildGitDiff(files []PRFile) string {
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", f.Filename, f.Filename))
		sb.WriteString(fmt.Sprintf("--- a/%s\n", f.Filename))
		sb.WriteString(fmt.Sprintf("+++ b/%s\n", f.Filename))
		if f.Patch != "" {
			sb.WriteString(f.Patch)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// UpdateCommitStatus posts a status context on the commit SHA
func (c *Client) UpdateCommitStatus(ctx context.Context, repoFullName, sha, state, description, targetURL, token string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/statuses/%s", repoFullName, sha)

	payload := map[string]string{
		"state":       state, // pending, success, failure, error
		"description": description,
		"context":     "Forke / Verification & AI Review",
	}
	if targetURL != "" {
		payload["target_url"] = targetURL
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Forke-Review-Engine/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post GitHub commit status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github commit status update returned %d: %s", resp.StatusCode, string(b))
	}

	return nil
}
