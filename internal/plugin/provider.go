// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Release contains the SemRel release data consumed by this plugin.
type Release struct {
	Version         string
	PreviousVersion string
	TagName         string
	Repository      string
	Changelog       string
	CommitSHA       string
	DryRun          bool
	Metadata        map[string]string
	Commits         []string
}

// Result captures the outcome of a plugin execution.
type Result struct {
	Name       string
	Outputs    map[string]string
	Skipped    bool
	SkipReason string
}

// Provider is the contract exposed by this plugin implementation.
type Provider interface {
	Name() string
	HealthCheck(context.Context) error
	Validate(map[string]interface{}) error
	Execute(context.Context, *Release) (*Result, error)
	ReleaseContext() []string
}

// BitbucketProvider creates tags and uploads release notes to Bitbucket.
type BitbucketProvider struct {
	BaseURL   string
	Workspace string
	RepoSlug  string
	Token     string
	Client    *http.Client
}

// NewBitbucketProvider constructs a Bitbucket provider with explicit configuration.
func NewBitbucketProvider(client *http.Client, baseURL, workspace, repoSlug, token string) *BitbucketProvider {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.bitbucket.org"
	}
	return &BitbucketProvider{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Workspace: strings.TrimSpace(workspace),
		RepoSlug:  strings.TrimSpace(repoSlug),
		Token:     strings.TrimSpace(token),
		Client:    client,
	}
}

// NewBitbucketProviderFromEnv constructs a Bitbucket provider from environment variables.
func NewBitbucketProviderFromEnv() *BitbucketProvider {
	return NewBitbucketProvider(nil, os.Getenv("BITBUCKET_BASE_URL"), os.Getenv("BITBUCKET_WORKSPACE"), os.Getenv("BITBUCKET_REPO_SLUG"), os.Getenv("BITBUCKET_TOKEN"))
}

func (b *BitbucketProvider) Name() string { return "provider-bitbucket" }

func (b *BitbucketProvider) HealthCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (b *BitbucketProvider) Validate(map[string]interface{}) error {
	switch {
	case b.Workspace == "":
		return fmt.Errorf("bitbucket: BITBUCKET_WORKSPACE is required")
	case b.RepoSlug == "":
		return fmt.Errorf("bitbucket: BITBUCKET_REPO_SLUG is required")
	case b.Token == "":
		return fmt.Errorf("bitbucket: BITBUCKET_TOKEN is required")
	}
	return nil
}

func (b *BitbucketProvider) ReleaseContext() []string {
	return []string{"version", "tag", "commit_sha", "changelog"}
}

func (b *BitbucketProvider) Execute(ctx context.Context, rel *Release) (*Result, error) {
	if err := b.HealthCheck(ctx); err != nil {
		return nil, err
	}
	if rel == nil {
		return nil, fmt.Errorf("bitbucket: release is required")
	}
	tagName := strings.TrimSpace(rel.TagName)
	if tagName == "" {
		tagName = strings.TrimSpace(rel.Version)
	}
	if tagName == "" {
		return nil, fmt.Errorf("bitbucket: release tag or version is required")
	}
	if rel.DryRun {
		return &Result{Name: b.Name(), Outputs: map[string]string{"tag": tagName, "dry_run": "true"}}, nil
	}
	if err := b.Validate(nil); err != nil {
		return nil, err
	}

	if err := b.createTag(ctx, tagName, rel.CommitSHA, rel.Version); err != nil {
		return nil, err
	}
	outputs := map[string]string{"tag": tagName}
	if strings.TrimSpace(rel.Changelog) != "" {
		filename := strings.ReplaceAll(tagName, "/", "-") + ".txt"
		if err := b.uploadNotes(ctx, filename, rel.Changelog); err != nil {
			return nil, err
		}
		outputs["notes_file"] = filename
	}
	return &Result{Name: b.Name(), Outputs: outputs}, nil
}

func (b *BitbucketProvider) createTag(ctx context.Context, tagName, commitSHA, version string) error {
	payload := map[string]interface{}{
		"name":    tagName,
		"message": "Release " + strings.TrimSpace(version),
	}
	if strings.TrimSpace(commitSHA) != "" {
		payload["target"] = map[string]string{"hash": commitSHA}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bitbucket: marshal tag payload: %w", err)
	}
	endpoint := b.BaseURL + "/2.0/repositories/" + url.PathEscape(b.Workspace) + "/" + url.PathEscape(b.RepoSlug) + "/refs/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bitbucket: build tag request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: create tag: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bitbucket: create tag returned %s", resp.Status)
	}
	return nil
}

func (b *BitbucketProvider) uploadNotes(ctx context.Context, filename, content string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		return fmt.Errorf("bitbucket: create multipart file: %w", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		return fmt.Errorf("bitbucket: write notes: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("bitbucket: close multipart writer: %w", err)
	}

	endpoint := b.BaseURL + "/2.0/repositories/" + url.PathEscape(b.Workspace) + "/" + url.PathEscape(b.RepoSlug) + "/downloads"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return fmt.Errorf("bitbucket: build upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := b.Client.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: upload notes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bitbucket: upload notes returned %s", resp.Status)
	}
	return nil
}
