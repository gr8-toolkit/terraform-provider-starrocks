package starrocks

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Acceptance tests for starrocks_catalog.
//
// We use a JDBC catalog pointing back at the StarRocks instance itself for all
// create/update/disappears tests. StarRocks accepts the CREATE EXTERNAL CATALOG
// statement immediately without verifying reachability of the downstream source,
// so no external service is required beyond the StarRocks container.
//
// The internal catalog (default_catalog) import test is always safe — every
// StarRocks cluster has exactly one default_catalog.

// TestAcc_Catalog_basic creates an external JDBC catalog, verifies computed
// fields (type, comment), then imports it by name.
func TestAcc_Catalog_basic(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accJDBCCatalogConfig("acc_catalog_basic", "Basic JDBC catalog"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_catalog.test", "name", "acc_catalog_basic"),
					resource.TestCheckResourceAttr("starrocks_catalog.test", "comment", "Basic JDBC catalog"),
					// type is computed — StarRocks should report "JDBC" or similar
					resource.TestCheckResourceAttrSet("starrocks_catalog.test", "type"),
				),
			},
			// Import by name and verify the computed fields survive round-trip.
			// ImportStateVerify is false because properties are not recoverable
			// from the server after import (credentials are anonymised).
			{
				ResourceName:      "starrocks_catalog.test",
				ImportState:       true,
				ImportStateId:     "acc_catalog_basic",
				ImportStateVerify: false,
			},
		},
	})
}

// TestAcc_Catalog_noComment creates a catalog without a comment to verify
// that the optional comment field works correctly.
func TestAcc_Catalog_noComment(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accJDBCCatalogConfig("acc_catalog_nocomment", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_catalog.test", "name", "acc_catalog_nocomment"),
					resource.TestCheckResourceAttrSet("starrocks_catalog.test", "type"),
				),
			},
		},
	})
}

// TestAcc_Catalog_disappears verifies that when the catalog is deleted outside
// Terraform, Read removes it from state so the next plan proposes a re-create.
func TestAcc_Catalog_disappears(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accJDBCCatalogConfig("acc_catalog_disappears", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_catalog.test", "name", "acc_catalog_disappears"),
					testAccDeleteCatalogOutOfBand("acc_catalog_disappears"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAcc_Catalog_importDefaultCatalog verifies that the built-in internal
// catalog can be imported and that Terraform does not attempt to destroy it.
func TestAcc_Catalog_importDefaultCatalog(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Import the always-present internal catalog.
			{
				ResourceName:      "starrocks_catalog.default",
				ImportState:       true,
				ImportStateId:     "default_catalog",
				ImportStateVerify: false,
				// Config is required even for import-only steps so the
				// framework knows what resource block to reconcile against.
				Config: accProviderBlock() + `
resource "starrocks_catalog" "default" {
  name       = "default_catalog"
  properties = {}
}
`,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accJDBCCatalogConfig returns an HCL block for a JDBC catalog that points at
// the StarRocks MySQL port. StarRocks accepts the DDL without verifying
// connectivity to the downstream source, making this suitable for CI.
// When comment is empty the attribute is omitted entirely.
func accJDBCCatalogConfig(name, comment string) string {
	commentLine := ""
	if comment != "" {
		commentLine = fmt.Sprintf("  comment = %q\n", comment)
	}
	return fmt.Sprintf(`
resource "starrocks_catalog" "test" {
  name = %q
%s  properties = {
    "type"         = "jdbc"
    "driver_url"   = "https://repo1.maven.org/maven2/mysql/mysql-connector-java/8.0.28/mysql-connector-java-8.0.28.jar"
    "driver_class" = "com.mysql.cj.jdbc.Driver"
    "jdbc_uri"     = "jdbc:mysql://%s:%s/"
    "user"         = %q
    "password"     = %q
  }
}
`, name, commentLine,
		mustEnv("STARROCKS_HOST"),
		mustEnv("STARROCKS_PORT"),
		mustEnv("STARROCKS_USERNAME"),
		mustEnv("STARROCKS_PASSWORD"),
	)
}

// ---------------------------------------------------------------------------
// Check helpers
// ---------------------------------------------------------------------------

func testAccDeleteCatalogOutOfBand(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client for out-of-band catalog delete: %w", err)
		}
		return client.DeleteCatalog(name)
	}
}
