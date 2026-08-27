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
	_ datasource.DataSource              = &pluginDataSource{}
	_ datasource.DataSourceWithConfigure = &pluginDataSource{}
)

// NewPluginDataSource returns a new starrocks_plugin data source implementation.
func NewPluginDataSource() datasource.DataSource {
	return &pluginDataSource{}
}

type pluginDataSource struct {
	client *client.Client
}

// pluginDataSourceModel is the Terraform state model for the starrocks_plugin
// data source.
type pluginDataSourceModel struct {
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	Version     types.String `tfsdk:"version"`
	Status      types.String `tfsdk:"status"`
}

func (d *pluginDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin"
}

func (d *pluginDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads metadata for an installed StarRocks plugin. " +
			"Useful for referencing plugin status in other resources or outputs without managing the plugin lifecycle.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The plugin name to look up, as it appears in `SHOW PLUGINS`.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The plugin type as reported by StarRocks (e.g. `AUDIT`).",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Plugin description as reported by StarRocks.",
			},
			"version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Plugin version as reported by StarRocks.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Plugin status as reported by StarRocks (e.g. `INSTALLED`).",
			},
		},
	}
}

// Read fetches the plugin via SHOW PLUGINS and populates state.
func (d *pluginDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state pluginDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := d.client.GetPlugin(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading plugin", err.Error())
		return
	}
	if p == nil {
		resp.Diagnostics.AddError(
			"Plugin not found",
			fmt.Sprintf("No plugin named %q exists in StarRocks.", state.Name.ValueString()),
		)
		return
	}

	state.Type = types.StringValue(p.Type)
	state.Description = types.StringValue(p.Description)
	state.Version = types.StringValue(p.Version)
	state.Status = types.StringValue(p.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *pluginDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
