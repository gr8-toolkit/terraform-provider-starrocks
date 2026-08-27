package provider

import (
	"context"
	"fmt"

	"github.com/gr8-toolkit/terraform-provider-starrocks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &resourceGroupDataSource{}
	_ datasource.DataSourceWithConfigure = &resourceGroupDataSource{}
)

func NewResourceGroupDataSource() datasource.DataSource {
	return &resourceGroupDataSource{}
}

type resourceGroupDataSource struct {
	client *client.Client
}

type resourceGroupDataSourceModel struct {
	Name                   types.String `tfsdk:"name"`
	MemLimit               types.String `tfsdk:"mem_limit"`
	ConcurrencyLimit       types.Int64  `tfsdk:"concurrency_limit"`
	BigQueryMemLimit       types.Int64  `tfsdk:"big_query_mem_limit"`
	BigQueryScanRowsLimit  types.Int64  `tfsdk:"big_query_scan_rows_limit"`
	BigQueryCPUSecondLimit types.Int64  `tfsdk:"big_query_cpu_second_limit"`
}

func (d *resourceGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_group"
}

func (d *resourceGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads metadata for a StarRocks resource group via `SHOW RESOURCE GROUP`. " +
			"Note: `cpu_weight`, `exclusive_cpu_cores`, `cpu_core_limit`, `max_cpu_cores`, and classifiers " +
			"are not available through `SHOW RESOURCE GROUP` without a specific parsing layer, " +
			"so only the fields StarRocks returns numerically are exposed here.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The resource group name to look up.",
			},
			"mem_limit": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Memory limit as reported by StarRocks (e.g. `\"80.0%\"`).",
			},
			"concurrency_limit": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum query concurrency for the resource group.",
			},
			"big_query_mem_limit": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Memory limit in bytes for big queries.",
			},
			"big_query_scan_rows_limit": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum scan rows for big queries.",
			},
			"big_query_cpu_second_limit": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum CPU seconds for big queries.",
			},
		},
	}
}

func (d *resourceGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state resourceGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rg, err := d.client.GetResourceGroup(state.Name.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddError(
				"Resource group not found",
				fmt.Sprintf("No resource group named %q exists in StarRocks.", state.Name.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading resource group", err.Error())
		return
	}
	// GetResourceGroup returns a non-nil struct with just the name when the
	// query returns zero rows (i.e. the group does not exist).
	if rg == nil || rg.Name.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Resource group not found",
			fmt.Sprintf("No resource group named %q exists in StarRocks.", state.Name.ValueString()),
		)
		return
	}

	state.MemLimit = rg.MemLimit
	state.ConcurrencyLimit = rg.ConcurrencyLimit
	state.BigQueryMemLimit = rg.BigQueryMemLimit
	state.BigQueryScanRowsLimit = rg.BigQueryScanRowsLimit
	state.BigQueryCPUSecondLimit = rg.BigQueryCPUSecondLimit

	// Null fields (zero-value not set by server) become 0 in state so
	// consumers always get a concrete value rather than null.
	if state.ConcurrencyLimit.IsNull() {
		state.ConcurrencyLimit = types.Int64Value(0)
	}
	if state.BigQueryMemLimit.IsNull() {
		state.BigQueryMemLimit = types.Int64Value(0)
	}
	if state.BigQueryScanRowsLimit.IsNull() {
		state.BigQueryScanRowsLimit = types.Int64Value(0)
	}
	if state.BigQueryCPUSecondLimit.IsNull() {
		state.BigQueryCPUSecondLimit = types.Int64Value(0)
	}
	if state.MemLimit.IsNull() {
		state.MemLimit = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *resourceGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected datasource configure type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}
	d.client = c
}
