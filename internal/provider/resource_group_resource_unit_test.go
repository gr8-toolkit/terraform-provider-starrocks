package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// resourceGroupResourceModel — getter methods
// ---------------------------------------------------------------------------

func TestResourceGroupModel_Getters(t *testing.T) {
	m := &resourceGroupResourceModel{
		Name:                   types.StringValue("test_rg"),
		CPUWeight:              types.Int64Value(10),
		ExclusiveCPUCores:      types.Int64Value(2),
		CPUCoreLimit:           types.Int64Value(8),
		MaxCPUCores:            types.Int64Value(16),
		MemLimit:               types.StringValue("80.0%"),
		ConcurrencyLimit:       types.Int64Value(5),
		BigQueryMemLimit:       types.Int64Value(1073741824),
		BigQueryScanRowsLimit:  types.Int64Value(100000),
		BigQueryCPUSecondLimit: types.Int64Value(100),
	}

	if m.GetName().ValueString() != "test_rg" {
		t.Errorf("GetName() = %q, want test_rg", m.GetName().ValueString())
	}
	if m.GetCPUWeight().ValueInt64() != 10 {
		t.Errorf("GetCPUWeight() = %d, want 10", m.GetCPUWeight().ValueInt64())
	}
	if m.GetExclusiveCPUCores().ValueInt64() != 2 {
		t.Errorf("GetExclusiveCPUCores() = %d, want 2", m.GetExclusiveCPUCores().ValueInt64())
	}
	if m.GetCPUCoreLimit().ValueInt64() != 8 {
		t.Errorf("GetCPUCoreLimit() = %d, want 8", m.GetCPUCoreLimit().ValueInt64())
	}
	if m.GetMaxCPUCores().ValueInt64() != 16 {
		t.Errorf("GetMaxCPUCores() = %d, want 16", m.GetMaxCPUCores().ValueInt64())
	}
	if m.GetMemLimit().ValueString() != "80.0%" {
		t.Errorf("GetMemLimit() = %q, want 80.0%%", m.GetMemLimit().ValueString())
	}
	if m.GetConcurrencyLimit().ValueInt64() != 5 {
		t.Errorf("GetConcurrencyLimit() = %d, want 5", m.GetConcurrencyLimit().ValueInt64())
	}
	if m.GetBigQueryMemLimit().ValueInt64() != 1073741824 {
		t.Errorf("GetBigQueryMemLimit() = %d, want 1073741824", m.GetBigQueryMemLimit().ValueInt64())
	}
	if m.GetBigQueryScanRowsLimit().ValueInt64() != 100000 {
		t.Errorf("GetBigQueryScanRowsLimit() = %d, want 100000", m.GetBigQueryScanRowsLimit().ValueInt64())
	}
	if m.GetBigQueryCPUSecondLimit().ValueInt64() != 100 {
		t.Errorf("GetBigQueryCPUSecondLimit() = %d, want 100", m.GetBigQueryCPUSecondLimit().ValueInt64())
	}
}

func TestResourceGroupModel_NullFields(t *testing.T) {
	// A model with only name set — all other fields should be null.
	m := &resourceGroupResourceModel{
		Name: types.StringValue("rg_sparse"),
	}

	if m.GetCPUWeight().IsNull() != true {
		t.Error("GetCPUWeight() should be null when unset")
	}
	if m.GetMemLimit().IsNull() != true {
		t.Error("GetMemLimit() should be null when unset")
	}
	if m.GetConcurrencyLimit().IsNull() != true {
		t.Error("GetConcurrencyLimit() should be null when unset")
	}
}

// ---------------------------------------------------------------------------
// isNotFoundError
// ---------------------------------------------------------------------------

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		// StarRocks 4.x error code 5079
		{"Error 5079 (42000): Getting analyzing error. Detail message: Unknown resource group 'acc_disappears' ", true},
		// Older variant
		{"resource group 'rg_test' does not exist", true},
		{"rg_test is not found", true},
		{"resource group not found", true},
		// Unrelated errors must not match
		{"connection refused", false},
		{"syntax error near 'GROUP'", false},
		{"table events does not exist", true}, // "does not exist" matches — acceptable false-positive
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := isNotFoundError(fmt.Errorf("%s", tt.msg))
			if got != tt.want {
				t.Errorf("isNotFoundError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}
