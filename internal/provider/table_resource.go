package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/gr8-toolkit/terraform-provider-starrocks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &tableResource{}
	_ resource.ResourceWithConfigure   = &tableResource{}
	_ resource.ResourceWithImportState = &tableResource{}
)

func NewTableResource() resource.Resource {
	return &tableResource{}
}

type tableResource struct {
	client *client.Client
}

// tableResourceModel is the Terraform state model for starrocks_table.
//
// Design notes:
//   - `database` + `name` together identify the table; both trigger replacement.
//   - `columns` is an ordered list — order matters both for DDL and for StarRocks
//     key-column semantics. It is the only mutable field: Update computes the
//     diff and issues the minimal set of ALTER TABLE statements.
//   - `engine`, `key_type`, `key_columns`, `distributed_by`, `properties` are
//     creation-time only and trigger replacement on change.
//   - `comment` is Optional+Computed and can be updated without recreation via
//     ALTER TABLE ... COMMENT.
type tableResourceModel struct {
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

// columnModel mirrors client.ColumnDef for the Terraform schema.
type columnModel struct {
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Nullable types.Bool   `tfsdk:"nullable"`
	Default  types.String `tfsdk:"default"`
	Comment  types.String `tfsdk:"comment"`
}

// columnAttrTypes is the attr.Type map for the column nested object.
var columnAttrTypes = map[string]attr.Type{
	"name":     types.StringType,
	"type":     types.StringType,
	"nullable": types.BoolType,
	"default":  types.StringType,
	"comment":  types.StringType,
}

func (r *tableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table"
}

func (r *tableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StarRocks OLAP table. " +
			"Column additions, removals, and type changes are handled in-place via `ALTER TABLE`. " +
			"Changes to `database`, `name`, `engine`, `key_type`, `key_columns`, `distributed_by`, or `properties` " +
			"require replacing (destroying and recreating) the table.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database in which to create the table.",
				PlanModifiers:       replaceString,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The table name.",
				PlanModifiers:       replaceString,
			},
			"engine": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Storage engine. Defaults to `OLAP`.",
				PlanModifiers:       replaceString,
			},
			"key_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Key type: `DUPLICATE KEY`, `AGGREGATE KEY`, `UNIQUE KEY`, or `PRIMARY KEY`. " +
					"Defaults to `DUPLICATE KEY`.",
				PlanModifiers: replaceString,
			},
			"key_columns": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Ordered list of key column names.",
				PlanModifiers: []planmodifier.List{
					listRequiresReplace{},
				},
			},
			"columns": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Ordered list of column definitions. Column order is preserved.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Column name.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Column data type (e.g. `INT`, `VARCHAR(128)`, `DATETIME`).",
						},
						"nullable": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
							MarkdownDescription: "Whether the column allows NULL values. Defaults to `true`.",
						},
						"default": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Default value for the column. Omit to leave unset.",
						},
						"comment": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Column comment.",
						},
					},
				},
			},
			"distributed_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Distribution clause, e.g. `DISTRIBUTED BY HASH(id)` or " +
					"`DISTRIBUTED BY HASH(id) BUCKETS 10`. " +
					"If omitted StarRocks uses random bucketing.",
				PlanModifiers: replaceString,
			},
			"comment": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Table-level comment. Can be updated without recreating the table.",
			},
			"properties": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Table PROPERTIES map, e.g. `{\"replication_num\" = \"1\"}`. Triggers replacement on change.",
				PlanModifiers: []planmodifier.Map{
					mapRequiresReplace{},
				},
			},
		},
	}
}

// Create builds and executes CREATE TABLE then reads back the computed fields.
func (r *tableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	td, diags := modelToTableDef(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.CreateTable(plan.Database.ValueString(), td); err != nil {
		resp.Diagnostics.AddError("Unable to create table", err.Error())
		return
	}

	// Read back to populate computed fields (engine, key_type, distributed_by, etc.)
	got, err := r.client.GetTable(plan.Database.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read table after create", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError("Table not found after create", qualifiedName(plan))
		return
	}

	resp.Diagnostics.Append(tableDefToState(ctx, got, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state from SHOW CREATE TABLE.
func (r *tableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetTable(state.Database.ValueString(), state.Name.ValueString())
	if err != nil {
		if isTableNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading table", err.Error())
		return
	}
	if got == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(tableDefToState(ctx, got, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles the three column-change scenarios without recreating the table:
//   - new column added          → ALTER TABLE ADD COLUMN
//   - column removed            → ALTER TABLE DROP COLUMN
//   - column type/attrs changed → ALTER TABLE MODIFY COLUMN
//
// It also handles a comment-only change via AlterTableComment.
// All other attribute changes are covered by RequiresReplace plan modifiers.
func (r *tableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan tableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	db := plan.Database.ValueString()
	tbl := plan.Name.ValueString()

	// --- resolve column diff ---
	oldCols, diags := listToColumnDefs(ctx, state.Columns)
	resp.Diagnostics.Append(diags...)
	newCols, diagsN := listToColumnDefs(ctx, plan.Columns)
	resp.Diagnostics.Append(diagsN...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldByName := make(map[string]client.ColumnDef, len(oldCols))
	for _, c := range oldCols {
		oldByName[c.Name] = c
	}
	newByName := make(map[string]client.ColumnDef, len(newCols))
	for _, c := range newCols {
		newByName[c.Name] = c
	}

	// Drop columns that no longer exist in the plan.
	for _, old := range oldCols {
		if _, exists := newByName[old.Name]; !exists {
			if err := r.client.AlterTableDropColumn(db, tbl, old.Name); err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Unable to drop column %q", old.Name),
					err.Error(),
				)
				return
			}
		}
	}

	// Add or modify columns that are new or changed.
	for _, newCol := range newCols {
		if oldCol, exists := oldByName[newCol.Name]; !exists {
			// New column.
			if err := r.client.AlterTableAddColumn(db, tbl, newCol); err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Unable to add column %q", newCol.Name),
					err.Error(),
				)
				return
			}
		} else if columnChanged(oldCol, newCol) {
			// Existing column with changed definition.
			if err := r.client.AlterTableModifyColumn(db, tbl, newCol); err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Unable to modify column %q", newCol.Name),
					err.Error(),
				)
				return
			}
		}
	}

	// Update table comment if it changed.
	if !plan.Comment.Equal(state.Comment) && !plan.Comment.IsNull() && !plan.Comment.IsUnknown() {
		if err := r.client.AlterTableComment(db, tbl, plan.Comment.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to update table comment", err.Error())
			return
		}
	}

	// Read back so state reflects exactly what StarRocks has.
	got, err := r.client.GetTable(db, tbl)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read table after update", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError("Table not found after update", qualifiedName(plan))
		return
	}

	resp.Diagnostics.Append(tableDefToState(ctx, got, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete executes DROP TABLE IF EXISTS.
func (r *tableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DropTable(state.Database.ValueString(), state.Name.ValueString()); err != nil {
		if !isTableNotFoundError(err) {
			resp.Diagnostics.AddError("Unable to delete table", err.Error())
		}
	}
}

// ImportState imports a table using the "database.table" import ID format.
func (r *tableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"database.table\", got %q.", req.ID),
		)
		return
	}
	db, tbl := parts[0], parts[1]

	got, err := r.client.GetTable(db, tbl)
	if err != nil {
		resp.Diagnostics.AddError("Error importing table", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError(
			"Table not found",
			fmt.Sprintf("No table %q exists in database %q.", tbl, db),
		)
		return
	}

	state := tableResourceModel{
		Database: types.StringValue(db),
		Name:     types.StringValue(tbl),
	}
	resp.Diagnostics.Append(tableDefToState(ctx, got, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), state.Database)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), state.Name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("engine"), state.Engine)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key_type"), state.KeyType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key_columns"), state.KeyColumns)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("columns"), state.Columns)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("distributed_by"), state.DistributedBy)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("comment"), state.Comment)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), state.Properties)...)
}

func (r *tableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// modelToTableDef converts the Terraform state model to the client TableDef.
func modelToTableDef(ctx context.Context, m tableResourceModel) (*client.TableDef, diag.Diagnostics) {
	var diags diag.Diagnostics

	cols, d := listToColumnDefs(ctx, m.Columns)
	diags.Append(d...)

	var keyColumns []string
	if !m.KeyColumns.IsNull() && !m.KeyColumns.IsUnknown() {
		diags.Append(m.KeyColumns.ElementsAs(ctx, &keyColumns, false)...)
	}

	properties := make(map[string]string)
	if !m.Properties.IsNull() && !m.Properties.IsUnknown() {
		diags.Append(m.Properties.ElementsAs(ctx, &properties, false)...)
	}

	return &client.TableDef{
		Database:   m.Database.ValueString(),
		Name:       m.Name.ValueString(),
		Engine:     m.Engine.ValueString(),
		KeyType:    m.KeyType.ValueString(),
		KeyColumns: keyColumns,
		Columns:    cols,
		DistBy:     m.DistributedBy.ValueString(),
		Comment:    m.Comment.ValueString(),
		Properties: properties,
	}, diags
}

// tableDefToState writes fields from a parsed TableDef back into the model.
func tableDefToState(ctx context.Context, t *client.TableDef, m *tableResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if t.Engine != "" {
		m.Engine = types.StringValue(t.Engine)
	} else if m.Engine.IsNull() || m.Engine.IsUnknown() {
		m.Engine = types.StringValue("OLAP")
	}

	if t.KeyType != "" {
		m.KeyType = types.StringValue(t.KeyType)
	} else if m.KeyType.IsNull() || m.KeyType.IsUnknown() {
		m.KeyType = types.StringValue("DUPLICATE KEY")
	}

	if len(t.KeyColumns) > 0 {
		elems := make([]attr.Value, len(t.KeyColumns))
		for i, k := range t.KeyColumns {
			elems[i] = types.StringValue(k)
		}
		kc, d := types.ListValue(types.StringType, elems)
		diags.Append(d...)
		m.KeyColumns = kc
	} else if m.KeyColumns.IsNull() || m.KeyColumns.IsUnknown() {
		m.KeyColumns = types.ListValueMust(types.StringType, []attr.Value{})
	}

	// Columns — always overwrite from DB so state is authoritative.
	colElems := make([]attr.Value, len(t.Columns))
	for i, col := range t.Columns {
		attrs := map[string]attr.Value{
			"name":     types.StringValue(col.Name),
			"type":     types.StringValue(col.Type),
			"nullable": types.BoolValue(col.Nullable),
			"default":  types.StringNull(),
			"comment":  types.StringNull(),
		}
		if col.Default != "" {
			attrs["default"] = types.StringValue(col.Default)
		}
		if col.Comment != "" {
			attrs["comment"] = types.StringValue(col.Comment)
		}
		obj, d := types.ObjectValue(columnAttrTypes, attrs)
		diags.Append(d...)
		colElems[i] = obj
	}
	colList, d := types.ListValue(types.ObjectType{AttrTypes: columnAttrTypes}, colElems)
	diags.Append(d...)
	m.Columns = colList

	if t.DistBy != "" {
		// Prefer the existing state value to avoid drift caused by StarRocks
		// normalising identifiers (e.g. backtick-quoting column names).
		// Only use the DB value when state is empty (first read after import).
		if m.DistributedBy.IsNull() || m.DistributedBy.IsUnknown() || m.DistributedBy.ValueString() == "" {
			m.DistributedBy = types.StringValue(t.DistBy)
		}
	} else if m.DistributedBy.IsNull() || m.DistributedBy.IsUnknown() {
		m.DistributedBy = types.StringValue("")
	}

	if t.Comment != "" {
		m.Comment = types.StringValue(t.Comment)
	} else if m.Comment.IsNull() || m.Comment.IsUnknown() {
		m.Comment = types.StringValue("")
	}

	// Properties: merge DB-returned properties into the existing state map
	// so that StarRocks-injected defaults (compression, fast_schema_evolution,
	// replicated_storage, etc.) never appear as unexpected new elements.
	// On import (state map is null/unknown/empty) absorb all DB properties.
	existingProps := make(map[string]string)
	if !m.Properties.IsNull() && !m.Properties.IsUnknown() {
		diags.Append(m.Properties.ElementsAs(ctx, &existingProps, false)...)
	}

	if len(existingProps) == 0 && len(t.Properties) > 0 {
		// Import path: absorb everything the DB returns.
		elems := make(map[string]attr.Value, len(t.Properties))
		for k, v := range t.Properties {
			elems[k] = types.StringValue(v)
		}
		props, d := types.MapValue(types.StringType, elems)
		diags.Append(d...)
		m.Properties = props
	} else if len(existingProps) > 0 {
		// Normal path: only update keys the user already tracks; ignore
		// StarRocks-injected keys that weren't in the original config.
		elems := make(map[string]attr.Value, len(existingProps))
		for k := range existingProps {
			if v, ok := t.Properties[k]; ok {
				elems[k] = types.StringValue(v)
			} else {
				elems[k] = types.StringValue(existingProps[k])
			}
		}
		props, d := types.MapValue(types.StringType, elems)
		diags.Append(d...)
		m.Properties = props
	} else if m.Properties.IsNull() || m.Properties.IsUnknown() {
		m.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	return diags
}

// listToColumnDefs converts a types.List of column objects to []client.ColumnDef.
func listToColumnDefs(ctx context.Context, list types.List) ([]client.ColumnDef, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var models []columnModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	cols := make([]client.ColumnDef, len(models))
	for i, m := range models {
		cols[i] = client.ColumnDef{
			Name:     m.Name.ValueString(),
			Type:     m.Type.ValueString(),
			Nullable: m.Nullable.ValueBool(),
			Default:  m.Default.ValueString(),
			Comment:  m.Comment.ValueString(),
		}
	}
	return cols, diags
}

// columnChanged reports whether any field of a column differs between old and
// new, signalling that MODIFY COLUMN is needed.
func columnChanged(old, new client.ColumnDef) bool {
	return !strings.EqualFold(old.Type, new.Type) ||
		old.Nullable != new.Nullable ||
		old.Default != new.Default ||
		old.Comment != new.Comment
}

// qualifiedName returns "database.table" for use in error messages.
func qualifiedName(m tableResourceModel) string {
	return m.Database.ValueString() + "." + m.Name.ValueString()
}

// isTableNotFoundError returns true when err looks like a "table not found"
// response from StarRocks.
func isTableNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown table") ||
		strings.Contains(msg, "is not found") ||
		(strings.Contains(msg, "table") && strings.Contains(msg, "doesn't exist")) ||
		strings.Contains(msg, "does not exist")
}

// ---------------------------------------------------------------------------
// Plan modifier helpers — RequiresReplace for List and Map attributes.
// The framework ships these for String via stringplanmodifier; List and Map
// need custom implementations.
// ---------------------------------------------------------------------------

type listRequiresReplace struct{}

func (l listRequiresReplace) Description(_ context.Context) string {
	return "If the value of this attribute changes, Terraform will destroy and recreate the resource."
}
func (l listRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return l.Description(ctx)
}
func (l listRequiresReplace) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.StateValue.IsNull() && !req.PlanValue.Equal(req.StateValue) {
		resp.RequiresReplace = true
	}
}

type mapRequiresReplace struct{}

func (m mapRequiresReplace) Description(_ context.Context) string {
	return "If the value of this attribute changes, Terraform will destroy and recreate the resource."
}
func (m mapRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (m mapRequiresReplace) PlanModifyMap(_ context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if !req.StateValue.IsNull() && !req.PlanValue.Equal(req.StateValue) {
		resp.RequiresReplace = true
	}
}
