# Development Workflow

## Prerequisites

- Go (version from `go.mod`)
- `prek` — install via `brew install prek` (drop-in replacement for pre-commit)
- Node.js / npm — required by the prettier prek hook
- `tfplugindocs` — installed locally via `make tfplugindocs`
- Docker — required to run acceptance tests locally

## Common Commands

```bash
# Build the provider binary
make build

# Run all unit tests
make test

# Run acceptance tests (starts StarRocks automatically)
make testacc
make testacc STARROCKS_VERSION=4.1.1

# Start / stop a local StarRocks instance
make starrocks
make starrocks-stop

# Install the provider locally for manual testing
make install

# Regenerate docs from schema + templates
make docs

# Install tfplugindocs tool
make tfplugindocs
```

## Prek (hook runner)

All commits must pass prek checks. To run them manually:

```bash
# Run against all files (same as CI)
prek run --all-files

# Run against staged changes only
prek run

# Install hooks so they run automatically on git commit
prek install
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

## Acceptance Tests

Acceptance tests require a live StarRocks instance and are gated behind `TF_ACC=1`.
They are skipped in the regular `make test` run.

### Run locally with Make

```bash
# Start StarRocks (default 3.5.20) and run all acceptance tests
make testacc

# Use a different version
make testacc STARROCKS_VERSION=4.1.1

# Start StarRocks without running tests (useful for manual exploration)
make starrocks
make starrocks STARROCKS_VERSION=4.1.1

# Stop and remove the container when done
make starrocks-stop
```

The `make starrocks` target starts the container and polls until it is healthy,
so `make testacc` is safe to run immediately without a separate wait step.

### CI

The `acceptance-test` job in CI runs automatically on every PR and push to `main`.
It spins up a matrix of the two supported StarRocks releases:

| Version | Image tag |
|---|---|
| 3.5 | `starrocks/allin1-ubuntu:3.5.20` |
| 4.0 | `starrocks/allin1-ubuntu:4.0.13` |
| 4.1 | `starrocks/allin1-ubuntu:4.1.1` |

GitHub Actions runs three jobs on every PR and push to `main`:

- **pre-commit** — runs `prek run --all-files`
- **test** — runs `go build` then `go test -v ./...`
- **acceptance-test** — runs `TestAcc_*` against a StarRocks version matrix

## Adding a New Resource

1. Create `starrocks/<name>_resource.go` following the pattern in `resource_group_resource.go`.
2. Register the new resource in `starrocks/provider.go` → `Resources()`.
3. Add a client method to `starrocks/client.go` that implements the SQL calls.
4. Add a corresponding test file `starrocks/<name>_resource_test.go` using `go-sqlmock`.
5. Add acceptance tests in `starrocks/<name>_acceptance_test.go` using `resource.Test`.
6. Create `examples/resources/starrocks_<name>/resource.tf` for use by tfplugindocs.
7. Create `templates/resources/<name>.md.tmpl` if custom doc templating is needed.
8. Run `make docs` to regenerate `docs/`.
9. Run `prek run --all-files` before committing.

## Docs Generation

Docs in `docs/` are **generated** — do not edit them by hand. Edit the template files in
`templates/` or the schema descriptions in the Go source, then run `make docs`.
