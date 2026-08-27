package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/gr8-toolkit/terraform-provider-starrocks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &catalogResource{}
	_ resource.ResourceWithConfigure   = &catalogResource{}
	_ resource.ResourceWithImportState = &catalogResource{}
)

func NewCatalogResource() resource.Resource {
	return &catalogResource{}
}

type catalogResource struct {
	client *client.Client
}

// catalogResourceModel is the Terraform state model for starrocks_catalog.
//
// Design notes:
//   - `name` doubles as the resource identity (no separate `id` attribute).
//   - `type` is Computed so it can be populated on import / read without the
//     user having to set it in config.
//   - `properties` is a free-form map of string key/value pairs that maps
//     directly to the PROPERTIES (...) clause of CREATE EXTERNAL CATALOG.
//     Sensitive values (credentials) inside the map are the user's
//     responsibility; the whole map is marked Sensitive to avoid leaking them
//     in plan output.
//   - `comment` is Optional+Computed so it survives import even when not set
//     in config.
//   - The internal catalog (default_catalog) can only be imported, not created
//     or destroyed via Terraform.
type catalogResourceModel struct {
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Comment    types.String `tfsdk:"comment"`
	Properties types.Map    `tfsdk:"properties"`
}

func (r *catalogResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog"
}

func (r *catalogResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StarRocks catalog. " +
			"External catalogs are created with `CREATE EXTERNAL CATALOG` and can be fully managed. " +
			"The internal catalog (`default_catalog`) can only be imported — it cannot be created or destroyed by Terraform.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the catalog. Use `default_catalog` to import the built-in internal catalog.",
				PlanModifiers: []planmodifier.String{
					// Renaming is not supported by StarRocks; force replacement instead.
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The catalog type as reported by StarRocks (e.g. `Internal`, `Hive`, `Iceberg`).",
			},
			"comment": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional description for external catalogs.",
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				MarkdownDescription: "Key/value properties passed to `PROPERTIES (...)` on external catalogs. " +
					"Must include at least `\"type\"`. Marked sensitive because credentials are commonly stored here.",
			},
		},
	}
}

// Create builds and executes CREATE EXTERNAL CATALOG.
// The internal catalog cannot be created — callers that attempt to manage
// default_catalog should import it instead.
func (r *catalogResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan catalogResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if strings.EqualFold(plan.Name.ValueString(), "default_catalog") {
		resp.Diagnostics.AddError(
			"Cannot create internal catalog",
			"The internal catalog (default_catalog) is built into every StarRocks cluster and cannot be created by Terraform. "+
				"Use `terraform import` to bring it under management instead.",
		)
		return
	}

	properties := make(map[string]string)
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		resp.Diagnostics.Append(plan.Properties.ElementsAs(ctx, &properties, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	comment := plan.Comment.ValueString()
	if err := r.client.CreateCatalog(plan.Name.ValueString(), comment, properties); err != nil {
		resp.Diagnostics.AddError("Unable to create catalog", err.Error())
		return
	}

	// Populate computed fields from the DB so state is accurate.
	cat, err := r.client.GetCatalog(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read catalog after create", err.Error())
		return
	}
	if cat == nil {
		resp.Diagnostics.AddError("Catalog not found after create", plan.Name.ValueString())
		return
	}

	plan.Type = types.StringValue(cat.Type)
	if plan.Comment.IsNull() || plan.Comment.IsUnknown() {
		plan.Comment = cat.Comment
	}
	if plan.Properties.IsNull() || plan.Properties.IsUnknown() {
		plan.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state from SHOW CATALOGS. For external catalogs the
// properties are intentionally kept from state (StarRocks anonymises
// credential values in SHOW CREATE CATALOG output, making round-trips lossy).
func (r *catalogResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state catalogResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cat, err := r.client.GetCatalog(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading catalog", err.Error())
		return
	}
	if cat == nil {
		// Catalog has been deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	state.Type = types.StringValue(cat.Type)
	// Keep comment from state when StarRocks returns empty (external catalogs
	// report NULL which becomes an empty string after scanning).
	if cat.Comment.ValueString() != "" {
		state.Comment = cat.Comment
	}
	// Properties are intentionally kept from prior state — the server
	// anonymises secrets in its output, so we cannot safely round-trip them.

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is a no-op at the SQL level — StarRocks has no ALTER CATALOG for
// property changes. Any change that reaches Update (which can only be comment
// or properties, since name uses RequiresReplace) is applied by
// delete-and-recreate so the framework never actually calls Update.
// We leave this method as a safety net that errors loudly if it is ever hit
// unexpectedly.
func (r *catalogResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Catalog updates require replacing the resource. "+
			"Changes to 'name' trigger replacement automatically; "+
			"for property changes, taint the resource and re-apply.",
	)
}

// Delete executes DROP CATALOG. The internal catalog is silently skipped.
func (r *catalogResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state catalogResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if strings.EqualFold(state.Name.ValueString(), "default_catalog") {
		// The internal catalog cannot be dropped; removing it from state is
		// the only valid action.
		return
	}

	if err := r.client.DeleteCatalog(state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete catalog", err.Error())
	}
}

// ImportState imports a catalog by name. Both external and the internal
// catalog (default_catalog) can be imported.
func (r *catalogResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cat, err := r.client.GetCatalog(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing catalog", err.Error())
		return
	}
	if cat == nil {
		resp.Diagnostics.AddError(
			"Catalog not found",
			fmt.Sprintf("No catalog named %q exists in StarRocks.", req.ID),
		)
		return
	}

	state := catalogResourceModel{
		Name:    types.StringValue(cat.Name),
		Type:    types.StringValue(cat.Type),
		Comment: cat.Comment,
		// Properties cannot be recovered from the server (credentials are
		// anonymised). Import with an empty map; the user must add them back
		// in config to avoid perpetual drift.
		Properties: types.MapValueMust(types.StringType, map[string]attr.Value{}),
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), state.Name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), state.Type)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("comment"), state.Comment)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), state.Properties)...)
}

func (r *catalogResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
