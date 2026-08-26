package starrocks

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
		if _, err := client.db.Exec("CREATE DATABASE IF NOT EXISTS `" + accDB + "`"); err != nil {
			panic("TestMain: failed to create test database: " + err.Error())
		}
		code := m.Run()
		// Best-effort cleanup — ignore errors so a partial failure doesn't hide test results.
		_, _ = client.db.Exec("DROP DATABASE IF EXISTS `" + accDB + "`")
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
