package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/gr8-toolkit/terraform-provider-starrocks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &databaseResource{}
	_ resource.ResourceWithConfigure   = &databaseResource{}
	_ resource.ResourceWithImportState = &databaseResource{}
)

func NewDatabaseResource() resource.Resource {
	return &databaseResource{}
}

type databaseResource struct {
	client *client.Client
}

// databaseResourceModel is the Terraform state model for starrocks_database.
//
// Design notes:
//   - `name` is the resource identity and triggers replacement on change.
//     StarRocks supports ALTER DATABASE RENAME but renaming silently breaks
//     all tables and catalog references — replacement is the safer default.
//   - `data_quota` and `replica_quota` are write-only (ALTER DATABASE SET …).
//     StarRocks does not expose them in SHOW CREATE DATABASE or any information
//     schema view accessible without privilege escalation, so they can only be
//     written, not read back. They are Optional+Computed so import works without
//     them drifting to null on the first refresh.
//   - `storage_volume` is likewise write-only for the same reason; it can be
//     set at CREATE time via PROPERTIES and updated via ALTER DATABASE SET.
//   - All three mutable fields are updatable in-place without replacing the DB.
type databaseResourceModel struct {
	Name          types.String `tfsdk:"name"`
	DataQuota     types.String `tfsdk:"data_quota"`
	ReplicaQuota  types.Int64  `tfsdk:"replica_quota"`
	StorageVolume types.String `tfsdk:"storage_volume"`
}

func (r *databaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (r *databaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StarRocks database. " +
			"`data_quota`, `replica_quota`, and `storage_volume` can be updated in-place. " +
			"Changing `name` destroys and recreates the database.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"data_quota": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Maximum data storage quota for the database, e.g. `\"10G\"`, `\"500M\"`, `\"2T\"`. " +
					"Accepts the same units as `ALTER DATABASE … SET DATA QUOTA`. " +
					"This value is write-only: StarRocks does not expose it in any readable view.",
			},
			"replica_quota": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Maximum number of tablet replicas for the database. " +
					"This value is write-only: StarRocks does not expose it in any readable view.",
			},
			"storage_volume": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Name of the storage volume to attach (shared-data clusters only). " +
					"This value is write-only: StarRocks does not expose it in any readable view.",
			},
		},
	}
}

// Create executes CREATE DATABASE and applies any quota/volume settings.
func (r *databaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	d := modelToDatabaseDef(plan)
	if err := r.client.CreateDatabase(d); err != nil {
		resp.Diagnostics.AddError("Unable to create database", err.Error())
		return
	}

	// Apply quota/volume settings that cannot be set in CREATE DATABASE.
	if d.DataQuota != "" || d.ReplicaQuota > 0 {
		if err := r.client.UpdateDatabase(d); err != nil {
			resp.Diagnostics.AddError("Unable to configure database quotas after create", err.Error())
			return
		}
	}

	// Normalise computed fields so state is consistent.
	setComputedDefaults(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read verifies the database still exists. Because the server exposes no
// readable quota or volume data, the existing state values are preserved as-is.
func (r *databaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	exists, err := r.client.GetDatabase(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading database", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// Quota and volume fields are write-only — keep whatever is in state.
	setComputedDefaults(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update issues ALTER DATABASE statements for changed quota / volume fields.
func (r *databaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan databaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	d := modelToDatabaseDef(plan)
	if err := r.client.UpdateDatabase(d); err != nil {
		resp.Diagnostics.AddError("Unable to update database", err.Error())
		return
	}

	setComputedDefaults(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete executes DROP DATABASE IF EXISTS.
func (r *databaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DropDatabase(state.Name.ValueString()); err != nil {
		if !client.IsDatabaseNotFoundError(err) {
			resp.Diagnostics.AddError("Unable to delete database", err.Error())
		}
	}
}

// ImportState imports a database by name. Quota and volume fields will be null
// after import because the server does not expose them; add them to config
// manually to avoid perpetual drift.
func (r *databaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := strings.TrimSpace(req.ID)
	if name == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Database name must not be empty.")
		return
	}

	exists, err := r.client.GetDatabase(name)
	if err != nil {
		resp.Diagnostics.AddError("Error importing database", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError("Database not found",
			fmt.Sprintf("No database named %q exists in StarRocks.", name))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(name))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("data_quota"), types.StringValue(""))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("replica_quota"), types.Int64Value(0))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("storage_volume"), types.StringValue(""))...)
}

func (r *databaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
// Helpers
// ---------------------------------------------------------------------------

func modelToDatabaseDef(m databaseResourceModel) client.DatabaseDef {
	return client.DatabaseDef{
		Name:          m.Name.ValueString(),
		DataQuota:     m.DataQuota.ValueString(),
		ReplicaQuota:  m.ReplicaQuota.ValueInt64(),
		StorageVolume: m.StorageVolume.ValueString(),
	}
}

// setComputedDefaults ensures Optional+Computed fields are never null in state,
// preventing perpetual drift when the user omits them from config.
func setComputedDefaults(m *databaseResourceModel) {
	if m.DataQuota.IsNull() || m.DataQuota.IsUnknown() {
		m.DataQuota = types.StringValue("")
	}
	if m.ReplicaQuota.IsNull() || m.ReplicaQuota.IsUnknown() {
		m.ReplicaQuota = types.Int64Value(0)
	}
	if m.StorageVolume.IsNull() || m.StorageVolume.IsUnknown() {
		m.StorageVolume = types.StringValue("")
	}
}
