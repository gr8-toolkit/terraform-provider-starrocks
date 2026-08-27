package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAcc_DatabaseDataSource_exists creates a database via the managed
// resource, then reads it back through the data source and verifies
// exists = true and the name attribute is correct.
func TestAcc_DatabaseDataSource_exists(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accDatabaseDataSourceConfig("acc_ds_db"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_database.test", "name", "acc_ds_db"),
					resource.TestCheckResourceAttr("data.starrocks_database.test", "exists", "true"),
				),
			},
		},
	})
}

// TestAcc_DatabaseDataSource_missing verifies that looking up a database that
// does not exist returns exists = false rather than an error.
func TestAcc_DatabaseDataSource_missing(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accDatabaseDataSourceOnlyConfig("nonexistent_db_xyz_acc"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_database.test", "exists", "false"),
				),
			},
		},
	})
}

// TestAcc_DatabaseDataSource_outOfBandDrop creates a database, drops it
// out-of-band (simulating external deletion), then verifies the data source
// returns exists = false on the next apply.
func TestAcc_DatabaseDataSource_outOfBandDrop(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — database is managed; data source sees it.
			{
				Config: accProviderBlock() + accDatabaseDataSourceConfig("acc_ds_db_oob"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_database.test", "exists", "true"),
					// Drop the database out-of-band so the next refresh sees it gone.
					testAccDropDatabaseOutOfBand("acc_ds_db_oob"),
				),
				ExpectNonEmptyPlan: true,
			},
			// Step 2 — data source only, no managed resource.
			// The database no longer exists, so exists must be false.
			{
				Config: accProviderBlock() + accDatabaseDataSourceOnlyConfig("acc_ds_db_oob"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_database.test", "exists", "false"),
				),
			},
		},
	})
}

// TestAcc_DatabaseDataSource_nameRequired verifies that omitting the required
// name attribute produces a plan-time error.
func TestAcc_DatabaseDataSource_nameRequired(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      accProviderBlock() + `data "starrocks_database" "test" {}`,
				ExpectError: regexp.MustCompile(`The argument "name" is required`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accDatabaseDataSourceConfig creates a managed database resource and a data
// source that depends on it.
func accDatabaseDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "starrocks_database" "test" {
  name = %q
}

data "starrocks_database" "test" {
  name = starrocks_database.test.name
}
`, name)
}

// accDatabaseDataSourceOnlyConfig returns a standalone data source lookup with
// no managing resource.
func accDatabaseDataSourceOnlyConfig(name string) string {
	return fmt.Sprintf(`
data "starrocks_database" "test" {
  name = %q
}
`, name)
}
