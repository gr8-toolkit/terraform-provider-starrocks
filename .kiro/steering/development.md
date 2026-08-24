# Development Workflow

## Prerequisites

- Go (version from `go.mod`)
- `pre-commit` — install via `pip install pre-commit` or `brew install pre-commit`
- Node.js / npm — required by the prettier pre-commit hook
- `tfplugindocs` — installed locally via `make tfplugindocs`

## Common Commands

```bash
# Build the provider binary
make build

# Run all unit tests
make test

# Install the provider locally for manual testing
make install

# Regenerate docs from schema + templates
make docs

# Install tfplugindocs tool
make tfplugindocs
```

## Pre-commit

All commits must pass pre-commit checks. To run them manually:

```bash
# Run against all files (same as CI)
pre-commit run --all-files

# Run against staged changes only
pre-commit run

# Install hooks so they run automatically on git commit
pre-commit install
```

Hooks configured (see `.pre-commit-config.yaml`):

| Hook | What it checks |
|---|---|
| `check-merge-conflict` | No unresolved merge conflict markers |
| `check-added-large-files` | No accidentally committed large binaries |
| `detect-private-key` | No private keys in the repo |
| `check-case-conflict` | No filename case conflicts |
| `mixed-line-ending` | Consistent line endings |
| `trailing-whitespace` | No trailing whitespace (markdown linebreak-safe) |
| `end-of-file-fixer` | Files end with a newline |
| `prettier` | YAML / AVSC files formatted with Prettier |
| `markdownlint` | Markdown style (config in `.markdownlint.yaml`) |

## CI

GitHub Actions runs two jobs on every PR and push to `main`:

- **pre-commit** — runs `pre-commit run --all-files`
- **test** — runs `go build` then `go test -v ./...`

## Adding a New Resource

1. Create `starrocks/<name>_resource.go` following the pattern in `resource_group_resource.go`.
2. Register the new resource in `starrocks/provider.go` → `Resources()`.
3. Add a client method to `starrocks/client.go` that implements the SQL calls.
4. Add a corresponding test file `starrocks/<name>_resource_test.go` using `go-sqlmock`.
5. Create `examples/resources/starrocks_<name>/resource.tf` for use by tfplugindocs.
6. Create `templates/resources/<name>.md.tmpl` if custom doc templating is needed.
7. Run `make docs` to regenerate `docs/`.
8. Run `pre-commit run --all-files` before committing.

## Docs Generation

Docs in `docs/` are **generated** — do not edit them by hand. Edit the template files in
`templates/` or the schema descriptions in the Go source, then run `make docs`.
