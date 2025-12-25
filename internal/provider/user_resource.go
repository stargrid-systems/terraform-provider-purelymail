// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

// UserResource defines the resource implementation.
type UserResource struct {
	client *api.Client
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	UserName                       types.String `tfsdk:"user_name"`
	NewUserName                    types.String `tfsdk:"new_user_name"`
	Password                       types.String `tfsdk:"password"`
	PasswordWo                     types.String `tfsdk:"password_wo"`
	EnableSearchIndexing           types.Bool   `tfsdk:"enable_search_indexing"`
	EnablePasswordReset            types.Bool   `tfsdk:"enable_password_reset"`
	RequireTwoFactorAuthentication types.Bool   `tfsdk:"require_two_factor_authentication"`
	Id                             types.String `tfsdk:"id"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Purelymail user account.",

		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "The username. This is the local part of the email address (e.g., 'alice' for 'alice@example.com'). Cannot be changed after creation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"new_user_name": schema.StringAttribute{
				MarkdownDescription: "New username to rename the user (write-only, only used during updates).",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The user's password (write-only, only used during creation and updates, never returned).",
				Optional:            true,
				Sensitive:           true,
			},
			"password_wo": schema.StringAttribute{
				MarkdownDescription: "The user's password (write-only variant that remains in state, useful for tracking password changes).",
				Optional:            true,
				Sensitive:           true,
			},
			"enable_search_indexing": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable search indexing for this user.",
				Optional:            true,
				Computed:            true,
			},
			"enable_password_reset": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable password reset methods for this user.",
				Optional:            true,
				Computed:            true,
			},
			"require_two_factor_authentication": schema.BoolAttribute{
				MarkdownDescription: "Whether to require two-factor authentication for this user.",
				Optional:            true,
				Computed:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The user identifier (same as user_name).",
				Computed:            true,
			},
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create user via API
	httpResp, err := r.client.CreateUser(ctx, api.CreateUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create user: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to create user (status: %d)", httpResp.StatusCode))
		return
	}

	// Build modify request for any additional settings
	modifyReq := api.ModifyUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	}
	hasModifications := false

	// Handle password (prefer password_wo if both are set)
	if !data.PasswordWo.IsNull() && !data.PasswordWo.IsUnknown() {
		modifyReq.NewPassword = valueStringPtr(data.PasswordWo)
		hasModifications = true
	} else if !data.Password.IsNull() && !data.Password.IsUnknown() {
		modifyReq.NewPassword = valueStringPtr(data.Password)
		hasModifications = true
	}

	// Handle other settings
	if !data.EnableSearchIndexing.IsNull() {
		modifyReq.EnableSearchIndexing = valueBoolPtr(data.EnableSearchIndexing)
		hasModifications = true
	}
	if !data.EnablePasswordReset.IsNull() {
		modifyReq.EnablePasswordReset = valueBoolPtr(data.EnablePasswordReset)
		hasModifications = true
	}
	if !data.RequireTwoFactorAuthentication.IsNull() {
		modifyReq.RequireTwoFactorAuthentication = valueBoolPtr(data.RequireTwoFactorAuthentication)
		hasModifications = true
	}

	// Apply modifications if any
	if hasModifications {
		modifyResp, err := r.client.ModifyUser(ctx, modifyReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to modify user: %s", err))
			return
		}
		defer modifyResp.Body.Close()

		if modifyResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to modify user (status: %d)", modifyResp.StatusCode))
			return
		}
	}

	data.Id = data.UserName

	// Read back the user to get current state
	readResp, err := r.client.GetUser(ctx, api.GetUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	})
	if err == nil && readResp.StatusCode == http.StatusOK {
		defer readResp.Body.Close()
		var getUserResp api.GetUserResponse
		if json.NewDecoder(readResp.Body).Decode(&getUserResp) == nil && getUserResp.Result != nil {
			if getUserResp.Result.EnableSearchIndexing != nil {
				data.EnableSearchIndexing = types.BoolValue(*getUserResp.Result.EnableSearchIndexing)
			} else {
				data.EnableSearchIndexing = types.BoolValue(false)
			}
			if getUserResp.Result.RecoveryEnabled != nil {
				data.EnablePasswordReset = types.BoolValue(*getUserResp.Result.RecoveryEnabled)
			} else {
				data.EnablePasswordReset = types.BoolValue(false)
			}
			if getUserResp.Result.RequireTwoFactorAuthentication != nil {
				data.RequireTwoFactorAuthentication = types.BoolValue(*getUserResp.Result.RequireTwoFactorAuthentication)
			} else {
				data.RequireTwoFactorAuthentication = types.BoolValue(false)
			}
		} else {
			// Fallback: set defaults
			data.EnableSearchIndexing = types.BoolValue(false)
			data.EnablePasswordReset = types.BoolValue(false)
			data.RequireTwoFactorAuthentication = types.BoolValue(false)
		}
	} else {
		// Fallback: set defaults
		data.EnableSearchIndexing = types.BoolValue(false)
		data.EnablePasswordReset = types.BoolValue(false)
		data.RequireTwoFactorAuthentication = types.BoolValue(false)
	}

	// Clear write-only fields (except password_wo which stays in state)
	data.Password = types.StringNull()
	data.NewUserName = types.StringNull()

	tflog.Trace(ctx, "created purelymail_user resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read user from API
	httpResp, err := r.client.GetUser(ctx, api.GetUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound || httpResp.StatusCode == 404 {
		// User was deleted outside of Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to read user (status: %d)", httpResp.StatusCode))
		return
	}

	// Decode response
	var getUserResp api.GetUserResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&getUserResp); err != nil {
		resp.Diagnostics.AddError("Decode Error", fmt.Sprintf("Unable to decode user response: %s", err))
		return
	}

	// Update state from API response
	if getUserResp.Result != nil {
		if getUserResp.Result.EnableSearchIndexing != nil {
			data.EnableSearchIndexing = types.BoolValue(*getUserResp.Result.EnableSearchIndexing)
		} else {
			data.EnableSearchIndexing = types.BoolValue(false)
		}
		if getUserResp.Result.RecoveryEnabled != nil {
			data.EnablePasswordReset = types.BoolValue(*getUserResp.Result.RecoveryEnabled)
		} else {
			data.EnablePasswordReset = types.BoolValue(false)
		}
		if getUserResp.Result.RequireTwoFactorAuthentication != nil {
			data.RequireTwoFactorAuthentication = types.BoolValue(*getUserResp.Result.RequireTwoFactorAuthentication)
		} else {
			data.RequireTwoFactorAuthentication = types.BoolValue(false)
		}
	} else {
		data.EnableSearchIndexing = types.BoolValue(false)
		data.EnablePasswordReset = types.BoolValue(false)
		data.RequireTwoFactorAuthentication = types.BoolValue(false)
	}

	// Clear write-only fields (except password_wo which stays in state)
	data.Password = types.StringNull()
	data.NewUserName = types.StringNull()

	tflog.Trace(ctx, "read purelymail_user resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build modify request
	modifyReq := api.ModifyUserJSONRequestBody{
		UserName: state.UserName.ValueString(),
	}

	// Handle username change
	if !data.NewUserName.IsNull() && data.NewUserName.ValueString() != state.UserName.ValueString() {
		modifyReq.NewUserName = valueStringPtr(data.NewUserName)
	}

	// Handle password update (prefer password_wo if both are set)
	if !data.PasswordWo.IsNull() && !data.PasswordWo.IsUnknown() {
		modifyReq.NewPassword = valueStringPtr(data.PasswordWo)
	} else if !data.Password.IsNull() && !data.Password.IsUnknown() {
		modifyReq.NewPassword = valueStringPtr(data.Password)
	}

	// Handle other settings
	if !data.EnableSearchIndexing.IsNull() {
		modifyReq.EnableSearchIndexing = valueBoolPtr(data.EnableSearchIndexing)
	}
	if !data.EnablePasswordReset.IsNull() {
		modifyReq.EnablePasswordReset = valueBoolPtr(data.EnablePasswordReset)
	}
	if !data.RequireTwoFactorAuthentication.IsNull() {
		modifyReq.RequireTwoFactorAuthentication = valueBoolPtr(data.RequireTwoFactorAuthentication)
	}

	httpResp, err := r.client.ModifyUser(ctx, modifyReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to modify user: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to modify user (status: %d)", httpResp.StatusCode))
		return
	}

	// Update username if it changed
	if !data.NewUserName.IsNull() && data.NewUserName.ValueString() != state.UserName.ValueString() {
		data.UserName = data.NewUserName
		data.Id = data.NewUserName
	} else {
		data.Id = data.UserName
	}

	// Read back the user to get current state
	readResp, err := r.client.GetUser(ctx, api.GetUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	})
	if err == nil && readResp.StatusCode == http.StatusOK {
		defer readResp.Body.Close()
		var getUserResp api.GetUserResponse
		if json.NewDecoder(readResp.Body).Decode(&getUserResp) == nil && getUserResp.Result != nil {
			if getUserResp.Result.EnableSearchIndexing != nil {
				data.EnableSearchIndexing = types.BoolValue(*getUserResp.Result.EnableSearchIndexing)
			} else {
				data.EnableSearchIndexing = types.BoolValue(false)
			}
			if getUserResp.Result.RecoveryEnabled != nil {
				data.EnablePasswordReset = types.BoolValue(*getUserResp.Result.RecoveryEnabled)
			} else {
				data.EnablePasswordReset = types.BoolValue(false)
			}
			if getUserResp.Result.RequireTwoFactorAuthentication != nil {
				data.RequireTwoFactorAuthentication = types.BoolValue(*getUserResp.Result.RequireTwoFactorAuthentication)
			} else {
				data.RequireTwoFactorAuthentication = types.BoolValue(false)
			}
		}
	}

	// Clear write-only fields (except password_wo which stays in state)
	data.Password = types.StringNull()
	data.NewUserName = types.StringNull()

	tflog.Trace(ctx, "updated purelymail_user resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.DeleteUser(ctx, api.DeleteUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete user (status: %d)", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted purelymail_user resource")
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the username as the import identifier
	resource.ImportStatePassthroughID(ctx, path.Root("user_name"), req, resp)
}

// Helper functions
func valueStringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func valueBoolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}
