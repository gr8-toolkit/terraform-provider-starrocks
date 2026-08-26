package starrocks

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &indexResource{}
	_ resource.ResourceWithConfigure   = &indexResource{}
	_ resource.ResourceWithImportState = &indexResource{}
)

func NewIndexResource() resource.Resource {
	return &indexResource{}
}

type indexResource struct {
	client *Client
}

// indexResourceModel is the Terraform state model for starrocks_index.
//
// Design notes:
//   - All five identity fields (`database`, `table`, `name`, `type`, `column`)
//     use RequiresReplace because StarRocks has no ALTER INDEX — the only way
//     to change any of them is to drop and recreate the index.
//   - `properties` is Optional+Computed and also RequiresReplace (NGRAMBF/GIN/
//     VECTOR params cannot be changed without recreating the index).
//   - `comment` is Optional+Computed. Changing it also requires replacement
//     because there is no standalone ALTER INDEX … COMMENT statement.
//   - `timeout` is Optional (default 300s) and only affects the Create/Delete
//     wait loop; it is not stored as real infrastructure state.
type indexResourceModel struct {
	Database   types.String `tfsdk:"database"`
	Table      types.String `tfsdk:"table"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Column     types.String `tfsdk:"column"`
	Properties types.Map    `tfsdk:"properties"`
	Comment    types.String `tfsdk:"comment"`
	Timeout    types.Int64  `tfsdk:"timeout"`
}

func (r *indexResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index"
}

func (r *indexResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StarRocks secondary index on a table column. " +
			"Supported index types: `BITMAP`, `NGRAMBF`, `GIN`, `VECTOR`. " +
			"All attributes except `timeout` trigger replacement on change because " +
			"StarRocks has no `ALTER INDEX` statement.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database that contains the table.",
				PlanModifiers:       replaceString,
			},
			"table": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The table to create the index on.",
				PlanModifiers:       replaceString,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The index name. Must be unique within the table.",
				PlanModifiers:       replaceString,
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Index type. One of `BITMAP`, `NGRAMBF`, `GIN`, `VECTOR`. " +
					"`BITMAP` is the default and most widely supported type.",
				PlanModifiers: replaceString,
			},
			"column": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The column to index. One column can have at most one index.",
				PlanModifiers:       replaceString,
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Optional index properties passed inside `(...)` after `USING <type>`. " +
					"Required for `NGRAMBF` (`gram_num`, `bloom_filter_fpp`), `GIN`, and `VECTOR` indexes. " +
					"Not used for `BITMAP`.",
				PlanModifiers: []planmodifier.Map{mapRequiresReplace{}},
			},
			"comment": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Index comment.",
				PlanModifiers:       replaceString,
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(300),
				MarkdownDescription: "Maximum seconds to wait for the asynchronous index job to finish. " +
					"Defaults to `300`. Not stored as infrastructure state.",
			},
		},
	}
}

// Create issues ALTER TABLE … ADD INDEX and waits for the async job to finish.
func (r *indexResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan indexResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idx, diags := modelToIndexDef(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout := int(plan.Timeout.ValueInt64())
	if err := r.client.CreateIndex(
		plan.Database.ValueString(),
		plan.Table.ValueString(),
		idx,
		timeout,
	); err != nil {
		resp.Diagnostics.AddError("Unable to create index", err.Error())
		return
	}

	// Read back computed fields (comment may have been normalised by StarRocks).
	got, err := r.client.GetIndex(
		plan.Database.ValueString(),
		plan.Table.ValueString(),
		plan.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read index after create", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError("Index not found after create", qualifiedIndexName(plan))
		return
	}

	applyIndexDefToModel(got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state from SHOW INDEXES.
func (r *indexResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state indexResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetIndex(
		state.Database.ValueString(),
		state.Table.ValueString(),
		state.Name.ValueString(),
	)
	if err != nil {
		if isIndexNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading index", err.Error())
		return
	}
	if got == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyIndexDefToModel(got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called because every attribute has RequiresReplace.
// This implementation exists only to satisfy the resource.Resource interface.
func (r *indexResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"All index attributes require replacement. "+
			"This is a bug — Update should never be called for starrocks_index.",
	)
}

// Delete issues DROP INDEX and waits for the async job to finish.
func (r *indexResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state indexResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout := int(state.Timeout.ValueInt64())
	if err := r.client.DropIndex(
		state.Database.ValueString(),
		state.Table.ValueString(),
		state.Name.ValueString(),
		timeout,
	); err != nil {
		if !isIndexNotFoundError(err) {
			resp.Diagnostics.AddError("Unable to delete index", err.Error())
		}
	}
}

// ImportState imports an index using the "database.table.index_name" format.
func (r *indexResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ".", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"database.table.index_name\", got %q.", req.ID),
		)
		return
	}
	db, tbl, name := parts[0], parts[1], parts[2]

	got, err := r.client.GetIndex(db, tbl, name)
	if err != nil {
		resp.Diagnostics.AddError("Error importing index", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError(
			"Index not found",
			fmt.Sprintf("No index %q on table %q.%q.", name, db, tbl),
		)
		return
	}

	state := indexResourceModel{
		Database: types.StringValue(db),
		Table:    types.StringValue(tbl),
		Timeout:  types.Int64Value(300),
	}
	applyIndexDefToModel(got, &state)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), state.Database)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), state.Table)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), state.Name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), state.Type)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("column"), state.Column)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), state.Properties)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("comment"), state.Comment)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("timeout"), state.Timeout)...)
}

func (r *indexResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *Client, got: %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// modelToIndexDef converts the Terraform state model to an IndexDef.
func modelToIndexDef(ctx context.Context, m indexResourceModel) (IndexDef, diag.Diagnostics) {
	var diags diag.Diagnostics
	props := make(map[string]string)
	if !m.Properties.IsNull() && !m.Properties.IsUnknown() {
		diags.Append(m.Properties.ElementsAs(ctx, &props, false)...)
	}
	return IndexDef{
		Name:       m.Name.ValueString(),
		Column:     m.Column.ValueString(),
		Type:       strings.ToUpper(m.Type.ValueString()),
		Comment:    m.Comment.ValueString(),
		Properties: props,
	}, diags
}

// applyIndexDefToModel writes the fields from a GetIndex result back into the
// model. Properties and timeout are preserved from the existing model since
// SHOW INDEXES does not return properties.
func applyIndexDefToModel(idx *IndexDef, m *indexResourceModel) {
	m.Name = types.StringValue(idx.Name)
	m.Column = types.StringValue(idx.Column)
	m.Type = types.StringValue(idx.Type)

	if idx.Comment != "" {
		m.Comment = types.StringValue(idx.Comment)
	} else if m.Comment.IsNull() || m.Comment.IsUnknown() {
		m.Comment = types.StringValue("")
	}

	// Properties are not returned by SHOW INDEXES — preserve state.
	if m.Properties.IsNull() || m.Properties.IsUnknown() {
		m.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	// Preserve timeout from state.
	if m.Timeout.IsNull() || m.Timeout.IsUnknown() {
		m.Timeout = types.Int64Value(300)
	}
}

// qualifiedIndexName returns "database.table.index" for error messages.
func qualifiedIndexName(m indexResourceModel) string {
	return m.Database.ValueString() + "." + m.Table.ValueString() + "." + m.Name.ValueString()
}
