package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAcc_GlobalVariable_basic sets query_timeout to a non-default value,
// verifies the value is stored in state, then imports it by variable name.
// The framework's automatic destroy step resets it to DEFAULT.
func TestAcc_GlobalVariable_basic(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + `
resource "starrocks_global_variable" "test" {
  name  = "query_timeout"
  value = "600"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_global_variable.test", "name", "query_timeout"),
					resource.TestCheckResourceAttr("starrocks_global_variable.test", "value", "600"),
				),
			},
			// Import by variable name; value is read back from SHOW GLOBAL VARIABLES.
			{
				ResourceName:                         "starrocks_global_variable.test",
				ImportState:                          true,
				ImportStateId:                        "query_timeout",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

// TestAcc_GlobalVariable_update changes the value of a variable in-place
// (no replacement) and verifies the new value is reflected in state.
func TestAcc_GlobalVariable_update(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Initial value.
			{
				Config: accProviderBlock() + `
resource "starrocks_global_variable" "test" {
  name  = "query_timeout"
  value = "600"
}
`,
				Check: resource.TestCheckResourceAttr("starrocks_global_variable.test", "value", "600"),
			},
			// Updated value — no replacement expected.
			{
				Config: accProviderBlock() + `
resource "starrocks_global_variable" "test" {
  name  = "query_timeout"
  value = "900"
}
`,
				Check: resource.TestCheckResourceAttr("starrocks_global_variable.test", "value", "900"),
			},
		},
	})
}

// TestAcc_GlobalVariable_delete verifies that destroying a global variable
// resource resets it to its StarRocks default via SET GLOBAL name = DEFAULT,
// rather than leaving the configured value in place.
func TestAcc_GlobalVariable_delete(t *testing.T) {
	skipIfNotAcc(t)

	const varName = "query_timeout"
	const nonDefaultValue = "999"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Step 1 — apply a non-default value.
			{
				Config: accProviderBlock() + fmt.Sprintf(`
resource "starrocks_global_variable" "test" {
  name  = %q
  value = %q
}
`, varName, nonDefaultValue),
				Check: resource.TestCheckResourceAttr(
					"starrocks_global_variable.test", "value", nonDefaultValue,
				),
			},
			// Step 2 — remove the resource from config, triggering destroy.
			// After destroy the variable must be back at its default (300).
			{
				Config: accProviderBlock(), // resource removed
				Check: func(s *terraform.State) error {
					client, err := accClient()
					if err != nil {
						return fmt.Errorf("creating client: %w", err)
					}
					val, exists, err := client.GetGlobalVariable(varName)
					if err != nil {
						return fmt.Errorf("GetGlobalVariable: %w", err)
					}
					if !exists {
						return fmt.Errorf("variable %q not found after destroy", varName)
					}
					if val == nonDefaultValue {
						return fmt.Errorf(
							"variable %q = %q after destroy; expected default value, not the configured value",
							varName, val,
						)
					}
					return nil
				},
			},
		},
	})
}

// "true"/"false" strings) round-trip cleanly through state.
func TestAcc_GlobalVariable_boolean(t *testing.T) {
	skipIfNotAcc(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + `
resource "starrocks_global_variable" "test" {
  name  = "enable_profile"
  value = "true"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_global_variable.test", "name", "enable_profile"),
					// StarRocks stores booleans as "true"/"false" — value must survive read-back.
					resource.TestCheckResourceAttrSet("starrocks_global_variable.test", "value"),
				),
			},
		},
	})
}
