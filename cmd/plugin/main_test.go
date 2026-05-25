package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func captureOutput(t *testing.T, fn func() int) (int, string, string) {
	t.Helper()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	exitCode := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, _ = io.Copy(&stdout, stdoutReader)
	_, _ = io.Copy(&stderr, stderrReader)
	_ = stdoutReader.Close()
	_ = stderrReader.Close()

	return exitCode, stdout.String(), stderr.String()
}

func TestRunRequiresCredentials(t *testing.T) {
	t.Setenv("SEMREL_PLUGIN_WORKSPACE", "workspace")
	t.Setenv("SEMREL_PLUGIN_REPO", "repo")
	t.Setenv("SEMREL_TAG_NAME", "v1.2.3")

	exitCode, _, stderr := captureOutput(t, run)

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "SEMREL_PLUGIN_USERNAME") {
		t.Fatalf("stderr = %q, want credentials error", stderr)
	}
}

func TestRunDryRun(t *testing.T) {
	t.Setenv("SEMREL_PLUGIN_USERNAME", "user")
	t.Setenv("SEMREL_PLUGIN_APP_PASSWORD", "pass")
	t.Setenv("SEMREL_PLUGIN_WORKSPACE", "workspace")
	t.Setenv("SEMREL_PLUGIN_REPO", "repo")
	t.Setenv("SEMREL_TAG_NAME", "v1.2.3")
	t.Setenv("SEMREL_DRY_RUN", "true")

	exitCode, stdout, stderr := captureOutput(t, run)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "would create release v1.2.3") {
		t.Fatalf("stdout = %q, want dry-run message", stdout)
	}
}

func TestRunRequiresTag(t *testing.T) {
	t.Setenv("SEMREL_PLUGIN_USERNAME", "user")
	t.Setenv("SEMREL_PLUGIN_APP_PASSWORD", "pass")
	t.Setenv("SEMREL_PLUGIN_WORKSPACE", "workspace")
	t.Setenv("SEMREL_PLUGIN_REPO", "repo")

	exitCode, _, stderr := captureOutput(t, run)

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "SEMREL_TAG_NAME is required") {
		t.Fatalf("stderr = %q, want tag error", stderr)
	}
}

func TestRunSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/repositories/workspace/repo/downloads" {
			t.Fatalf("Path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	t.Setenv("SEMREL_PLUGIN_BASE_URL", server.URL)
	t.Setenv("SEMREL_PLUGIN_USERNAME", "user")
	t.Setenv("SEMREL_PLUGIN_APP_PASSWORD", "pass")
	t.Setenv("SEMREL_PLUGIN_WORKSPACE", "workspace")
	t.Setenv("SEMREL_PLUGIN_REPO", "repo")
	t.Setenv("SEMREL_TAG_NAME", "v1.2.3")
	t.Setenv("SEMREL_CHANGELOG", "notes")

	exitCode, stdout, stderr := captureOutput(t, run)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "release v1.2.3 created") {
		t.Fatalf("stdout = %q, want success message", stdout)
	}
}

func TestRunFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("SEMREL_PLUGIN_BASE_URL", server.URL)
	t.Setenv("SEMREL_PLUGIN_USERNAME", "user")
	t.Setenv("SEMREL_PLUGIN_APP_PASSWORD", "pass")
	t.Setenv("SEMREL_PLUGIN_WORKSPACE", "workspace")
	t.Setenv("SEMREL_PLUGIN_REPO", "repo")
	t.Setenv("SEMREL_TAG_NAME", "v1.2.3")

	exitCode, _, stderr := captureOutput(t, run)

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "provider-bitbucket:") {
		t.Fatalf("stderr = %q, want provider error", stderr)
	}
}
