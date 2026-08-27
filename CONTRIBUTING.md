# Contributing to terraform-provider-starrocks

Thank you for your interest in contributing. This guide covers everything you
need to get started: setting up your environment, running the test suite,
adding a new resource, and submitting a pull request.

## Prerequisites

| Tool | Purpose | Install |
|---|---|---|
| Go ≥ 1.25 | Build and test | <https://go.dev/doc/install> |
| prek | Git hook runner | `brew install prek` |
| Docker | Acceptance tests | <https://docs.docker.com/get-docker/> |
| Node.js / npm | Prettier hook | <https://nodejs.org/en/download> |
| tfplugindocs | Docs generation | `make tfplugindocs` |

## Setting up your environment

```bash
git clone https://github.com/gr8-toolkit/terraform-provider-starrocks.git
cd terraform-provider-starrocks

# Install git hooks (runs prek checks before every commit)
prek install

# Install the tfplugindocs tool
make tfplugindocs
```

## Common commands

```bash
make build          # compile the provider binary to ./dist/
make test           # run all unit tests
make testacc        # run acceptance tests (starts StarRocks via Docker)
make starrocks      # start local StarRocks container (default 3.5.20)
make starrocks-stop # stop and remove the container
make docs           # regenerate docs/
make install        # install the provider into the local Terraform plugin cache
```

## Running tests

### Unit tests

Unit tests use [go-sqlmock](https://github.com/DATA-DOG/go-sqlmock) — no live
database is required.

```bash
make test
# or
go test ./...
```

### Acceptance tests

Acceptance tests require a running StarRocks instance and the `TF_ACC=1`
environment variable. `make testacc` handles both automatically.

```bash
# Uses StarRocks 3.5.20 by default
make testacc

# Test against a specific version
make testacc STARROCKS_VERSION=4.1.1
```

To run a single acceptance test:

```bash
TF_ACC=1 \
STARROCKS_HOST=127.0.0.1 \
STARROCKS_PORT=9030 \
STARROCKS_USERNAME=root \
STARROCKS_PASSWORD="" \
  go test -v -run TestAcc_Table_addColumn -timeout 20m ./internal/provider/
```

## Code style

- Follow standard Go formatting (`gofmt`). The prek `gofmt` hook enforces this
  on every commit.
- All Go files must be `gofmt`-clean before opening a PR.
- YAML files are formatted by **Prettier**. Run `prek run prettier --all-files`
  to auto-fix.
- Markdown is linted by **markdownlint** (max line length 120, tables excluded).
  Run `prek run markdownlint --all-files` to check.
- Commit messages must follow the
  [Conventional Commits](https://www.conventionalcommits.org/) spec, enforced
  by `semantic.yml`.

## Adding a new resource

Follow the pattern established by the existing resources.

1. **Client methods** — add `Create`, `Get`, `Update`/`Delete` functions to
   `starrocks/client.go`. Keep SQL construction in the client layer so resources
   stay testable without a live database.

2. **Resource file** — create `starrocks/<name>_resource.go`. Implement:
   - `Metadata` — set `TypeName` to `starrocks_<name>`
   - `Schema` — document every attribute with `MarkdownDescription`
   - `Create`, `Read`, `Update`, `Delete`
   - `ImportState` — every resource must be importable
   - `Configure` — type-assert `req.ProviderData.(*Client)`

3. **Register** — add `New<Name>Resource` to the slice in
   `starrocks/provider.go → Resources()`.

4. **Unit tests** — create `starrocks/<name>_resource_test.go`. Use go-sqlmock
   to test client methods and any pure-Go logic. No live DB required.

5. **Acceptance tests** — create `starrocks/<name>_acceptance_test.go`.
   Every acceptance test function must start with `skipIfNotAcc(t)` and be
   prefixed `TestAcc_<Name>_`.

6. **Example** — create `examples/resources/starrocks_<name>/resource.tf`
   covering the main usage patterns, and `import.sh` showing the import syntax.

7. **Docs** — run `make docs` to regenerate `docs/resources/<name>.md`.

8. **Pre-commit** — run `prek run --all-files` and fix any issues before
   committing.

## Pull request checklist

- [ ] `make build` succeeds
- [ ] `make test` passes (all unit tests green)
- [ ] `prek run --all-files` passes
- [ ] New resource has unit tests and acceptance tests
- [ ] `make docs` has been run and generated files are committed
- [ ] Commit messages follow Conventional Commits

## Reporting bugs

Open an issue on GitHub. Include:

- Terraform / OpenTofu version
- Provider version
- StarRocks version
- Minimal reproducing configuration
- Full error output

## License

By contributing you agree that your contributions will be licensed under the
[MIT License](LICENSE).
