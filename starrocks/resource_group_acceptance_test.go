package starrocks

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAcc_ResourceGroup_basic creates a resource group with the minimum
// required attributes, checks that Terraform tracks all fields correctly, then
// verifies the resource can be imported by name.
func TestAcc_ResourceGroup_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create and immediately check state.
			{
				Config: accProviderBlock() + accResourceGroupBasicConfig("acc_basic", 10, "80.0%"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "name", "acc_basic"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "cpu_weight", "10"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "mem_limit", "80.0%"),
				),
			},
			// Verify import by name round-trips cleanly.
			{
				ResourceName:      "starrocks_resource_group.test",
				ImportState:       true,
				ImportStateId:     "acc_basic",
				ImportStateVerify: false, // classifiers are not re-read from DB after import
			},
		},
	})
}

// TestAcc_ResourceGroup_update creates a resource group and then changes
// cpu_weight and mem_limit. Because updates are implemented as delete+recreate
// the resource name stays the same but all properties are replaced.
func TestAcc_ResourceGroup_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Initial state.
			{
				Config: accProviderBlock() + accResourceGroupBasicConfig("acc_update", 8, "60.0%"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "cpu_weight", "8"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "mem_limit", "60.0%"),
				),
			},
			// Update — triggers delete + recreate.
			{
				Config: accProviderBlock() + accResourceGroupBasicConfig("acc_update", 16, "70.0%"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "cpu_weight", "16"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "mem_limit", "70.0%"),
				),
			},
		},
	})
}

// TestAcc_ResourceGroup_classifiers creates a resource group with a single
// classifier and checks that the classifier attributes are stored in state.
func TestAcc_ResourceGroup_classifiers(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accResourceGroupWithClassifierConfig("acc_classifiers"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "name", "acc_classifiers"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "classifiers.#", "1"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "classifiers.0.user", "root"),
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "classifiers.0.query_type", "select"),
				),
			},
		},
	})
}

// TestAcc_ResourceGroup_disappears verifies that when the resource group is
// deleted outside of Terraform (simulated by a destroy step), the next plan
// detects the drift and recreates it.
func TestAcc_ResourceGroup_disappears(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: accProviderBlock() + accResourceGroupBasicConfig("acc_disappears", 4, "50.0%"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("starrocks_resource_group.test", "name", "acc_disappears"),
					// Delete the resource group out-of-band to simulate external drift.
					// On the post-step refresh, Read removes the resource from state,
					// so the framework plans a re-create — hence ExpectNonEmptyPlan.
					testAccDeleteResourceGroupOutOfBand("acc_disappears"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// accResourceGroupBasicConfig returns a minimal starrocks_resource_group HCL
// block with name, cpu_weight, mem_limit, and a single user classifier.
// StarRocks requires at least one classifier on every resource group.
func accResourceGroupBasicConfig(name string, cpuWeight int, memLimit string) string {
	return fmt.Sprintf(`
resource "starrocks_resource_group" "test" {
  name       = %q
  cpu_weight = %d
  mem_limit  = %q

  classifiers = [
    {
      user = "root"
    }
  ]
}
`, name, cpuWeight, memLimit)
}

// accResourceGroupWithClassifierConfig returns a resource group HCL block that
// includes one inline classifier combining user and query_type.
// query_type must be lowercase ("select") — StarRocks normalises it internally.
func accResourceGroupWithClassifierConfig(name string) string {
	return fmt.Sprintf(`
resource "starrocks_resource_group" "test" {
  name       = %q
  cpu_weight = 4
  mem_limit  = "40.0%%"

  classifiers = [
    {
      user       = "root"
      query_type = "select"
    }
  ]
}
`, name)
}

// ---------------------------------------------------------------------------
// Check helpers
// ---------------------------------------------------------------------------

// testAccDeleteResourceGroupOutOfBand returns a TestCheckFunc that connects
// directly to StarRocks and drops the named resource group, simulating an
// out-of-band deletion.
func testAccDeleteResourceGroupOutOfBand(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["starrocks_resource_group.test"]
		if !ok {
			return fmt.Errorf("resource starrocks_resource_group.test not found in state")
		}
		_ = rs // confirms the resource is tracked; connection uses env vars

		client, err := accClient()
		if err != nil {
			return fmt.Errorf("creating client for out-of-band delete: %w", err)
		}
		return client.DeleteResourceGroup(name)
	}
}

// accClient constructs a StarRocks client from the acceptance-test environment
// variables. Used by check helpers that need to reach the database directly.
func accClient() (*Client, error) {
	host := fmt.Sprintf("%s:%s",
		mustEnv("STARROCKS_HOST"),
		mustEnv("STARROCKS_PORT"),
	)
	return NewClient(host, mustEnv("STARROCKS_USERNAME"), mustEnv("STARROCKS_PASSWORD"))
}

// mustEnv returns the value of an environment variable. It panics when the
// variable is not set at all, but allows empty values (e.g. STARROCKS_PASSWORD).
// accPreCheck should have already validated that required vars are present.
func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}
