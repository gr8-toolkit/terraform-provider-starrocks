package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAcc_TableDataSource_basic creates a table via the managed resource, reads
// it back through the data source, and cross-checks all key fields.
func TestAcc_TableDataSource_basic(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accTableDataSourceConfig("acc_ds_table_basic"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_table.test", "database", accDB),
					resource.TestCheckResourceAttr("data.starrocks_table.test", "name", "acc_ds_table_basic"),
					resource.TestCheckResourceAttrSet("data.starrocks_table.test", "engine"),
					resource.TestCheckResourceAttrSet("data.starrocks_table.test", "key_type"),
					// columns list must be non-empty.
					resource.TestCheckResourceAttrSet("data.starrocks_table.test", "columns.#"),
					// Cross-check computed fields from the resource.
					resource.TestCheckResourceAttrPair(
						"data.starrocks_table.test", "engine",
						"starrocks_table.test", "engine",
					),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_table.test", "key_type",
						"starrocks_table.test", "key_type",
					),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_table.test", "columns.#",
						"starrocks_table.test", "columns.#",
					),
				),
			},
		},
	})
}

// TestAcc_TableDataSource_columnFields verifies that individual column
// attributes (name, type, nullable) are correctly surfaced by the data source.
func TestAcc_TableDataSource_columnFields(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accTableDataSourceConfig("acc_ds_table_cols"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_table.test", "columns.0.name", "id"),
					resource.TestCheckResourceAttr("data.starrocks_table.test", "columns.0.nullable", "false"),
					resource.TestCheckResourceAttr("data.starrocks_table.test", "columns.1.name", "payload"),
					resource.TestCheckResourceAttr("data.starrocks_table.test", "columns.1.nullable", "true"),
				),
			},
		},
	})
}

// TestAcc_TableDataSource_notFound verifies that looking up a table that does
// not exist produces a clear error.
func TestAcc_TableDataSource_notFound(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      accProviderBlock() + accTableDataSourceOnlyConfig(accDB, "nonexistent_table_xyz"),
				ExpectError: regexp.MustCompile(`Table not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accTableDataSourceConfig creates a table resource and reads it back via a
// data source in the same config, using accTableBasicConfig as the base.
func accTableDataSourceConfig(name string) string {
	return accTableBasicConfig(name) + fmt.Sprintf(`
data "starrocks_table" "test" {
  database = %q
  name     = starrocks_table.test.name
}
`, accDB)
}

// accTableDataSourceOnlyConfig is a standalone data source lookup with no
// managing resource — used to test the not-found error path.
func accTableDataSourceOnlyConfig(db, name string) string {
	return fmt.Sprintf(`
data "starrocks_table" "test" {
  database = %q
  name     = %q
}
`, db, name)
}
