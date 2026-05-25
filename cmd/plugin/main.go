// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package main

import (
	"log"

	plugin "github.com/SemRels/provider-bitbucket/internal/plugin"
)

func main() {
	client := plugin.NewClient(plugin.Config{})
	log.Printf("provider-bitbucket plugin ready: creates Bitbucket releases (%T)", client)
}
