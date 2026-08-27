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
	_ datasource.DataSource              = &catalogDataSource{}
	_ datasource.DataSourceWithConfigure = &catalogDataSource{}
)

func NewCatalogDataSource() datasource.DataSource {
	return &catalogDataSource{}
}

type catalogDataSource struct {
	client *client.Client
}

type catalogDataSourceModel struct {
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Comment types.String `tfsdk:"comment"`
}

func (d *catalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog"
}

func (d *catalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads metadata for a StarRocks catalog. " +
			"Works for both external catalogs and the built-in `default_catalog`.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The catalog name to look up.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The catalog type as reported by StarRocks (e.g. `Internal`, `Hive`, `Iceberg`).",
			},
			"comment": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The catalog comment as reported by StarRocks.",
			},
		},
	}
}

func (d *catalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state catalogDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cat, err := d.client.GetCatalog(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading catalog", err.Error())
		return
	}
	if cat == nil {
		resp.Diagnostics.AddError(
			"Catalog not found",
			fmt.Sprintf("No catalog named %q exists in StarRocks.", state.Name.ValueString()),
		)
		return
	}

	state.Type = types.StringValue(cat.Type)
	state.Comment = cat.Comment

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *catalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
