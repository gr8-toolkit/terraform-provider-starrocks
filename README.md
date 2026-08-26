# terraform-provider-starrocks

[![CI](https://github.com/gr8-toolkit/terraform-provider-starrocks/actions/workflows/ci.yml/badge.svg)](https://github.com/gr8-toolkit/terraform-provider-starrocks/actions/workflows/ci.yml)
[![prek](https://img.shields.io/badge/hooks-prek-blue)](https://github.com/j178/prek)
[![Terraform Registry](https://img.shields.io/badge/Terraform_Registry-gr8--toolkit%2Fstarrocks-purple?logo=terraform)](https://registry.terraform.io/providers/gr8-toolkit/starrocks/latest)
[![OpenTofu Registry](https://img.shields.io/badge/OpenTofu_Registry-gr8--toolkit%2Fstarrocks-orange?logo=opentofu)](https://search.opentofu.org/provider/gr8-toolkit/starrocks/latest)

A Terraform provider for [StarRocks](https://www.starrocks.io/) — a high-performance, real-time OLAP database. Manage
databases, tables, indexes, catalogs, resource groups, and cluster-wide configuration entirely from Terraform or
OpenTofu.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) ≥ 1.0 **or**
  [OpenTofu](https://opentofu.org/docs/intro/install/) ≥ 1.6
- [Go](https://go.dev/doc/install) ≥ 1.25 (only required to build from source)
- A running StarRocks cluster accessible over the MySQL protocol (default port `9030`)

## Provider configuration

```hcl
terraform {
  required_providers {
    starrocks = {
      source  = "registry.terraform.io/gr8-toolkit/starrocks"
      version = "~> 0.1"
    }
  }
}

provider "starrocks" {
  host     = "127.0.0.1"
  port     = 9030
  username = "root"
  password = ""
}
```

The provider connects to StarRocks over the MySQL wire protocol using the
[go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) driver.

| Argument | Description | Default |
|---|---|---|
| `host` | StarRocks FE host | — |
| `port` | MySQL protocol port | — |
| `username` | Login user | — |
| `password` | Login password (sensitive) | — |

## Supported resources

| Resource | Description |
|---|---|
| [`starrocks_database`](docs/resources/database.md) | Create and manage databases |
| [`starrocks_table`](docs/resources/table.md) | OLAP tables with column add / remove / type-change support |
| [`starrocks_index`](docs/resources/index.md) | Secondary indexes (BITMAP, NGRAMBF, GIN, VECTOR) |
| [`starrocks_catalog`](docs/resources/catalog.md) | Internal and external catalogs (Hive, Iceberg, JDBC, …) |
| [`starrocks_resource_group`](docs/resources/resource_group.md) | Resource groups and classifiers |
| [`starrocks_global_variable`](docs/resources/global_variable.md) | Cluster-wide system variables |

## Example usage

```hcl
resource "starrocks_database" "app" {
  name       = "app_db"
  data_quota = "100G"
}

resource "starrocks_table" "events" {
  database = starrocks_database.app.name
  name     = "events"

  key_type    = "DUPLICATE KEY"
  key_columns = ["event_id"]

  columns = [
    { name = "event_id", type = "BIGINT",       nullable = false },
    { name = "user_id",  type = "BIGINT",       nullable = false },
    { name = "payload",  type = "VARCHAR(4096)", nullable = true  },
    { name = "ts",       type = "DATETIME",      nullable = false },
  ]

  distributed_by = "DISTRIBUTED BY HASH(event_id) BUCKETS 10"
  properties     = { "replication_num" = "3", "compression" = "LZ4" }
}

resource "starrocks_index" "events_payload" {
  database = starrocks_database.app.name
  table    = starrocks_table.events.name
  name     = "idx_payload"
  type     = "NGRAMBF"
  column   = "payload"

  properties = {
    "gram_num"        = "4"
    "bloom_filter_fpp" = "0.05"
  }
}

resource "starrocks_global_variable" "query_timeout" {
  name  = "query_timeout"
  value = "600"
}
```

## Importing existing resources

Every resource supports `terraform import`. Use the ID format shown in the table below.

| Resource | Import ID format | Example |
|---|---|---|
| `starrocks_database` | `<database>` | `mydb` |
| `starrocks_table` | `<database>.<table>` | `mydb.events` |
| `starrocks_index` | `<database>.<table>.<index>` | `mydb.events.idx_payload` |
| `starrocks_catalog` | `<catalog>` | `hive_catalog` |
| `starrocks_resource_group` | `<name>` | `analytics_rg` |
| `starrocks_global_variable` | `<variable_name>` | `query_timeout` |

## Building from source

```bash
git clone https://github.com/gr8-toolkit/terraform-provider-starrocks.git
cd terraform-provider-starrocks

# Build the provider binary
make build

# Install into the local plugin cache for manual testing
make install

# Run unit tests
make test

# Run acceptance tests against a local StarRocks instance
make testacc                          # defaults to StarRocks 3.5.20
make testacc STARROCKS_VERSION=4.1.1  # specific version
```

## Generating documentation

Docs under `docs/` are generated — do not edit them directly.

```bash
make docs
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
