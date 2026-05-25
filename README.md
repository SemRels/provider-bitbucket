# provider-bitbucket

`provider-bitbucket` is a SemRel release provider that creates Bitbucket tags and uploads release notes to the repository downloads area.

## Configuration

Environment variables:

- `BITBUCKET_WORKSPACE`
- `BITBUCKET_REPO_SLUG`
- `BITBUCKET_TOKEN`

## Behavior

- Creates a tag through Bitbucket REST API v2
- Uploads release notes as a text asset through the downloads endpoint

## Development

```bash
go mod tidy
go build ./...
go test ./...
```
