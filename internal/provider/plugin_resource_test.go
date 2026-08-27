package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ---------------------------------------------------------------------------
// Unit tests — no live DB required
// ---------------------------------------------------------------------------

func TestIsPluginNotFoundError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"Error 1064: Unknown plugin 'auditdemo'", true},
		{"plugin auditdemo not found", true},
		{"plugin does not exist", true},
		{"connection refused", false},
		{"syntax error near PLUGIN", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := isPluginNotFoundError(fmt.Errorf("%s", tt.msg))
			if got != tt.want {
				t.Errorf("isPluginNotFoundError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestBuildInstallPluginSQL_noProperties(t *testing.T) {
	got := buildInstallPluginSQL("/path/to/plugin.zip", nil)
	want := `INSTALL PLUGIN FROM "/path/to/plugin.zip"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildInstallPluginSQL_withProperties(t *testing.T) {
	got := buildInstallPluginSQL("http://example.com/plugin.zip", map[string]string{
		"md5sum": "abc123",
	})
	for _, fragment := range []string{
		`INSTALL PLUGIN FROM "http://example.com/plugin.zip"`,
		"PROPERTIES",
		`"md5sum" = "abc123"`,
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("SQL missing %q\nSQL: %s", fragment, got)
		}
	}
}

// TestBuildInstallPluginSQL_propertiesSorted verifies that multiple properties
// are emitted in sorted key order so the SQL is deterministic.
func TestBuildInstallPluginSQL_propertiesSorted(t *testing.T) {
	got := buildInstallPluginSQL("/p.zip", map[string]string{
		"zoo": "last",
		"aaa": "first",
		"mmm": "middle",
	})
	aaaPos := strings.Index(got, `"aaa"`)
	mmmPos := strings.Index(got, `"mmm"`)
	zooPos := strings.Index(got, `"zoo"`)
	if aaaPos < 0 || mmmPos < 0 || zooPos < 0 {
		t.Fatalf("not all keys present in SQL: %s", got)
	}
	if !(aaaPos < mmmPos && mmmPos < zooPos) {
		t.Errorf("properties not sorted: aaa@%d mmm@%d zoo@%d\nSQL: %s", aaaPos, mmmPos, zooPos, got)
	}
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a live StarRocks instance
// ---------------------------------------------------------------------------

// skipIfNoPluginSource skips the test when STARROCKS_PLUGIN_SOURCE is not set.
// Plugin install tests require a reachable plugin zip (local path or URL).
// In offline/CI environments without network access or a local zip, set this
// variable to enable them:
//
//	STARROCKS_PLUGIN_SOURCE=/path/to/AuditLoader.zip make testacc
func skipIfNoPluginSource(t *testing.T) string {
	t.Helper()
	src, ok := os.LookupEnv("STARROCKS_PLUGIN_SOURCE")
	if !ok || src == "" {
		t.Skip("skipping plugin install test: STARROCKS_PLUGIN_SOURCE not set")
	}
	return src
}

// TestAcc_Plugin_basic installs a plugin from the source given in
// STARROCKS_PLUGIN_SOURCE, verifies computed fields, then imports by name.
// Skipped when STARROCKS_PLUGIN_SOURCE is unset.
func TestAcc_Plugin_basic(t *testing.T) {
	skipIfNotAcc(t)
	source := skipIfNoPluginSource(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accPluginConfig("AuditLoader", source),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_plugin.test", "name", "AuditLoader"),
					resource.TestCheckResourceAttrSet("starrocks_plugin.test", "type"),
					resource.TestCheckResourceAttrSet("starrocks_plugin.test", "status"),
				),
			},
			{
				ResourceName:      "starrocks_plugin.test",
				ImportState:       true,
				ImportStateId:     "AuditLoader",
				ImportStateVerify: false,
			},
		},
	})
}

// TestAcc_Plugin_disappears verifies that when a plugin is uninstalled outside
// Terraform, Read removes it from state so the next plan proposes a re-install.
// Skipped when STARROCKS_PLUGIN_SOURCE is unset.
func TestAcc_Plugin_disappears(t *testing.T) {
	skipIfNotAcc(t)
	source := skipIfNoPluginSource(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accPluginConfig("AuditLoader", source),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_plugin.test", "name", "AuditLoader"),
					testAccUninstallPluginOutOfBand("AuditLoader"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAcc_Plugin_auditLoader installs the AuditLoader plugin from the
// official StarRocks release URL (no STARROCKS_PLUGIN_SOURCE required).
// It verifies that:
//   - computed fields are populated after install (no unknown-value error)
//   - properties defaults to an empty map when omitted from config
//   - an import round-trip succeeds
//
// This test requires network access to releases.starrocks.io.
func TestAcc_Plugin_auditLoader(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accPluginAuditLoaderConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_plugin.audit", "name", "AuditLoader"),
					resource.TestCheckResourceAttr("starrocks_plugin.audit", "source", "https://releases.starrocks.io/resources/AuditLoader.zip"),
					// computed fields must be known (not unknown) after apply
					resource.TestCheckResourceAttrSet("starrocks_plugin.audit", "type"),
					resource.TestCheckResourceAttrSet("starrocks_plugin.audit", "status"),
					resource.TestCheckResourceAttrSet("starrocks_plugin.audit", "version"),
					// properties omitted in config — must resolve to empty map, not unknown
					resource.TestCheckResourceAttr("starrocks_plugin.audit", "properties.%", "0"),
				),
			},
			{
				ResourceName:      "starrocks_plugin.audit",
				ImportState:       true,
				ImportStateId:     "AuditLoader",
				ImportStateVerify: false, // source/properties are not recoverable from SHOW PLUGINS
			},
		},
	})
}

// TestAcc_Plugin_importNoReplace verifies the import-then-apply behaviour
// introduced by pluginSourceRequiresReplace and pluginPropertiesRequiresReplace.
//
// Scenario: a plugin is managed by Terraform, then imported fresh (simulating
// a hand-installed plugin brought under Terraform management). After import the
// state holds sentinel values for source ("") and properties ({}). The very
// next apply with the real config must produce an empty diff — it must NOT
// trigger a destroy+recreate of the already-running plugin.
func TestAcc_Plugin_importNoReplace(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — install the plugin so it exists in StarRocks.
			{
				Config: accProviderBlock() + accPluginAuditLoaderConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_plugin.audit", "name", "AuditLoader"),
					resource.TestCheckResourceAttrSet("starrocks_plugin.audit", "status"),
					resource.TestCheckResourceAttrSet("starrocks_plugin.audit", "version"),
				),
			},
			// Step 2 — import by name; this sets source="" and properties={}
			// in state, simulating `terraform import starrocks_plugin.audit AuditLoader`.
			{
				ResourceName:      "starrocks_plugin.audit",
				ImportState:       true,
				ImportStateId:     "AuditLoader",
				ImportStateVerify: false,
			},
			// Step 3 — re-apply the original config. The plan must be empty:
			// source and properties must NOT force replacement even though the
			// imported state holds sentinel values different from the config.
			// ExpectNonEmptyPlan defaults to false, so this step fails the test
			// if Terraform proposes any changes (including a replace).
			{
				Config:             accProviderBlock() + accPluginAuditLoaderConfig(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// accPluginAuditLoaderConfig returns the HCL for the AuditLoader plugin
// using the official release URL with no properties block.
func accPluginAuditLoaderConfig() string {
	return `
resource "starrocks_plugin" "audit" {
  name   = "AuditLoader"
  source = "https://releases.starrocks.io/resources/AuditLoader.zip"
}
`
}

// accPluginConfig returns an HCL block for a starrocks_plugin resource.
// source must be a reachable path or URL — set via STARROCKS_PLUGIN_SOURCE.
func accPluginConfig(name, source string) string {
	return fmt.Sprintf(`
resource "starrocks_plugin" "test" {
  name   = %q
  source = %q
}
`, name, source)
}

// ---------------------------------------------------------------------------
// Check helpers
// ---------------------------------------------------------------------------

func testAccUninstallPluginOutOfBand(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client for out-of-band plugin uninstall: %w", err)
		}
		return c.UninstallPlugin(name)
	}
}
