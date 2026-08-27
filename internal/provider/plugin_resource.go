package provider

import (
	"context"
	"fmt"
	"sort"
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
	_ resource.Resource                = &pluginResource{}
	_ resource.ResourceWithConfigure   = &pluginResource{}
	_ resource.ResourceWithImportState = &pluginResource{}
)

// NewPluginResource returns a new starrocks_plugin resource implementation.
func NewPluginResource() resource.Resource {
	return &pluginResource{}
}

type pluginResource struct {
	client *client.Client
}

// pluginResourceModel is the Terraform state model for starrocks_plugin.
//
// Design notes:
//   - `name` is the plugin identity. Plugins cannot be renamed; changing the
//     name triggers replacement (uninstall old, install new).
//   - `source` is the install path or URL passed to INSTALL PLUGIN FROM.
//     Changing it also triggers replacement because the only way to change the
//     source is to uninstall and reinstall.
//   - `properties` is an optional map forwarded to the PROPERTIES (...) clause
//     of INSTALL PLUGIN FROM. Used for e.g. md5sum verification.
//   - `type`, `status`, `description`, `version` are Computed — populated from
//     SHOW PLUGINS after install and refreshed on every Read.
type pluginResourceModel struct {
	Name        types.String `tfsdk:"name"`
	Source      types.String `tfsdk:"source"`
	Properties  types.Map    `tfsdk:"properties"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	Version     types.String `tfsdk:"version"`
	Status      types.String `tfsdk:"status"`
}

func (r *pluginResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin"
}

func (r *pluginResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StarRocks plugin. " +
			"Plugins are installed with `INSTALL PLUGIN FROM` and removed with `UNINSTALL PLUGIN`. " +
			"Changing `name` or `source` destroys the existing plugin and creates a new one. " +
			"Requires the `SYSTEM`-level `PLUGIN` privilege.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The plugin name, as it appears in `SHOW PLUGINS`. " +
					"Changing this attribute destroys the old plugin and creates a new one.",
				PlanModifiers: replaceString,
			},
			"source": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The install source: an absolute path to a `.zip` file or plugin " +
					"directory, or an `http`/`https` URL pointing to a `.zip` file. " +
					"Changing this attribute destroys the old plugin and installs from the new source.",
				PlanModifiers: replaceString,
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Optional key/value properties forwarded to the `PROPERTIES (...)` " +
					"clause of `INSTALL PLUGIN FROM`. Common use: `{\"md5sum\" = \"<hash>\"}` " +
					"to verify the zip file integrity. Changing properties triggers replacement.",
				PlanModifiers: []planmodifier.Map{mapRequiresReplace{}},
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

// Create executes INSTALL PLUGIN FROM and reads back the computed fields.
func (r *pluginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pluginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	properties := make(map[string]string)
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		resp.Diagnostics.Append(plan.Properties.ElementsAs(ctx, &properties, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if err := r.client.InstallPlugin(plan.Source.ValueString(), properties); err != nil {
		resp.Diagnostics.AddError("Unable to install plugin", err.Error())
		return
	}

	p, err := r.client.GetPlugin(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read plugin after install", err.Error())
		return
	}
	if p == nil {
		resp.Diagnostics.AddError(
			"Plugin not found after install",
			fmt.Sprintf("Plugin %q was not found via SHOW PLUGINS after installation. "+
				"Verify the name matches what StarRocks assigns to the installed plugin.", plan.Name.ValueString()),
		)
		return
	}

	applyPluginToModel(p, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the computed fields from SHOW PLUGINS.
func (r *pluginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pluginResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := r.client.GetPlugin(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading plugin", err.Error())
		return
	}
	if p == nil {
		// Plugin was removed outside Terraform — plan a re-create.
		resp.State.RemoveResource(ctx)
		return
	}

	applyPluginToModel(p, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never reached because every attribute either has RequiresReplace or
// is Computed. This implementation satisfies the resource.Resource interface and
// emits a loud error if it is ever called unexpectedly.
func (r *pluginResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"All starrocks_plugin attributes require replacement. "+
			"This is a provider bug — Update should never be called.",
	)
}

// Delete executes UNINSTALL PLUGIN.
func (r *pluginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pluginResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UninstallPlugin(state.Name.ValueString()); err != nil {
		// If the plugin is already gone treat it as a success.
		if !isPluginNotFoundError(err) {
			resp.Diagnostics.AddError("Unable to uninstall plugin", err.Error())
		}
	}
}

// ImportState imports a plugin by name.
func (r *pluginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := strings.TrimSpace(req.ID)
	if name == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Plugin name must not be empty.")
		return
	}

	p, err := r.client.GetPlugin(name)
	if err != nil {
		resp.Diagnostics.AddError("Error importing plugin", err.Error())
		return
	}
	if p == nil {
		resp.Diagnostics.AddError(
			"Plugin not found",
			fmt.Sprintf("No plugin named %q exists in StarRocks.", name),
		)
		return
	}

	// source and properties cannot be recovered from SHOW PLUGINS — the user
	// must add them to config after import to avoid perpetual drift.
	state := pluginResourceModel{
		Name:       types.StringValue(p.Name),
		Source:     types.StringValue(""),
		Properties: types.MapValueMust(types.StringType, map[string]attr.Value{}),
	}
	applyPluginToModel(p, &state)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), state.Name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source"), state.Source)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), state.Properties)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), state.Type)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("description"), state.Description)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("version"), state.Version)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("status"), state.Status)...)
}

func (r *pluginResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// applyPluginToModel writes SHOW PLUGINS fields back into the state model.
// source and properties are intentionally left unchanged — they are not
// returned by SHOW PLUGINS and must be preserved from the prior state/plan.
// Callers must ensure Properties is already set to a known value before
// calling this function, or set it explicitly afterwards.
func applyPluginToModel(p *client.Plugin, m *pluginResourceModel) {
	m.Type = types.StringValue(p.Type)
	m.Description = types.StringValue(p.Description)
	m.Version = types.StringValue(p.Version)
	m.Status = types.StringValue(p.Status)
	// Resolve Properties to an empty map when it is still null/unknown so
	// Terraform never sees an unknown value after apply.
	if m.Properties.IsNull() || m.Properties.IsUnknown() {
		m.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
}

// buildInstallPluginSQL is the pure-Go SQL builder exposed for unit tests.
func buildInstallPluginSQL(source string, properties map[string]string) string {
	q := fmt.Sprintf("INSTALL PLUGIN FROM %q", source)
	if len(properties) > 0 {
		var pairs []string
		for k, v := range properties {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, v))
		}
		sort.Strings(pairs)
		q += " PROPERTIES (" + strings.Join(pairs, ", ") + ")"
	}
	return q
}

// isPluginNotFoundError reports whether err looks like a "plugin not found"
// response from StarRocks.
func isPluginNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown plugin") ||
		strings.Contains(msg, "plugin") && strings.Contains(msg, "not found") ||
		strings.Contains(msg, "plugin") && strings.Contains(msg, "does not exist")
}
