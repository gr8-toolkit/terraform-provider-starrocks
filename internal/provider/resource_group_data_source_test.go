package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAcc_ResourceGroupDataSource_basic creates a resource group via the
// managed resource, reads it back through the data source, and verifies
// the numeric fields are populated and consistent.
func TestAcc_ResourceGroupDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accResourceGroupDataSourceConfig("acc_ds_rg", 2, "80.0%"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_resource_group.test", "name", "acc_ds_rg"),
					resource.TestCheckResourceAttrSet("data.starrocks_resource_group.test", "mem_limit"),
					// Data source mem_limit must match what the resource stored.
					resource.TestCheckResourceAttrPair(
						"data.starrocks_resource_group.test", "mem_limit",
						"starrocks_resource_group.test", "mem_limit",
					),
				),
			},
		},
	})
}

// TestAcc_ResourceGroupDataSource_bigQueryLimits creates a resource group with
// big query limits and verifies the data source reads them back correctly.
func TestAcc_ResourceGroupDataSource_bigQueryLimits(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accResourceGroupDataSourceWithBigQueryConfig("acc_ds_rg_bq"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_resource_group.test", "name", "acc_ds_rg_bq"),
					// big_query_* fields must be non-zero.
					resource.TestCheckResourceAttrSet("data.starrocks_resource_group.test", "big_query_mem_limit"),
					resource.TestCheckResourceAttrSet("data.starrocks_resource_group.test", "big_query_scan_rows_limit"),
					resource.TestCheckResourceAttrSet("data.starrocks_resource_group.test", "big_query_cpu_second_limit"),
					// Values must be consistent with the resource.
					resource.TestCheckResourceAttrPair(
						"data.starrocks_resource_group.test", "big_query_mem_limit",
						"starrocks_resource_group.test", "big_query_mem_limit",
					),
				),
			},
		},
	})
}

// TestAcc_ResourceGroupDataSource_notFound verifies that looking up a resource
// group that does not exist produces a clear error.
func TestAcc_ResourceGroupDataSource_notFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      accProviderBlock() + accResourceGroupDataSourceOnlyConfig("nonexistent_rg_xyz"),
				ExpectError: regexp.MustCompile(`[Rr]esource group not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accResourceGroupDataSourceConfig creates a resource group and a data source
// that depends on it.
func accResourceGroupDataSourceConfig(name string, cpuWeight int, memLimit string) string {
	return accResourceGroupBasicConfig(name, cpuWeight, memLimit) + fmt.Sprintf(`
data "starrocks_resource_group" "test" {
  name = starrocks_resource_group.test.name
}
`)
}

// accResourceGroupDataSourceWithBigQueryConfig creates a resource group that
// includes big query limits so the data source can verify those fields.
func accResourceGroupDataSourceWithBigQueryConfig(name string) string {
	return fmt.Sprintf(`
resource "starrocks_resource_group" "test" {
  name                      = %q
  cpu_weight                = 1
  mem_limit                 = "40.0%%"
  big_query_mem_limit       = 1073741824
  big_query_scan_rows_limit = 100000
  big_query_cpu_second_limit = 100

  classifiers = [
    { user = "root" }
  ]
}

data "starrocks_resource_group" "test" {
  name = starrocks_resource_group.test.name
}
`, name)
}

// accResourceGroupDataSourceOnlyConfig is a standalone data source with no
// managing resource — used to test the not-found error path.
func accResourceGroupDataSourceOnlyConfig(name string) string {
	return fmt.Sprintf(`
data "starrocks_resource_group" "test" {
  name = %q
}
`, name)
}
