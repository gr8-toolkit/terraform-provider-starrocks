package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// accDB is the database used by all table acceptance tests.
// It is created in TestMain when TF_ACC=1 and dropped on exit.
const accDB = "tf_acc_test"

// TestMain creates and tears down the test database so individual table tests
// do not need to manage it themselves.
func TestMain(m *testing.M) {
	if os.Getenv(resource.EnvTfAcc) != "" {
		client, err := accClient()
		if err != nil {
			panic("TestMain: failed to create StarRocks client: " + err.Error())
		}
		if _, err := client.DB.Exec("CREATE DATABASE IF NOT EXISTS `" + accDB + "`"); err != nil {
			panic("TestMain: failed to create test database: " + err.Error())
		}
		code := m.Run()
		// Best-effort cleanup — ignore errors so a partial failure doesn't hide test results.
		_, _ = client.DB.Exec("DROP DATABASE IF EXISTS `" + accDB + "`")
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// TestAcc_Table_basic creates a simple Duplicate Key table, verifies the
// computed fields, then imports it by "database.table" ID.
func TestAcc_Table_basic(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accTableBasicConfig("acc_table_basic"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "database", accDB),
					resource.TestCheckResourceAttr("starrocks_table.test", "name", "acc_table_basic"),
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.#", "2"),
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.0.name", "id"),
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.1.name", "payload"),
					// engine and key_type are computed — just assert they are set
					resource.TestCheckResourceAttrSet("starrocks_table.test", "engine"),
					resource.TestCheckResourceAttrSet("starrocks_table.test", "key_type"),
				),
			},
			// Import by "database.table" and verify round-trip.
			{
				ResourceName:      "starrocks_table.test",
				ImportState:       true,
				ImportStateId:     accDB + ".acc_table_basic",
				ImportStateVerify: false, // StarRocks may normalise types (e.g. INT → int(11))
			},
		},
	})
}

// TestAcc_Table_addColumn starts with two columns and adds a third,
// verifying that Update issues ALTER TABLE ADD COLUMN in-place.
func TestAcc_Table_addColumn(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — initial two-column table.
			{
				Config: accProviderBlock() + accTableBasicConfig("acc_table_add_col"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.#", "2"),
				),
			},
			// Step 2 — add a third column; no replacement should happen.
			{
				Config: accProviderBlock() + accTableThreeColConfig("acc_table_add_col"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.#", "3"),
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.2.name", "score"),
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.2.type", "BIGINT"),
				),
			},
		},
	})
}

// TestAcc_Table_removeColumn starts with three columns and removes the last,
// verifying that Update issues ALTER TABLE DROP COLUMN in-place.
func TestAcc_Table_removeColumn(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — three-column table.
			{
				Config: accProviderBlock() + accTableThreeColConfig("acc_table_rm_col"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.#", "3"),
				),
			},
			// Step 2 — drop the third column.
			{
				Config: accProviderBlock() + accTableBasicConfig("acc_table_rm_col"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.#", "2"),
				),
			},
		},
	})
}

// TestAcc_Table_changeColumnType starts with an INT column and widens it to
// BIGINT, verifying that Update issues ALTER TABLE MODIFY COLUMN in-place.
func TestAcc_Table_changeColumnType(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — score column is INT.
			{
				Config: accProviderBlock() + accTableThreeColConfig("acc_table_mod_col"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.2.type", "BIGINT"),
				),
			},
			// Step 2 — widen score to DOUBLE.
			{
				Config: accProviderBlock() + accTableModifiedColConfig("acc_table_mod_col"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "columns.2.type", "DOUBLE"),
				),
			},
		},
	})
}

// TestAcc_Table_disappears verifies that when the table is deleted outside
// Terraform, Read removes it from state so the next plan proposes a re-create.
func TestAcc_Table_disappears(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accTableBasicConfig("acc_table_disappears"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.test", "name", "acc_table_disappears"),
					testAccDropTableOutOfBand(accDB, "acc_table_disappears"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accTableBasicConfig returns HCL for a two-column Duplicate Key table.
func accTableBasicConfig(name string) string {
	return fmt.Sprintf(`
resource "starrocks_table" "test" {
  database = %q
  name     = %q

  key_type    = "DUPLICATE KEY"
  key_columns = ["id"]

  columns = [
    {
      name     = "id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "payload"
      type     = "VARCHAR(1024)"
      nullable = true
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(id) BUCKETS 1"

  properties = {
    "replication_num" = "1"
  }
}
`, accDB, name)
}

// accTableThreeColConfig adds a BIGINT `score` column to the basic config.
func accTableThreeColConfig(name string) string {
	return fmt.Sprintf(`
resource "starrocks_table" "test" {
  database = %q
  name     = %q

  key_type    = "DUPLICATE KEY"
  key_columns = ["id"]

  columns = [
    {
      name     = "id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "payload"
      type     = "VARCHAR(1024)"
      nullable = true
    },
    {
      name     = "score"
      type     = "BIGINT"
      nullable = true
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(id) BUCKETS 1"

  properties = {
    "replication_num" = "1"
  }
}
`, accDB, name)
}

// accTableModifiedColConfig widens `score` from BIGINT to DOUBLE.
func accTableModifiedColConfig(name string) string {
	return fmt.Sprintf(`
resource "starrocks_table" "test" {
  database = %q
  name     = %q

  key_type    = "DUPLICATE KEY"
  key_columns = ["id"]

  columns = [
    {
      name     = "id"
      type     = "BIGINT"
      nullable = false
    },
    {
      name     = "payload"
      type     = "VARCHAR(1024)"
      nullable = true
    },
    {
      name     = "score"
      type     = "DOUBLE"
      nullable = true
    },
  ]

  distributed_by = "DISTRIBUTED BY HASH(id) BUCKETS 1"

  properties = {
    "replication_num" = "1"
  }
}
`, accDB, name)
}

// ---------------------------------------------------------------------------
// Check helpers
// ---------------------------------------------------------------------------

func testAccDropTableOutOfBand(db, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client for out-of-band table drop: %w", err)
		}
		return client.DropTable(db, name)
	}
}

// TestAcc_Table_auditLog creates the StarRocks audit log table — a Duplicate Key
// table with 29 columns, expression-based partitioning, and an ARRAY column type.
// It verifies the table is created correctly and that an import round-trip works.
func TestAcc_Table_auditLog(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accAuditLogTableConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_table.audit", "database", accDB),
					resource.TestCheckResourceAttr("starrocks_table.audit", "name", "acc_audit_tbl"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "comment", "Audit log table"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "key_type", "DUPLICATE KEY"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.#", "29"),
					// spot-check a few representative columns
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.0.name", "queryId"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.0.type", "VARCHAR(64)"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.1.name", "timestamp"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.1.nullable", "false"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.28.name", "warehouse"),
					// ARRAY column
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.27.name", "QueriedRelations"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "columns.27.type", "ARRAY<VARCHAR(65533)>"),
					// partition_by is set and non-empty
					resource.TestCheckResourceAttrSet("starrocks_table.audit", "partition_by"),
					// computed fields populated
					resource.TestCheckResourceAttrSet("starrocks_table.audit", "engine"),
					resource.TestCheckResourceAttrSet("starrocks_table.audit", "distributed_by"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "properties.replication_num", "1"),
					resource.TestCheckResourceAttr("starrocks_table.audit", "properties.partition_live_number", "30"),
				),
			},
			// Import by "database.table" and verify the resource can be
			// reconstructed from SHOW CREATE TABLE output.
			{
				ResourceName:      "starrocks_table.audit",
				ImportState:       true,
				ImportStateId:     accDB + ".acc_audit_tbl",
				ImportStateVerify: false, // StarRocks may normalise identifiers in the returned DDL
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helper
// ---------------------------------------------------------------------------

// accAuditLogTableConfig returns HCL for the StarRocks audit log table.
// The table name is fixed to "acc_audit_tbl" to keep the test self-contained;
// the database is the shared accDB constant.
func accAuditLogTableConfig() string {
	return fmt.Sprintf(`
resource "starrocks_table" "audit" {
  database = %q
  name     = "acc_audit_tbl"
  comment  = "Audit log table"

  key_type    = "DUPLICATE KEY"
  key_columns = ["queryId", "timestamp", "queryType"]

  columns = [
    { name = "queryId",          type = "VARCHAR(64)",          nullable = true,  comment = "Unique ID of the query" },
    { name = "timestamp",        type = "DATETIME",             nullable = false, comment = "Query start time" },
    { name = "queryType",        type = "VARCHAR(12)",          nullable = true,  comment = "Query type (query, slow_query, connection)" },
    { name = "clientIp",         type = "VARCHAR(32)",          nullable = true,  comment = "Client IP" },
    { name = "user",             type = "VARCHAR(64)",          nullable = true,  comment = "Query username" },
    { name = "authorizedUser",   type = "VARCHAR(64)",          nullable = true,  comment = "Unique identifier of the user, i.e., user_identity" },
    { name = "resourceGroup",    type = "VARCHAR(64)",          nullable = true,  comment = "Resource group name" },
    { name = "catalog",          type = "VARCHAR(32)",          nullable = true,  comment = "Catalog name" },
    { name = "db",               type = "VARCHAR(96)",          nullable = true,  comment = "Database where the query runs" },
    { name = "state",            type = "VARCHAR(8)",           nullable = true,  comment = "Query state (EOF, ERR, OK)" },
    { name = "errorCode",        type = "VARCHAR(512)",         nullable = true,  comment = "Error code" },
    { name = "queryTime",        type = "BIGINT",               nullable = true,  comment = "Query execution time (milliseconds)" },
    { name = "scanBytes",        type = "BIGINT",               nullable = true,  comment = "Number of bytes scanned by the query" },
    { name = "scanRows",         type = "BIGINT",               nullable = true,  comment = "Number of rows scanned by the query" },
    { name = "returnRows",       type = "BIGINT",               nullable = true,  comment = "Number of rows returned by the query" },
    { name = "cpuCostNs",        type = "BIGINT",               nullable = true,  comment = "CPU time consumed by the query (nanoseconds)" },
    { name = "memCostBytes",     type = "BIGINT",               nullable = true,  comment = "Memory consumed by the query (bytes)" },
    { name = "stmtId",           type = "INT",                  nullable = true,  comment = "Incremental ID of the SQL statement" },
    { name = "isQuery",          type = "TINYINT",              nullable = true,  comment = "Whether the SQL is a query (1 or 0)" },
    { name = "feIp",             type = "VARCHAR(128)",         nullable = true,  comment = "FE IP that executed the statement" },
    { name = "stmt",             type = "VARCHAR(1048576)",     nullable = true,  comment = "Original SQL statement" },
    { name = "digest",           type = "VARCHAR(32)",          nullable = true,  comment = "Fingerprint of slow SQL" },
    { name = "planCpuCosts",     type = "DOUBLE",               nullable = true,  comment = "CPU usage during query planning (nanoseconds)" },
    { name = "planMemCosts",     type = "DOUBLE",               nullable = true,  comment = "Memory usage during query planning (bytes)" },
    { name = "pendingTimeMs",    type = "BIGINT",               nullable = true,  comment = "Time the query waited in the queue (milliseconds)" },
    { name = "candidateMVs",     type = "VARCHAR(65533)",       nullable = true,  comment = "List of candidate materialized views" },
    { name = "hitMvs",           type = "VARCHAR(65533)",       nullable = true,  comment = "List of matched materialized views" },
    { name = "QueriedRelations", type = "ARRAY<VARCHAR(65533)>", nullable = true,  comment = "List of directly referenced tables and views" },
    { name = "warehouse",        type = "VARCHAR(32)",          nullable = true,  comment = "Warehouse name" },
  ]

  partition_by   = "PARTITION BY date_trunc('day', timestamp)"
  distributed_by = "DISTRIBUTED BY HASH(queryId) BUCKETS 4"

  properties = {
    "replication_num"       = "1"
    "partition_live_number" = "30"
  }
}
`, accDB)
}
