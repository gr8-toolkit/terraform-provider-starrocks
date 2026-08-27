package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a live StarRocks instance
// ---------------------------------------------------------------------------

// TestAcc_PluginDataSource_builtin reads back the always-present built-in
// audit plugin without installing anything. This test works in every
// environment — no STARROCKS_PLUGIN_SOURCE is required.
func TestAcc_PluginDataSource_builtin(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accPluginDataSourceOnlyConfig("__builtin_AuditLogBuilder"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_plugin.test", "name", "__builtin_AuditLogBuilder"),
					resource.TestCheckResourceAttr("data.starrocks_plugin.test", "type", "AUDIT"),
					resource.TestCheckResourceAttr("data.starrocks_plugin.test", "status", "INSTALLED"),
					resource.TestCheckResourceAttrSet("data.starrocks_plugin.test", "version"),
				),
			},
		},
	})
}

// TestAcc_PluginDataSource_withResource installs a plugin via the managed
// resource and reads it back through the data source, verifying field
// consistency. Skipped when STARROCKS_PLUGIN_SOURCE is not set.
func TestAcc_PluginDataSource_withResource(t *testing.T) {
	skipIfNotAcc(t)
	source := skipIfNoPluginSource(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accPluginDataSourceWithResourceConfig("AuditLoader", source),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.starrocks_plugin.test", "name", "AuditLoader"),
					resource.TestCheckResourceAttrSet("data.starrocks_plugin.test", "type"),
					resource.TestCheckResourceAttrSet("data.starrocks_plugin.test", "status"),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_plugin.test", "type",
						"starrocks_plugin.test", "type",
					),
					resource.TestCheckResourceAttrPair(
						"data.starrocks_plugin.test", "status",
						"starrocks_plugin.test", "status",
					),
				),
			},
		},
	})
}

// TestAcc_PluginDataSource_notFound verifies that reading a non-existent plugin
// produces a clear error rather than a panic or empty state.
func TestAcc_PluginDataSource_notFound(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      accProviderBlock() + accPluginDataSourceOnlyConfig("nonexistent_plugin_xyz"),
				ExpectError: regexp.MustCompile(`Plugin not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accPluginDataSourceWithResourceConfig installs a plugin via a managed
// resource and reads it back via a data source in the same config.
func accPluginDataSourceWithResourceConfig(name, source string) string {
	return fmt.Sprintf(`
resource "starrocks_plugin" "test" {
  name   = %q
  source = %q
}

data "starrocks_plugin" "test" {
  name = starrocks_plugin.test.name
}
`, name, source)
}

// accPluginDataSourceOnlyConfig returns a standalone data source block with no
// managing resource.
func accPluginDataSourceOnlyConfig(name string) string {
	return fmt.Sprintf(`
data "starrocks_plugin" "test" {
  name = %q
}
`, name)
}
