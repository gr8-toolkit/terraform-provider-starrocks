# ---------------------------------------------------------------------------
# BITMAP index — the default and most widely supported type.
# Best for equality queries (=, IN) on columns with moderate-to-high cardinality.
# ---------------------------------------------------------------------------
resource "starrocks_index" "bitmap_status" {
  database = "mydb"
  table    = "orders"
  name     = "idx_status"
  type     = "BITMAP"
  column   = "status"
  comment  = "Accelerates equality filters on order status"
  timeout  = 300
}

# ---------------------------------------------------------------------------
# N-gram Bloom filter index — accelerates LIKE queries and ngram_search().
# Only valid on STRING / CHAR / VARCHAR columns.
# ---------------------------------------------------------------------------
resource "starrocks_index" "ngrambf_description" {
  database = "mydb"
  table    = "products"
  name     = "idx_desc_ngram"
  type     = "NGRAMBF"
  column   = "description"
  comment  = "Speeds up LIKE '%keyword%' searches on product descriptions"
  timeout  = 300

  # gram_num controls the sub-string length used for tokenisation.
  # bloom_filter_fpp is the false-positive probability (0.0001–0.05).
  properties = {
    "gram_num"         = "4"
    "bloom_filter_fpp" = "0.05"
  }
}

# ---------------------------------------------------------------------------
# GIN (full-text inverted) index — accelerates full-text search via MATCH.
# Requires StarRocks v3.3+.
# ---------------------------------------------------------------------------
resource "starrocks_index" "gin_content" {
  database = "mydb"
  table    = "articles"
  name     = "idx_content_gin"
  type     = "GIN"
  column   = "content"
  comment  = "Full-text inverted index for article content"
  timeout  = 600
}

# ---------------------------------------------------------------------------
# Example: index declared alongside the table it belongs to, using
# a resource reference to guarantee creation order.
# ---------------------------------------------------------------------------
resource "starrocks_table" "events" {
  database = "mydb"
  name     = "events"

  key_type    = "DUPLICATE KEY"
  key_columns = ["event_id"]

  columns = [
    {
      name     = "event_id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "event_type"
      type     = "VARCHAR(64)"
      nullable = false
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(event_id) BUCKETS 4"
  properties     = { "replication_num" = "3" }
}

resource "starrocks_index" "events_type" {
  database = starrocks_table.events.database
  table    = starrocks_table.events.name
  name     = "idx_event_type"
  type     = "BITMAP"
  column   = "event_type"
  comment  = "Speeds up filtering by event type"
  timeout  = 300
}
