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
	_ datasource.DataSource              = &indexDataSource{}
	_ datasource.DataSourceWithConfigure = &indexDataSource{}
)

func NewIndexDataSource() datasource.DataSource {
	return &indexDataSource{}
}

type indexDataSource struct {
	client *client.Client
}

type indexDataSourceModel struct {
	Database types.String `tfsdk:"database"`
	Table    types.String `tfsdk:"table"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Column   types.String `tfsdk:"column"`
	Comment  types.String `tfsdk:"comment"`
}

func (d *indexDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index"
}

func (d *indexDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads metadata for a StarRocks secondary index. " +
			"Identifies the index by database, table, and index name.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database that contains the table.",
			},
			"table": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The table the index belongs to.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The index name.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The index type (e.g. `BITMAP`, `NGRAMBF`).",
			},
			"column": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The column the index is defined on.",
			},
			"comment": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Index comment.",
			},
		},
	}
}

func (d *indexDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state indexDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := d.client.GetIndex(
		state.Database.ValueString(),
		state.Table.ValueString(),
		state.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error reading index", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError(
			"Index not found",
			fmt.Sprintf("No index %q on table %q.%q.",
				state.Name.ValueString(),
				state.Database.ValueString(),
				state.Table.ValueString(),
			),
		)
		return
	}

	state.Type = types.StringValue(got.Type)
	state.Column = types.StringValue(got.Column)
	state.Comment = types.StringValue(got.Comment)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *indexDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
