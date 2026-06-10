// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	plugin "github.com/SemRels/provider-bitbucket/internal/plugin"
)

const pluginSchemaVersion = 1

func main() {
	_, _ = fmt.Fprintf(os.Stderr, "plugin_schema_version=%d\n", pluginSchemaVersion)
	os.Exit(run())
}

func run() int {
	cfg := plugin.ConfigFromEnv()
	if cfg.Username == "" || cfg.AppPassword == "" {
		fmt.Fprintln(os.Stderr, "provider-bitbucket: SEMREL_PLUGIN_USERNAME and SEMREL_PLUGIN_APP_PASSWORD are required")
		return 1
	}
	if cfg.Workspace == "" || cfg.RepoSlug == "" {
		fmt.Fprintln(os.Stderr, "provider-bitbucket: SEMREL_PLUGIN_WORKSPACE and SEMREL_PLUGIN_REPO are required")
		return 1
	}

	tagName := os.Getenv("SEMREL_TAG_NAME")
	if tagName == "" {
		fmt.Fprintln(os.Stderr, "provider-bitbucket: SEMREL_TAG_NAME is required")
		return 1
	}
	if os.Getenv("SEMREL_DRY_RUN") == "true" {
		fmt.Printf("provider-bitbucket: [dry-run] would create release %s\n", tagName)
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := plugin.New(cfg)
	if _, err := client.CreateRelease(ctx, tagName, os.Getenv("SEMREL_CHANGELOG")); err != nil {
		fmt.Fprintln(os.Stderr, "provider-bitbucket:", err)
		return 1
	}

	fmt.Printf("provider-bitbucket: release %s created\n", tagName)
	return 0
}
