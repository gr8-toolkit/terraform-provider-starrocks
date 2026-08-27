resource "starrocks_table" "audit" {
  database = "starrocks_audit_db__"
  name     = "starrocks_audit_tbl__"
  comment  = "Audit log table"

  key_type    = "DUPLICATE KEY"
  key_columns = ["queryId", "timestamp", "queryType"]

  columns = [
    { name = "queryId",          type = "VARCHAR(64)",       nullable = true,  comment = "Unique ID of the query" },
    { name = "timestamp",        type = "DATETIME",          nullable = false, comment = "Query start time" },
    { name = "queryType",        type = "VARCHAR(12)",       nullable = true,  comment = "Query type (query, slow_query, connection)" },
    { name = "clientIp",         type = "VARCHAR(32)",       nullable = true,  comment = "Client IP" },
    { name = "user",             type = "VARCHAR(64)",       nullable = true,  comment = "Query username" },
    { name = "authorizedUser",   type = "VARCHAR(64)",       nullable = true,  comment = "Unique identifier of the user, i.e., user_identity" },
    { name = "resourceGroup",    type = "VARCHAR(64)",       nullable = true,  comment = "Resource group name" },
    { name = "catalog",          type = "VARCHAR(32)",       nullable = true,  comment = "Catalog name" },
    { name = "db",               type = "VARCHAR(96)",       nullable = true,  comment = "Database where the query runs" },
    { name = "state",            type = "VARCHAR(8)",        nullable = true,  comment = "Query state (EOF, ERR, OK)" },
    { name = "errorCode",        type = "VARCHAR(512)",      nullable = true,  comment = "Error code" },
    { name = "queryTime",        type = "BIGINT",            nullable = true,  comment = "Query execution time (milliseconds)" },
    { name = "scanBytes",        type = "BIGINT",            nullable = true,  comment = "Number of bytes scanned by the query" },
    { name = "scanRows",         type = "BIGINT",            nullable = true,  comment = "Number of rows scanned by the query" },
    { name = "returnRows",       type = "BIGINT",            nullable = true,  comment = "Number of rows returned by the query" },
    { name = "cpuCostNs",        type = "BIGINT",            nullable = true,  comment = "CPU time consumed by the query (nanoseconds)" },
    { name = "memCostBytes",     type = "BIGINT",            nullable = true,  comment = "Memory consumed by the query (bytes)" },
    { name = "stmtId",           type = "INT",               nullable = true,  comment = "Incremental ID of the SQL statement" },
    { name = "isQuery",          type = "TINYINT",           nullable = true,  comment = "Whether the SQL is a query (1 or 0)" },
    { name = "feIp",             type = "VARCHAR(128)",      nullable = true,  comment = "FE IP that executed the statement" },
    { name = "stmt",             type = "VARCHAR(1048576)",  nullable = true,  comment = "Original SQL statement" },
    { name = "digest",           type = "VARCHAR(32)",       nullable = true,  comment = "Fingerprint of slow SQL" },
    { name = "planCpuCosts",     type = "DOUBLE",            nullable = true,  comment = "CPU usage during query planning (nanoseconds)" },
    { name = "planMemCosts",     type = "DOUBLE",            nullable = true,  comment = "Memory usage during query planning (bytes)" },
    { name = "pendingTimeMs",    type = "BIGINT",            nullable = true,  comment = "Time the query waited in the queue (milliseconds)" },
    { name = "candidateMVs",     type = "VARCHAR(65533)",    nullable = true,  comment = "List of candidate materialized views" },
    { name = "hitMvs",           type = "VARCHAR(65533)",    nullable = true,  comment = "List of matched materialized views" },
    { name = "QueriedRelations", type = "ARRAY<VARCHAR(65533)>", nullable = true, comment = "List of directly referenced tables and views" },
    { name = "warehouse",        type = "VARCHAR(32)",       nullable = true,  comment = "Warehouse name" },
  ]

  partition_by   = "PARTITION BY date_trunc('day', timestamp)"
  distributed_by = "DISTRIBUTED BY HASH(queryId) BUCKETS 4"

  properties = {
    "replication_num"      = "1"
    "partition_live_number" = "30"
  }
}
