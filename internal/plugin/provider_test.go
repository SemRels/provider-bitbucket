// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBitbucketProviderExecuteCreatesTagAndUploadsNotes(t *testing.T) {
	t.Parallel()

	var tagCalls atomic.Int32
	var downloadCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer token-123", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/2.0/repositories/workspace/repo/refs/tags":
			tagCalls.Add(1)
			w.WriteHeader(http.StatusCreated)
		case "/2.0/repositories/workspace/repo/downloads":
			downloadCalls.Add(1)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewBitbucketProvider(server.Client(), server.URL, "workspace", "repo", "token-123")
	result, err := provider.Execute(context.Background(), &Release{Version: "1.2.3", TagName: "v1.2.3", CommitSHA: "abc123", Changelog: "release notes"})
	require.NoError(t, err)
	require.EqualValues(t, 1, tagCalls.Load())
	require.EqualValues(t, 1, downloadCalls.Load())
	require.Equal(t, "v1.2.3", result.Outputs["tag"])
	require.Equal(t, "v1.2.3.txt", result.Outputs["notes_file"])
}
