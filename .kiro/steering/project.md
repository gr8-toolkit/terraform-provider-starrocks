# Project Overview

## What This Is

`terraform-provider-starrocks` is a Terraform provider for [StarRocks](https://www.starrocks.io/),
a high-performance analytical database. It is built with the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) (not the older SDK).

**Module path:** `github.com/gr8-toolkit/terraform-provider-starrocks`

**Registry address:** `registry.terraform.io/gr8-toolkit/starrocks`

## Architecture

```text
main.go                        # Provider entry-point, version injection via ldflags
starrocks/
  provider.go                  # Provider schema (host, port, username, password) and resource registration
  client.go                    # StarRocks client — connects via MySQL wire protocol (go-sql-driver/mysql)
  resource_group_resource.go   # starrocks_resource_group CRUD resource
  *_test.go                    # Unit tests using go-sqlmock for DB mocking
examples/                      # Terraform HCL examples (used by tfplugindocs)
templates/                     # tfplugindocs templates for docs generation
docs/                          # Generated provider documentation (committed to repo)
tools/tools.go                 # blank-import pin for tfplugindocs build tool
```

## Supported Resources

| Resource | Description |
|---|---|
| `starrocks_resource_group` | Manages StarRocks resource groups and classifiers |

## Key Design Decisions

- StarRocks uses the MySQL wire protocol, so the client wraps `database/sql` with
  `go-sql-driver/mysql`.
- The `ResourceGroupModel` interface decouples the client from the Terraform resource model,
  making unit-testing easier.
- Resource group updates are implemented as **delete + recreate** (no ALTER support needed).
- `mem_limit` must be stored with one decimal place (e.g. `"80.0%"`) to match StarRocks'
  internal representation and prevent perpetual drift.
- Classifiers returned by `SHOW RESOURCE GROUP` are not re-read into state after create/update;
  only the planned values are kept to avoid parser fragility.
