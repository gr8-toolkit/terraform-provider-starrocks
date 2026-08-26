# ---------------------------------------------------------------------------
# Minimal database — no quota or storage-volume settings.
# ---------------------------------------------------------------------------
resource "starrocks_database" "simple" {
  name = "analytics"
}

# ---------------------------------------------------------------------------
# Database with a data quota and replica quota.
# data_quota and replica_quota are write-only: Terraform sends them to
# StarRocks but cannot read them back, so they are only tracked in state.
# ---------------------------------------------------------------------------
resource "starrocks_database" "with_quotas" {
  name          = "metrics"
  data_quota    = "500G"
  replica_quota = 102400
}

# ---------------------------------------------------------------------------
# Cloud-native database with a storage volume (shared-data clusters only).
# ---------------------------------------------------------------------------
resource "starrocks_database" "cloud_native" {
  name           = "datalake"
  storage_volume = "s3_storage_volume"
}

# ---------------------------------------------------------------------------
# Database used as a target for tables — resource references ensure ordering.
# ---------------------------------------------------------------------------
resource "starrocks_database" "app" {
  name       = "app_db"
  data_quota = "100G"
}

resource "starrocks_table" "users" {
  database = starrocks_database.app.name
  name     = "users"

  key_type    = "PRIMARY KEY"
  key_columns = ["user_id"]

  columns = [
    {
      name     = "user_id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "username"
      type     = "VARCHAR(128)"
      nullable = false
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(user_id) BUCKETS 4"
  properties     = { "replication_num" = "3" }
}
