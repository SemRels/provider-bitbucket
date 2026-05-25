# provider-bitbucket

`provider-bitbucket` is a SemRel subprocess provider for Bitbucket Cloud.

## Configuration

The plugin reads configuration from environment variables:

- `SEMREL_PLUGIN_USERNAME`
- `SEMREL_PLUGIN_APP_PASSWORD`
- `SEMREL_PLUGIN_WORKSPACE`
- `SEMREL_PLUGIN_REPO`
- `SEMREL_PLUGIN_BASE_URL` (optional)
- `SEMREL_PLUGIN_NOTES_FILENAME` (optional, defaults to `<tag>.md`)

It also uses SemRel release context variables such as `SEMREL_TAG_NAME`, `SEMREL_CHANGELOG`, and `SEMREL_DRY_RUN`.

## Behavior

- Uploads release notes to the Bitbucket downloads endpoint as the Bitbucket release artifact for the created tag
- Exits with status `0` on success and non-zero on failure

## Development

```bash
go build ./...
go test ./... -cover
```
