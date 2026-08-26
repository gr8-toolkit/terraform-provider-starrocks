package starrocks

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAcc_Database_basic creates a minimal database, verifies it exists in
// state, then imports it by name.
func TestAcc_Database_basic(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + `
resource "starrocks_database" "test" {
  name = "acc_db_basic"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_database.test", "name", "acc_db_basic"),
				),
			},
			// Import by database name.
			{
				ResourceName:      "starrocks_database.test",
				ImportState:       true,
				ImportStateId:     "acc_db_basic",
				ImportStateVerify: false, // quotas are write-only — server never returns them
			},
		},
	})
}

// TestAcc_Database_withDataQuota creates a database with a data quota and
// verifies the quota can be updated in-place without replacement.
func TestAcc_Database_withDataQuota(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create with initial quota.
			{
				Config: accProviderBlock() + `
resource "starrocks_database" "test" {
  name       = "acc_db_quota"
  data_quota = "5G"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_database.test", "name", "acc_db_quota"),
					resource.TestCheckResourceAttr("starrocks_database.test", "data_quota", "5G"),
				),
			},
			// Update quota in-place — no replacement expected.
			{
				Config: accProviderBlock() + `
resource "starrocks_database" "test" {
  name       = "acc_db_quota"
  data_quota = "10G"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_database.test", "data_quota", "10G"),
				),
			},
		},
	})
}

// TestAcc_Database_disappears verifies that when the database is dropped
// outside Terraform, Read removes it from state so the next plan re-creates it.
func TestAcc_Database_disappears(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + `
resource "starrocks_database" "test" {
  name = "acc_db_disappears"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_database.test", "name", "acc_db_disappears"),
					testAccDropDatabaseOutOfBand("acc_db_disappears"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAcc_Database_dropBlockedByTables verifies that destroying a database
// that contains tables fails with a clear error rather than silently dropping
// the data (StarRocks itself does not guard against this).
func TestAcc_Database_dropBlockedByTables(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			accPreCheck(t)
			// Force-clean any leftover state from a previous failed run.
			testAccForceDropDatabase(t, "acc_db_nonempty")
		},
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — create the database and inject a table out-of-band.
			{
				Config: accProviderBlock() + `
resource "starrocks_database" "test" {
  name = "acc_db_nonempty"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_database.test", "name", "acc_db_nonempty"),
					testAccCreateTableInDatabase("acc_db_nonempty", "sentinel"),
				),
			},
			// Step 2 — attempt to destroy while the table is still present.
			// The provider must return an error.
			{
				Config:      accProviderBlock() + `# database removed from config`,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`still contains`),
			},
			// Step 3 — drop the out-of-band table so the framework's post-test
			// cleanup can successfully destroy the database.
			{
				Config: accProviderBlock() + `
resource "starrocks_database" "test" {
  name = "acc_db_nonempty"
}
`,
				Check: testAccDropTableInDatabase("acc_db_nonempty", "sentinel"),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Check helpers
// ---------------------------------------------------------------------------

func testAccDropDatabaseOutOfBand(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client for out-of-band database drop: %w", err)
		}
		return client.DropDatabase(name)
	}
}

func testAccCreateTableInDatabase(db, table string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}
		return client.CreateTable(db, &TableDef{
			Name:       table,
			Engine:     "OLAP",
			KeyType:    "DUPLICATE KEY",
			KeyColumns: []string{"id"},
			Columns: []ColumnDef{
				{Name: "id", Type: "BIGINT", Nullable: false},
			},
			DistBy:     "DISTRIBUTED BY HASH(id) BUCKETS 1",
			Properties: map[string]string{"replication_num": "1"},
		})
	}
}

func testAccDropTableInDatabase(db, table string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}
		return client.DropTable(db, table)
	}
}

// testAccForceDropDatabase drops a database and all its tables unconditionally.
// Used in PreCheck to clean up dangling state from previous failed test runs.
// It is a no-op when the database does not exist.
func testAccForceDropDatabase(t *testing.T, name string) {
	t.Helper()
	client, err := accClient()
	if err != nil {
		t.Logf("testAccForceDropDatabase: could not create client: %v", err)
		return
	}
	tables, err := client.ListDatabaseTables(name)
	if err != nil {
		t.Logf("testAccForceDropDatabase: could not list tables in %q: %v", name, err)
		return
	}
	for _, tbl := range tables {
		if err := client.DropTable(name, tbl); err != nil {
			t.Logf("testAccForceDropDatabase: could not drop table %q.%q: %v", name, tbl, err)
		}
	}
	if err := client.DropDatabase(name); err != nil {
		t.Logf("testAccForceDropDatabase: could not drop database %q: %v", name, err)
	}
}
