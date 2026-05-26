# provider-bitbucket

Publishes the semrel release to Bitbucket.

This plugin is distributed as the standalone Go binary `semrel-plugin-provider-bitbucket`. Semrel executes the binary as a subprocess, provides plugin configuration through `SEMREL_PLUGIN_*` environment variables, provides release context through `SEMREL_*` environment variables, reads standard output, and treats exit code `0` as success and any non-zero exit code as failure. Install the binary in `~/.semrel/plugins/` or anywhere on your `$PATH`.

## Installation

```bash
go install github.com/SemRels/provider-bitbucket/cmd/plugin@latest
```

## Configuration

```yaml
plugins:
  - name: provider-bitbucket
    path: ~/.semrel/plugins/semrel-plugin-provider-bitbucket
    env:
      SEMREL_PLUGIN_WORKSPACE: "acme"
      SEMREL_PLUGIN_REPO: "service-api"
      SEMREL_PLUGIN_APP_PASSWORD: "${BITBUCKET_APP_PASSWORD}"
      SEMREL_PLUGIN_USERNAME: "semrel-bot"
```

## `SEMREL_PLUGIN_*` variables

| Name | Required | Description | Default |
| --- | --- | --- | --- |
| `SEMREL_PLUGIN_WORKSPACE` | Required | Bitbucket workspace name. | None |
| `SEMREL_PLUGIN_REPO` | Optional | Bitbucket repository slug. Defaults from the git remote when available. | Derived from git remote |
| `SEMREL_PLUGIN_APP_PASSWORD` | Required | Bitbucket app password. | None |
| `SEMREL_PLUGIN_USERNAME` | Required | Bitbucket username. | None |

## `SEMREL_*` release context used

| Variable | Description |
| --- | --- |
| `SEMREL_TAG_NAME` | Git tag name semrel will create or publish. |
| `SEMREL_CHANGELOG` | Generated changelog text for the release. |
| `SEMREL_DRY_RUN` | Whether semrel is running in dry-run mode. |

## Example behavior

The plugin creates or updates a Bitbucket release for the current tag and publishes the changelog text as the release notes.

## License

Apache-2.0
