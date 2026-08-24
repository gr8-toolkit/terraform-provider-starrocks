# Coding Conventions

## Go

- Follow standard Go formatting (`gofmt`). All Go files must be `gofmt`-clean.
- Package name is `starrocks` for all files under `starrocks/`.
- Use the **Terraform Plugin Framework** (`github.com/hashicorp/terraform-plugin-framework`),
  not the legacy SDK.
- Resource models implement the `ResourceGroupModel` interface pattern so the client layer
  is decoupled from Terraform types and can be tested independently.
- All `sql.Rows` must be `defer rows.Close()`-d immediately after the error check.
- Avoid raw string interpolation for SQL identifiers where possible; resource/classifier names
  are trusted internal values, but document any assumption clearly.
- Test files use `go-sqlmock` (`github.com/DATA-DOG/go-sqlmock`) — no live DB required.

## Terraform Resource Conventions

- Resource type names follow the pattern `starrocks_<resource>` (provider prefix + snake_case noun).
- Schema attributes use `snake_case`.
- All `Required` attributes must have no default. Use `Optional` + `Computed` where the API
  may return values not set by the user.
- Mark sensitive values (passwords, tokens) with `Sensitive: true`.
- `ImportState` must be implemented for every resource.

## YAML / Config Files

- YAML files (`.yml`, `.yaml`) are formatted by **Prettier** (v3.1.1). Do not hand-format them;
  let Prettier handle it. Run `pre-commit run prettier --all-files` to auto-fix.
- GitHub Actions workflow files live under `.github/workflows/`.

## Markdown

- Markdown files are linted by **markdownlint** using the config in `.markdownlint.yaml`:
  - Max line length: **120 characters** (tables excluded).
  - Allowed inline HTML elements: `<a>`, `<p>`, `<img>`.
- Do not edit generated docs under `docs/` by hand — regenerate with `make docs`.
- The `docs/` directory follows the [Terraform Registry docs format](https://developer.hashicorp.com/terraform/registry/providers/docs).

## Versioning & Releases

- Versions are injected at build time via `-ldflags "-X main.version=..."` (see `goreleaser.yml`).
- Releases are published via GitHub Actions using GoReleaser (`goreleaser.yml`).
- Commit messages follow the **Conventional Commits** spec (enforced by `semantic.yml`).

## File Endings

- All files must end with a single newline (enforced by `end-of-file-fixer`).
- Use Unix line endings (LF). Mixed line endings are rejected by `mixed-line-ending`.
- No trailing whitespace on any line.
