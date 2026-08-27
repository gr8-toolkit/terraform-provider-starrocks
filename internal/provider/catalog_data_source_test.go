package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAcc_CatalogDataSource_externalCatalog creates a JDBC catalog via the
// managed resource, then reads it back through the data source and verifies
// the computed fields match.
func TestAcc_CatalogDataSource_externalCatalog(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accCatalogDataSourceConfig("acc_ds_catalog"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_catalog.test", "name", "acc_ds_catalog"),
					resource.TestCheckResourceAttrSet("data.starrocks_catalog.test", "type"),
					// Fields from the managed resource must match the data source.
					resource.TestCheckResourceAttrPair(
						"data.starrocks_catalog.test", "type",
						"starrocks_catalog.test", "type",
					),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_catalog.test", "comment",
						"starrocks_catalog.test", "comment",
					),
				),
			},
		},
	})
}

// TestAcc_CatalogDataSource_defaultCatalog reads the always-present internal
// catalog without first creating a managed resource.
func TestAcc_CatalogDataSource_defaultCatalog(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + `
data "starrocks_catalog" "default" {
  name = "default_catalog"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_catalog.default", "name", "default_catalog"),
					resource.TestCheckResourceAttr("data.starrocks_catalog.default", "type", "Internal"),
				),
			},
		},
	})
}

// TestAcc_CatalogDataSource_notFound verifies that looking up a non-existent
// catalog produces a clear error.
func TestAcc_CatalogDataSource_notFound(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      accProviderBlock() + accCatalogDataSourceOnlyConfig("nonexistent_catalog_xyz"),
				ExpectError: regexp.MustCompile(`Catalog not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accCatalogDataSourceConfig creates a JDBC catalog resource and reads it back
// via a data source in the same config.
func accCatalogDataSourceConfig(name string) string {
	return accJDBCCatalogConfig(name, "DS test catalog") + fmt.Sprintf(`
data "starrocks_catalog" "test" {
  name = starrocks_catalog.test.name
}
`)
}

// accCatalogDataSourceOnlyConfig returns an HCL block for a data source lookup
// with no managing resource — used to test the not-found error path.
func accCatalogDataSourceOnlyConfig(name string) string {
	return fmt.Sprintf(`
data "starrocks_catalog" "test" {
  name = %q
}
`, name)
}
