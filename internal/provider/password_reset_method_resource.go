package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PasswordResetMethodResource{}
var _ resource.ResourceWithImportState = &PasswordResetMethodResource{}

func NewPasswordResetMethodResource() resource.Resource {
	return &PasswordResetMethodResource{}
}

// PasswordResetMethodResource defines the resource implementation.
type PasswordResetMethodResource struct {
	client *api.Client
}

// PasswordResetMethodResourceModel describes the resource data model.
type PasswordResetMethodResourceModel struct {
	UserName      types.String `tfsdk:"user_name"`
	Type          types.String `tfsdk:"type"`
	Target        types.String `tfsdk:"target"`
	Description   types.String `tfsdk:"description"`
	AllowMfaReset types.Bool   `tfsdk:"allow_mfa_reset"`
	Id            types.String `tfsdk:"id"`
}

func (r *PasswordResetMethodResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password_reset_method"
}

func (r *PasswordResetMethodResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a password reset method for a Purelymail user. At least one password reset method must exist before enabling two-factor authentication on the user.",

		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "The full email address this password reset method belongs to (e.g., 'alice@example.com').",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of password reset method (e.g., 'email' or 'phone').",
				Required:            true,
			},
			"target": schema.StringAttribute{
				MarkdownDescription: "The target for the password reset method (email address or phone number).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description for this password reset method.",
				Optional:            true,
			},
			"allow_mfa_reset": schema.BoolAttribute{
				MarkdownDescription: "Whether this method can be used to reset MFA.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier for this password reset method (format: username:target).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *PasswordResetMethodResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *PasswordResetMethodResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PasswordResetMethodResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create password reset method via API
	upsertReq := api.UpsertPasswordResetRequest{
		UserName: data.UserName.ValueString(),
		Type:     data.Type.ValueString(),
		Target:   data.Target.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		desc := data.Description.ValueString()
		upsertReq.Description = &desc
	}

	if !data.AllowMfaReset.IsNull() && !data.AllowMfaReset.IsUnknown() {
		allow := data.AllowMfaReset.ValueBool()
		upsertReq.AllowMfaReset = &allow
	}

	httpResp, err := r.client.CreateOrUpdatePasswordResetMethod(ctx, upsertReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create password reset method: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to create password reset method (status: %d)", httpResp.StatusCode))
		return
	}

	// Set ID
	data.Id = types.StringValue(fmt.Sprintf("%s:%s", data.UserName.ValueString(), data.Target.ValueString()))

	// Read back to get current state
	if _, err := r.readPasswordResetMethod(ctx, &data); err != nil {
		resp.Diagnostics.AddWarning("Read Warning", fmt.Sprintf("Created password reset method but unable to read back: %s", err))
	}

	tflog.Trace(ctx, "created purelymail_password_reset_method resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PasswordResetMethodResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PasswordResetMethodResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := r.readPasswordResetMethod(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read password reset method: %s", err))
		return
	}

	if !found {
		// Password reset method no longer exists - remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	tflog.Trace(ctx, "read purelymail_password_reset_method resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PasswordResetMethodResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PasswordResetMethodResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state PasswordResetMethodResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update via upsert API
	upsertReq := api.UpsertPasswordResetRequest{
		UserName: data.UserName.ValueString(),
		Type:     data.Type.ValueString(),
		Target:   data.Target.ValueString(),
	}

	// Include existing target if it changed (for update operation)
	if state.Target.ValueString() != data.Target.ValueString() {
		existing := state.Target.ValueString()
		upsertReq.ExistingTarget = &existing
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		desc := data.Description.ValueString()
		upsertReq.Description = &desc
	}

	if !data.AllowMfaReset.IsNull() && !data.AllowMfaReset.IsUnknown() {
		allow := data.AllowMfaReset.ValueBool()
		upsertReq.AllowMfaReset = &allow
	}

	httpResp, err := r.client.CreateOrUpdatePasswordResetMethod(ctx, upsertReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update password reset method: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to update password reset method (status: %d)", httpResp.StatusCode))
		return
	}

	// Update ID if target changed
	data.Id = types.StringValue(fmt.Sprintf("%s:%s", data.UserName.ValueString(), data.Target.ValueString()))

	// Read back to get current state
	if _, err := r.readPasswordResetMethod(ctx, &data); err != nil {
		resp.Diagnostics.AddWarning("Read Warning", fmt.Sprintf("Updated password reset method but unable to read back: %s", err))
	}

	tflog.Trace(ctx, "updated purelymail_password_reset_method resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PasswordResetMethodResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PasswordResetMethodResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.DeletePasswordResetMethod(ctx, api.DeletePasswordResetRequest{
		UserName: data.UserName.ValueString(),
		Target:   data.Target.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete password reset method: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete password reset method (status: %d)", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted purelymail_password_reset_method resource")
}

func (r *PasswordResetMethodResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "username:target"
	idParts := strings.Split(req.ID, ":")
	if len(idParts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in format 'username:target', got: %s", req.ID),
		)
		return
	}

	// Set the attributes
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_name"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("target"), idParts[1])...)
}

// readPasswordResetMethod reads the password reset method from the API
// Returns true if found, false if not found (but no error).
func (r *PasswordResetMethodResource) readPasswordResetMethod(ctx context.Context, data *PasswordResetMethodResourceModel) (bool, error) {
	httpResp, err := r.client.ListPasswordResetMethods(ctx, api.ListPasswordResetRequest{
		UserName: data.UserName.ValueString(),
	})
	if err != nil {
		return false, fmt.Errorf("unable to list password reset methods: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API returned status %d", httpResp.StatusCode)
	}

	var listResp api.ListPasswordResetResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&listResp); err != nil {
		return false, fmt.Errorf("unable to decode response: %w", err)
	}

	if listResp.Result == nil || listResp.Result.Users == nil || len(*listResp.Result.Users) == 0 {
		return false, nil
	}

	// Find the matching method by target
	targetValue := data.Target.ValueString()
	for _, method := range *listResp.Result.Users {
		if method.Target != nil && *method.Target == targetValue {
			// Update data from API response
			if method.Type != nil {
				data.Type = types.StringValue(*method.Type)
			}
			if method.Description != nil {
				data.Description = types.StringValue(*method.Description)
			} else {
				data.Description = types.StringNull()
			}
			if method.AllowMfaReset != nil {
				data.AllowMfaReset = types.BoolValue(*method.AllowMfaReset)
			} else {
				data.AllowMfaReset = types.BoolValue(false)
			}
			return true, nil
		}
	}

	// Password reset method not found
	return false, nil
}
