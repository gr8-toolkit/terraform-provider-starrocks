package provider

import (
	"context"
	"fmt"

	"github.com/gr8-toolkit/terraform-provider-starrocks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &tableDataSource{}
	_ datasource.DataSourceWithConfigure = &tableDataSource{}
)

func NewTableDataSource() datasource.DataSource {
	return &tableDataSource{}
}

type tableDataSource struct {
	client *client.Client
}

type tableDataSourceModel struct {
	Database      types.String `tfsdk:"database"`
	Name          types.String `tfsdk:"name"`
	Engine        types.String `tfsdk:"engine"`
	KeyType       types.String `tfsdk:"key_type"`
	KeyColumns    types.List   `tfsdk:"key_columns"`
	Columns       types.List   `tfsdk:"columns"`
	DistributedBy types.String `tfsdk:"distributed_by"`
	Comment       types.String `tfsdk:"comment"`
	Properties    types.Map    `tfsdk:"properties"`
}

func (d *tableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table"
}

func (d *tableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	columnSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Computed: true, MarkdownDescription: "Column name."},
			"type":     schema.StringAttribute{Computed: true, MarkdownDescription: "Column data type."},
			"nullable": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the column allows NULL values."},
			"default":  schema.StringAttribute{Computed: true, MarkdownDescription: "Default value, empty string if unset."},
			"comment":  schema.StringAttribute{Computed: true, MarkdownDescription: "Column comment."},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads metadata for a StarRocks table by parsing `SHOW CREATE TABLE`. " +
			"Use `database.name` as the identifier.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database that contains the table.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The table name.",
			},
			"engine": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage engine (e.g. `OLAP`).",
			},
			"key_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Key type (e.g. `DUPLICATE KEY`, `PRIMARY KEY`).",
			},
			"key_columns": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Ordered list of key column names.",
			},
			"columns": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Ordered list of column definitions.",
				NestedObject:        columnSchema,
			},
			"distributed_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Distribution clause as reported by StarRocks.",
			},
			"comment": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Table-level comment.",
			},
			"properties": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Table PROPERTIES as reported by StarRocks.",
			},
		},
	}
}

func (d *tableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state tableDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := d.client.GetTable(state.Database.ValueString(), state.Name.ValueString())
	if err != nil {
		if isTableNotFoundError(err) {
			resp.Diagnostics.AddError(
				"Table not found",
				fmt.Sprintf("No table %q exists in database %q.", state.Name.ValueString(), state.Database.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading table", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError(
			"Table not found",
			fmt.Sprintf("No table %q exists in database %q.", state.Name.ValueString(), state.Database.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(tableDefToDataSourceState(ctx, got, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *tableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// tableDefToDataSourceState writes a client.TableDef into a tableDataSourceModel.
// Unlike the resource version this always absorbs all DB-returned values.
func tableDefToDataSourceState(ctx context.Context, t *client.TableDef, m *tableDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.Engine = types.StringValue(t.Engine)
	m.KeyType = types.StringValue(t.KeyType)
	m.DistributedBy = types.StringValue(t.DistBy)
	m.Comment = types.StringValue(t.Comment)

	// Key columns
	kcElems := make([]attr.Value, len(t.KeyColumns))
	for i, k := range t.KeyColumns {
		kcElems[i] = types.StringValue(k)
	}
	kc, d := types.ListValue(types.StringType, kcElems)
	diags.Append(d...)
	m.KeyColumns = kc

	// Columns
	colElems := make([]attr.Value, len(t.Columns))
	for i, col := range t.Columns {
		attrs := map[string]attr.Value{
			"name":     types.StringValue(col.Name),
			"type":     types.StringValue(col.Type),
			"nullable": types.BoolValue(col.Nullable),
			"default":  types.StringValue(col.Default),
			"comment":  types.StringValue(col.Comment),
		}
		obj, d := types.ObjectValue(columnAttrTypes, attrs)
		diags.Append(d...)
		colElems[i] = obj
	}
	colList, d := types.ListValue(types.ObjectType{AttrTypes: columnAttrTypes}, colElems)
	diags.Append(d...)
	m.Columns = colList

	// Properties
	propElems := make(map[string]attr.Value, len(t.Properties))
	for k, v := range t.Properties {
		propElems[k] = types.StringValue(v)
	}
	props, d := types.MapValue(types.StringType, propElems)
	diags.Append(d...)
	m.Properties = props

	return diags
}
