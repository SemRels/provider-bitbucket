// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bitbucket "github.com/SemRels/provider-bitbucket/internal/plugin"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *bitbucket.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return bitbucket.NewClient(bitbucket.Config{
		BaseURL:     srv.URL,
		Workspace:   "myorg",
		RepoSlug:    "myrepo",
		Username:    "user",
		AppPassword: "pass",
	})
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("SEMREL_PLUGIN_BASE_URL", "https://example.test/api")
	t.Setenv("BITBUCKET_REPO_FULL_NAME", "workspace/repo")
	t.Setenv("SEMREL_PLUGIN_USERNAME", "user")
	t.Setenv("BITBUCKET_APP_PASSWORD", "pass")

	cfg := bitbucket.ConfigFromEnv()

	if cfg.BaseURL != "https://example.test/api" || cfg.Workspace != "workspace" || cfg.RepoSlug != "repo" || cfg.Username != "user" || cfg.AppPassword != "pass" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	t.Parallel()
	c := bitbucket.NewClient(bitbucket.Config{Workspace: "org", RepoSlug: "repo"})
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
}

func TestNewAlias(t *testing.T) {
	t.Parallel()
	if bitbucket.New(bitbucket.Config{Workspace: "org", RepoSlug: "repo"}) == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCreateTag_Success(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/refs/tags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if header := r.Header.Get("Authorization"); !strings.HasPrefix(header, "Basic ") {
			t.Fatalf("Authorization = %q, want Basic auth", header)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "v1.0.0", "message": "Release v1.0.0"})
	})
	c := newTestClient(t, mux)

	tag, err := c.CreateTag(context.Background(), "v1.0.0", "abc123", "Release v1.0.0")
	if err != nil {
		t.Fatalf("CreateTag() error: %v", err)
	}
	if tag.Name != "v1.0.0" {
		t.Errorf("tag.Name = %q, want %q", tag.Name, "v1.0.0")
	}
}

func TestCreateTag_Error(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/refs/tags", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"tag already exists"}}`, http.StatusConflict)
	})
	c := newTestClient(t, mux)

	_, err := c.CreateTag(context.Background(), "v1.0.0", "abc123", "")
	if err == nil {
		t.Error("CreateTag() should return error on 409")
	}
}

func TestCreateRelease_Success(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
	})
	c := newTestClient(t, mux)

	download, err := c.CreateRelease(context.Background(), "v1.2.3", "notes")
	if err != nil {
		t.Fatalf("CreateRelease() error: %v", err)
	}
	if download.Name != "v1.2.3.md" {
		t.Fatalf("download.Name = %q, want %q", download.Name, "v1.2.3.md")
	}
}

func TestCreateRelease_RequiresTag(t *testing.T) {
	t.Parallel()
	c := bitbucket.NewClient(bitbucket.Config{Workspace: "org", RepoSlug: "repo"})
	if _, err := c.CreateRelease(context.Background(), "", "notes"); err == nil {
		t.Fatal("CreateRelease() error = nil, want error")
	}
}

func TestUploadDownload_Success(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})
	c := newTestClient(t, mux)

	filePath := filepath.Join(t.TempDir(), "release.tar.gz")
	if err := os.WriteFile(filePath, []byte("fake archive content"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	dl, err := c.UploadDownload(context.Background(), filePath)
	if err != nil {
		t.Fatalf("UploadDownload() error: %v", err)
	}
	if dl.Name != filepath.Base(filePath) {
		t.Errorf("Download.Name = %q, want %q", dl.Name, filepath.Base(filePath))
	}
}

func TestUploadDownload_OpenError(t *testing.T) {
	t.Parallel()
	c := bitbucket.NewClient(bitbucket.Config{Workspace: "org", RepoSlug: "repo"})
	if _, err := c.UploadDownload(context.Background(), "missing-file.txt"); err == nil {
		t.Fatal("UploadDownload() error = nil, want error")
	}
}

func TestListDownloads_Success(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"values": []map[string]any{{"name": "myrepo-1.0.0.tar.gz", "size": 1024}, {"name": "myrepo-1.0.0.zip", "size": 2048}}})
	})
	c := newTestClient(t, mux)

	downloads, err := c.ListDownloads(context.Background())
	if err != nil {
		t.Fatalf("ListDownloads() error: %v", err)
	}
	if len(downloads) != 2 {
		t.Errorf("len(downloads) = %d, want 2", len(downloads))
	}
}

func TestListDownloads_Error(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	c := newTestClient(t, mux)

	if _, err := c.ListDownloads(context.Background()); err == nil {
		t.Fatal("ListDownloads() error = nil, want error")
	}
}

func TestSetPipelineVariable_Success(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/pipelines_config/variables/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})
	c := newTestClient(t, mux)

	if err := c.SetPipelineVariable(context.Background(), "VERSION", "1.0.0", false); err != nil {
		t.Fatalf("SetPipelineVariable() error: %v", err)
	}
}

func TestSetPipelineVariable_Error(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/pipelines_config/variables/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	})
	c := newTestClient(t, mux)

	if err := c.SetPipelineVariable(context.Background(), "VERSION", "1.0.0", false); err == nil {
		t.Fatal("SetPipelineVariable() error = nil, want error")
	}
}

func TestBasicAuthHeaderValue(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusCreated)
	})
	c := newTestClient(t, mux)

	if _, err := c.CreateRelease(context.Background(), "v1.0.0", "notes"); err != nil {
		t.Fatalf("CreateRelease() error: %v", err)
	}
}
