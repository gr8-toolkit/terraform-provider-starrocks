# ---------------------------------------------------------------------------
# Query timeout — controls how long (seconds) a query can run cluster-wide.
# Destroying this resource resets query_timeout to its StarRocks default.
# ---------------------------------------------------------------------------
resource "starrocks_global_variable" "query_timeout" {
  name  = "query_timeout"
  value = "600"
}

# ---------------------------------------------------------------------------
# Memory limit per query on each BE node (bytes).
# ---------------------------------------------------------------------------
resource "starrocks_global_variable" "exec_mem_limit" {
  name  = "exec_mem_limit"
  value = "4294967296" # 4 GiB
}

# ---------------------------------------------------------------------------
# Enable or disable the query profile (boolean variable).
# StarRocks stores booleans as "true" / "false" strings.
# ---------------------------------------------------------------------------
resource "starrocks_global_variable" "enable_profile" {
  name  = "enable_profile"
  value = "true"
}

# ---------------------------------------------------------------------------
# Multiple variables managed together so they are all reset atomically
# when the module is destroyed.
# ---------------------------------------------------------------------------
resource "starrocks_global_variable" "pipeline_dop" {
  name  = "pipeline_dop"
  value = "0" # 0 = auto-detect from CPU cores
}

resource "starrocks_global_variable" "parallel_fragment_exec_instance_num" {
  name  = "parallel_fragment_exec_instance_num"
  value = "8"
}
