# ---------------------------------------------------------------------------
# Duplicate Key table — the default and most common table type.
# Identical rows are stored as-is; no aggregation or deduplication occurs.
# ---------------------------------------------------------------------------
resource "starrocks_table" "events" {
  database = "mydb"
  name     = "events"
  comment  = "Raw event log"

  key_type    = "DUPLICATE KEY"
  key_columns = ["event_id"]

  columns = [
    {
      name     = "event_id"
      type     = "BIGINT"
      nullable = false
      comment  = "Unique event identifier"
    },
    {
      name     = "user_id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "event_type"
      type     = "VARCHAR(64)"
      nullable = false
    },
    {
      name     = "event_time"
      type     = "DATETIME"
      nullable = false
    },
    {
      name     = "properties"
      type     = "VARCHAR(4096)"
      nullable = true
      default  = "{}"
      comment  = "JSON payload"
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(event_id) BUCKETS 10"

  properties = {
    "replication_num" = "3"
    "compression"     = "LZ4"
  }
}

# ---------------------------------------------------------------------------
# Primary Key table — supports efficient upserts and point deletes.
# ---------------------------------------------------------------------------
resource "starrocks_table" "users" {
  database = "mydb"
  name     = "users"
  comment  = "User profile table"

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
    {
      name     = "email"
      type     = "VARCHAR(255)"
      nullable = true
    },
    {
      name     = "created_at"
      type     = "DATETIME"
      nullable = false
    },
    {
      name     = "is_active"
      type     = "BOOLEAN"
      nullable = false
      default  = "true"
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(user_id) BUCKETS 4"

  properties = {
    "replication_num"          = "3"
    "enable_persistent_index"  = "true"
  }
}

# ---------------------------------------------------------------------------
# Aggregate Key table — aggregates value columns on ingestion.
# Useful for pre-aggregated metrics and reporting tables.
# ---------------------------------------------------------------------------
resource "starrocks_table" "page_views" {
  database = "mydb"
  name     = "page_views"
  comment  = "Hourly page-view counts per URL"

  key_type    = "AGGREGATE KEY"
  key_columns = ["dt", "page_url"]

  columns = [
    {
      name     = "dt"
      type     = "DATE"
      nullable = false
    },
    {
      name     = "page_url"
      type     = "VARCHAR(1024)"
      nullable = false
    },
    # Value columns must carry an aggregation type suffix in the type field.
    {
      name     = "pv"
      type     = "BIGINT SUM"
      nullable = false
      default  = "0"
      comment  = "Page views (aggregated as SUM)"
    },
    {
      name     = "uv"
      type     = "BIGINT SUM"
      nullable = false
      default  = "0"
      comment  = "Unique visitors (aggregated as SUM)"
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(page_url) BUCKETS 8"

  properties = {
    "replication_num" = "3"
  }
}

# ---------------------------------------------------------------------------
# Unique Key table — deduplicates rows by key on ingestion.
# ---------------------------------------------------------------------------
resource "starrocks_table" "orders" {
  database = "mydb"
  name     = "orders"
  comment  = "Latest order state (deduplicated by order_id)"

  key_type    = "UNIQUE KEY"
  key_columns = ["order_id"]

  columns = [
    {
      name     = "order_id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "customer_id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "status"
      type     = "VARCHAR(32)"
      nullable = false
      default  = "pending"
    },
    {
      name     = "total_amount"
      type     = "DECIMAL(18, 2)"
      nullable = false
      default  = "0.00"
    },
    {
      name     = "updated_at"
      type     = "DATETIME"
      nullable = false
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(order_id) BUCKETS 4"

  properties = {
    "replication_num" = "3"
  }
}
