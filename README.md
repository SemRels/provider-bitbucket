# provider-bitbucket

[![Latest Release](https://img.shields.io/github/v/release/SemRels/provider-bitbucket?label=version&color=blue)](https://github.com/SemRels/provider-bitbucket/releases/latest)
[![CI](https://github.com/SemRels/provider-bitbucket/actions/workflows/ci.yml/badge.svg)](https://github.com/SemRels/provider-bitbucket/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/SemRels/provider-bitbucket)](LICENSE)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/SemRels/provider-bitbucket/badge)](https://scorecard.dev/viewer/?uri=github.com/SemRels/provider-bitbucket)
[![REUSE status](https://api.reuse.software/badge/github.com/SemRels/provider-bitbucket)](https://api.reuse.software/info/github.com/SemRels/provider-bitbucket)

Publishes the semrel release to Bitbucket.

This plugin is distributed as the standalone Go binary `semrel-plugin-provider-bitbucket`. Semrel executes the binary as a subprocess, provides plugin configuration through `SEMREL_PLUGIN_*` environment variables, provides release context through `SEMREL_*` environment variables, reads standard output, and treats exit code `0` as success and any non-zero exit code as failure. Install the binary in `~/.semrel/plugins/` or anywhere on your `$PATH`.

## Installation

### Binary

```bash
go install github.com/SemRels/provider-bitbucket/cmd/plugin@latest
```

### Docker

Pre-built, multi-platform images (linux/amd64, linux/arm64) are published to the GitHub Container Registry on every release:

```bash
docker pull ghcr.io/semrels/provider-bitbucket:latest
```

Images are signed with [cosign](https://github.com/sigstore/cosign) and include a full SBOM attestation. Verify the signature:

```bash
cosign verify ghcr.io/semrels/provider-bitbucket:latest \
  --certificate-identity-regexp 'https://github.com/SemRels/provider-bitbucket/.github/workflows/container.yml.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```


## Configuration

```yaml
plugins:
  - uses: "@semrel/provider-bitbucket"
    phase: release
    args:
      workspace: "acme"
      repo: "service-api"
      app_password: "${BITBUCKET_APP_PASSWORD}"
      username: "semrel-bot"
      comment_on_prs: "true"
```

## Options

Every option can be set as an entry under `args` in `.semrel.yaml`. semrel passes each entry to the plugin as the environment variable shown in the table, so you can also set that variable directly (for example in your CI environment) instead of using `args`.

| Argument | Environment variable | Required | Description | Default |
| --- | --- | --- | --- | --- |
| `workspace` | `SEMREL_PLUGIN_WORKSPACE` | Required | Bitbucket workspace name. | None |
| `repo` | `SEMREL_PLUGIN_REPO` | Optional | Bitbucket repository slug. Defaults from the git remote when available. | Derived from git remote |
| `app_password` | `SEMREL_PLUGIN_APP_PASSWORD` | Required | Bitbucket app password. | None |
| `username` | `SEMREL_PLUGIN_USERNAME` | Required | Bitbucket username. | None |
| `comment_on_prs` | `SEMREL_PLUGIN_COMMENT_ON_PRS` | Optional | Comment on pull requests associated with commits in the release. Defaults to true; set to `false` to disable. The app password needs repository and pull request write permissions. | true |

## `SEMREL_*` release context used

| Variable | Description |
| --- | --- |
| `SEMREL_TAG_NAME` | Git tag name semrel will create or publish. |
| `SEMREL_CHANGELOG` | Generated changelog text for the release. |
| `SEMREL_DRY_RUN` | Whether semrel is running in dry-run mode. |

## Example behavior

The plugin creates or updates a Bitbucket release for the current tag and publishes the changelog text as the release notes. By default it posts an idempotent link comment to associated pull requests. Set `SEMREL_PLUGIN_COMMENT_ON_PRS` to `false` to disable comments.

## License

Apache-2.0
