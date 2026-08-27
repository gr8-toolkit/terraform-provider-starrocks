package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAcc_GlobalVariableDataSource_basic sets a global variable via the managed
// resource, then reads it back through the data source. Verifies the values
// match via TestCheckResourceAttrPair.
func TestAcc_GlobalVariableDataSource_basic(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accGlobalVariableDataSourceConfig("query_timeout", "600"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_global_variable.test", "name", "query_timeout"),
					resource.TestCheckResourceAttrSet("data.starrocks_global_variable.test", "value"),
					// Data source value must match what the resource stored.
					resource.TestCheckResourceAttrPair(
						"data.starrocks_global_variable.test", "value",
						"starrocks_global_variable.test", "value",
					),
				),
			},
		},
	})
}

// TestAcc_GlobalVariableDataSource_alwaysExists reads a system variable that
// is guaranteed to exist in every StarRocks instance without managing it as a
// resource. Verifies the data source can read pre-existing system variables.
func TestAcc_GlobalVariableDataSource_alwaysExists(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + `
data "starrocks_global_variable" "test" {
  name = "query_timeout"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_global_variable.test", "name", "query_timeout"),
					resource.TestCheckResourceAttrSet("data.starrocks_global_variable.test", "value"),
				),
			},
		},
	})
}

// TestAcc_GlobalVariableDataSource_notFound verifies that looking up a variable
// that does not exist produces a clear error.
func TestAcc_GlobalVariableDataSource_notFound(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      accProviderBlock() + accGlobalVariableDataSourceOnlyConfig("nonexistent_variable_xyz"),
				ExpectError: regexp.MustCompile(`Global variable not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accGlobalVariableDataSourceConfig sets a global variable via a managed
// resource and reads it back via a data source.
func accGlobalVariableDataSourceConfig(name, value string) string {
	return fmt.Sprintf(`
resource "starrocks_global_variable" "test" {
  name  = %q
  value = %q
}

data "starrocks_global_variable" "test" {
  name = starrocks_global_variable.test.name
}
`, name, value)
}

// accGlobalVariableDataSourceOnlyConfig is a standalone data source lookup
// with no managing resource — used to test the not-found error path.
func accGlobalVariableDataSourceOnlyConfig(name string) string {
	return fmt.Sprintf(`
data "starrocks_global_variable" "test" {
  name = %q
}
`, name)
}
