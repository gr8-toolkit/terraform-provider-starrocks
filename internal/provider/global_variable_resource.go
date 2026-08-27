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
	_ resource.Resource                = &globalVariableResource{}
	_ resource.ResourceWithConfigure   = &globalVariableResource{}
	_ resource.ResourceWithImportState = &globalVariableResource{}
)

func NewGlobalVariableResource() resource.Resource {
	return &globalVariableResource{}
}

type globalVariableResource struct {
	client *client.Client
}

// globalVariableResourceModel is the Terraform state model for
// starrocks_global_variable.
//
// Design notes:
//   - `name` is the variable name and the resource identity. It uses
//     RequiresReplace because variable names are fixed; changing one would
//     mean managing an entirely different variable.
//   - `value` is Required and always a string. StarRocks coerces it to the
//     appropriate type (integer, boolean, …) on the server side.
//   - Destroying the resource resets the variable to its StarRocks default
//     via `SET GLOBAL <name> = DEFAULT`, rather than leaving it at the
//     configured value.
type globalVariableResourceModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

func (r *globalVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_variable"
}

func (r *globalVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StarRocks global system variable. " +
			"Applies the value cluster-wide via `SET GLOBAL`. " +
			"Destroying the resource resets the variable to its default value with `SET GLOBAL <name> = DEFAULT`.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The variable name (e.g. `query_timeout`, `exec_mem_limit`). " +
					"Changing this attribute destroys the old resource and creates a new one.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The desired global value. " +
					"Always supplied as a string; StarRocks coerces it to the variable's " +
					"underlying type (integer, boolean, etc.).",
			},
		},
	}
}

// Create applies the variable value globally.
func (r *globalVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan globalVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetGlobalVariable(plan.Name.ValueString(), plan.Value.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to set global variable", err.Error())
		return
	}

	// Read back the value StarRocks actually stored (it may normalise the
	// representation, e.g. "true" → "1", "300" → "300").
	actual, exists, err := r.client.GetGlobalVariable(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read global variable after set", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError(
			"Variable not found after set",
			fmt.Sprintf("Variable %q was not found. Verify the name is a valid StarRocks system variable.", plan.Name.ValueString()),
		)
		return
	}

	plan.Value = types.StringValue(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the value from SHOW GLOBAL VARIABLES.
func (r *globalVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state globalVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	actual, exists, err := r.client.GetGlobalVariable(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading global variable", err.Error())
		return
	}
	if !exists {
		// Variable no longer exists (e.g. StarRocks version downgrade removed
		// it). Remove from state so Terraform can re-create it.
		resp.State.RemoveResource(ctx)
		return
	}

	state.Value = types.StringValue(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update re-applies the variable with the new value.
func (r *globalVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan globalVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetGlobalVariable(plan.Name.ValueString(), plan.Value.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to update global variable", err.Error())
		return
	}

	// Read back to capture any server-side normalisation.
	actual, exists, err := r.client.GetGlobalVariable(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read global variable after update", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError("Variable not found after update", plan.Name.ValueString())
		return
	}

	plan.Value = types.StringValue(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete resets the variable to its StarRocks default.
func (r *globalVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state globalVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.ResetGlobalVariable(state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to reset global variable to default", err.Error())
	}
}

// ImportState imports a global variable by name.
func (r *globalVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := strings.TrimSpace(req.ID)
	if name == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Variable name must not be empty.")
		return
	}

	actual, exists, err := r.client.GetGlobalVariable(name)
	if err != nil {
		resp.Diagnostics.AddError("Error importing global variable", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError(
			"Variable not found",
			fmt.Sprintf("No global variable %q exists in StarRocks.", name),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(name))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("value"), types.StringValue(actual))...)
}

func (r *globalVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
