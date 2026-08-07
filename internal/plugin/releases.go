// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package plugin provides a Bitbucket Cloud releases provider that creates
// download entries and annotated tags via the Bitbucket REST API v2.0.
//
// Authentication uses HTTP Basic Auth with an app password:
// https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.bitbucket.org/2.0"
	defaultTimeout = 30 * time.Second
)

// Client is a Bitbucket Cloud API client for release operations.
type Client struct {
	baseURL     string
	workspace   string
	repoSlug    string
	username    string
	appPassword string
	http        *http.Client
}

// Config configures the Bitbucket client.
type Config struct {
	BaseURL     string
	Workspace   string
	RepoSlug    string
	Username    string
	AppPassword string
	Timeout     time.Duration
}

func ConfigFromEnv() Config {
	workspace := strings.TrimSpace(os.Getenv("SEMREL_PLUGIN_WORKSPACE"))
	if workspace == "" {
		workspace = strings.TrimSpace(os.Getenv("BITBUCKET_WORKSPACE"))
	}

	repo := strings.TrimSpace(os.Getenv("SEMREL_PLUGIN_REPO"))
	if repo == "" {
		repo = strings.TrimSpace(os.Getenv("BITBUCKET_REPO_SLUG"))
	}
	if workspace == "" || repo == "" {
		fullName := strings.TrimSpace(os.Getenv("BITBUCKET_REPO_FULL_NAME"))
		parts := strings.SplitN(fullName, "/", 2)
		if len(parts) == 2 {
			if workspace == "" {
				workspace = parts[0]
			}
			if repo == "" {
				repo = parts[1]
			}
		}
	}

	username := strings.TrimSpace(os.Getenv("SEMREL_PLUGIN_USERNAME"))
	if username == "" {
		username = strings.TrimSpace(os.Getenv("BITBUCKET_USERNAME"))
	}

	appPassword := strings.TrimSpace(os.Getenv("SEMREL_PLUGIN_APP_PASSWORD"))
	if appPassword == "" {
		appPassword = strings.TrimSpace(os.Getenv("BITBUCKET_APP_PASSWORD"))
	}

	return Config{
		BaseURL:     coalesce(strings.TrimSpace(os.Getenv("SEMREL_PLUGIN_BASE_URL")), defaultBaseURL),
		Workspace:   workspace,
		RepoSlug:    repo,
		Username:    username,
		AppPassword: appPassword,
	}
}

// NewClient creates a new Bitbucket API client.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	t := cfg.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Client{
		baseURL:     strings.TrimRight(base, "/"),
		workspace:   cfg.Workspace,
		repoSlug:    cfg.RepoSlug,
		username:    cfg.Username,
		appPassword: cfg.AppPassword,
		http:        &http.Client{Timeout: t},
	}
}

func New(cfg Config) *Client {
	return NewClient(cfg)
}

// Tag represents a Bitbucket repository tag.
type Tag struct {
	Name   string `json:"name"`
	Target struct {
		Hash string `json:"hash"`
	} `json:"target"`
	Message string `json:"message,omitempty"`
}

// CreateTag creates an annotated tag in the Bitbucket repository.
func (c *Client) CreateTag(ctx context.Context, name, commitHash, message string) (*Tag, error) {
	payload := map[string]any{
		"name":    name,
		"target":  map[string]string{"hash": commitHash},
		"message": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: marshal tag payload: %w", err)
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/refs/tags", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bitbucket: create tag request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: create tag: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("create tag", resp)
	}

	var responseTag Tag
	if err := json.NewDecoder(resp.Body).Decode(&responseTag); err != nil {
		return nil, fmt.Errorf("bitbucket: decode tag response: %w", err)
	}
	return &responseTag, nil
}

// Download represents a Bitbucket repository download entry.
type Download struct {
	Name  string                       `json:"name"`
	Size  int64                        `json:"size"`
	Links map[string]map[string]string `json:"links,omitempty"`
}

func (c *Client) CreateRelease(ctx context.Context, tagName, changelog string) (*Download, error) {
	fileName := strings.TrimSpace(os.Getenv("SEMREL_PLUGIN_NOTES_FILENAME"))
	if fileName == "" {
		fileName = strings.TrimSpace(tagName) + ".md"
	}
	if strings.TrimSpace(fileName) == "" {
		return nil, fmt.Errorf("bitbucket: tag name is required")
	}
	content := changelog
	if content == "" {
		content = tagName
	}
	return c.uploadDownloadContent(ctx, fileName, []byte(content))
}

// UploadDownload uploads a file to the Bitbucket repository downloads.
func (c *Client) UploadDownload(ctx context.Context, filePath string) (*Download, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	return c.uploadMultipart(ctx, filepath.Base(filePath), f)
}

func (c *Client) uploadDownloadContent(ctx context.Context, name string, content []byte) (*Download, error) {
	return c.uploadMultipart(ctx, name, bytes.NewReader(content))
}

func (c *Client) uploadMultipart(ctx context.Context, name string, content io.Reader) (*Download, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("files", name)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: create form file: %w", err)
	}
	if _, err := io.Copy(fw, content); err != nil {
		return nil, fmt.Errorf("bitbucket: copy file: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("bitbucket: close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/downloads", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: upload download request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: upload download: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("upload download", resp)
	}

	return &Download{Name: name}, nil
}

// ListDownloads lists downloads for the repository.
func (c *Client) ListDownloads(ctx context.Context) ([]Download, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/downloads", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: list downloads request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: list downloads: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("list downloads", resp)
	}

	var result struct {
		Values []Download `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("bitbucket: decode downloads: %w", err)
	}
	return result.Values, nil
}

// PipelineVariable represents a Bitbucket repository variable.
type PipelineVariable struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Secured bool   `json:"secured"`
}

// SetPipelineVariable creates or updates a pipeline variable in the repository.
func (c *Client) SetPipelineVariable(ctx context.Context, key, value string, secured bool) error {
	payload := PipelineVariable{Key: key, Value: value, Secured: secured}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bitbucket: marshal variable: %w", err)
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/pipelines_config/variables/", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bitbucket: set variable request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: set variable: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return c.apiError("set pipeline variable", resp)
	}
	return nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.username != "" {
		req.SetBasicAuth(c.username, c.appPassword)
	}
}

func (c *Client) apiError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("bitbucket: %s: status %d: %s", op, resp.StatusCode, strings.TrimSpace(string(body)))
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
