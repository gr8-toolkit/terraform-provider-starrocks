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
	_ datasource.DataSource              = &globalVariableDataSource{}
	_ datasource.DataSourceWithConfigure = &globalVariableDataSource{}
)

func NewGlobalVariableDataSource() datasource.DataSource {
	return &globalVariableDataSource{}
}

type globalVariableDataSource struct {
	client *client.Client
}

type globalVariableDataSourceModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

func (d *globalVariableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_variable"
}

func (d *globalVariableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current value of a StarRocks global system variable. " +
			"Useful for referencing cluster-wide settings managed outside Terraform.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The variable name to look up (e.g. `query_timeout`).",
			},
			"value": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current value of the variable as reported by StarRocks.",
			},
		},
	}
}

func (d *globalVariableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state globalVariableDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	value, exists, err := d.client.GetGlobalVariable(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading global variable", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError(
			"Global variable not found",
			fmt.Sprintf("No global variable named %q exists in StarRocks.", state.Name.ValueString()),
		)
		return
	}

	state.Value = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *globalVariableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
