package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// providerFactories returns a map of provider factories for use in acceptance
// tests. It uses providerserver.NewProtocol6WithError so that the framework
// serves the provider over protocol version 6, matching the manifest entry.
//
// Tests that call resource.Test / resource.UnitTest must pass this map as
// resource.TestCase.ProtoV6ProviderFactories.
var providerFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"starrocks": providerserver.NewProtocol6WithError(New("test")()),
}

// accPreCheck validates that the environment variables required to run
// acceptance tests are present. Call it inside resource.TestCase.PreCheck.
//
// Required (must be set, non-empty):
//
//	STARROCKS_HOST      – StarRocks host (e.g. "127.0.0.1")
//	STARROCKS_PORT      – StarRocks MySQL port (e.g. "9030")
//	STARROCKS_USERNAME  – login user
//
// Optional (empty string is valid, but the variable must exist in the env):
//
//	STARROCKS_PASSWORD  – login password (default root has no password)
func accPreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		"STARROCKS_HOST",
		"STARROCKS_PORT",
		"STARROCKS_USERNAME",
	} {
		if os.Getenv(v) == "" {
			t.Fatalf("acceptance test requires environment variable %s to be set", v)
		}
	}
	if _, ok := os.LookupEnv("STARROCKS_PASSWORD"); !ok {
		t.Fatal("acceptance test requires environment variable STARROCKS_PASSWORD to be set (use empty string for no password)")
	}
}

// accProviderBlock returns a provider configuration block populated from
// environment variables. Panics if called before accPreCheck has validated the
// variables — call accPreCheck first inside resource.TestCase.PreCheck.
func accProviderBlock() string {
	return fmt.Sprintf(`
provider "starrocks" {
  host     = %q
  port     = %s
  username = %q
  password = %q
}
`,
		os.Getenv("STARROCKS_HOST"),
		os.Getenv("STARROCKS_PORT"),
		os.Getenv("STARROCKS_USERNAME"),
		os.Getenv("STARROCKS_PASSWORD"),
	)
}

// skipIfNotAcc is a convenience helper that skips a test when TF_ACC is unset.
// resource.Test already performs this check automatically, but tests using
// resource.UnitTest do not — use this helper there.
func skipIfNotAcc(t *testing.T) {
	t.Helper()
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("set %s=1 to run acceptance tests", resource.EnvTfAcc)
	}
}
