// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	RequireTwoFactorAuthentication types.Bool   `tfsdk:"require_two_factor_authentication"`
	PasswordResetMethods           types.List   `tfsdk:"password_reset_methods"`
	Id                             types.String `tfsdk:"id"`
}

// PasswordResetMethodModel describes a password reset method nested object.
type PasswordResetMethodModel struct {
	Type          types.String `tfsdk:"type"`
	Target        types.String `tfsdk:"target"`
	Description   types.String `tfsdk:"description"`
	AllowMfaReset types.Bool   `tfsdk:"allow_mfa_reset"`
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
			"require_two_factor_authentication": schema.BoolAttribute{
				MarkdownDescription: "Whether to require two-factor authentication for this user. Note: At least one password reset method must be configured before this can be enabled.",
				Optional:            true,
				Computed:            true,
			},
			"password_reset_methods": schema.ListNestedAttribute{
				MarkdownDescription: "Password reset methods for this user. At least one is required if two-factor authentication is enabled.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							MarkdownDescription: "The type of password reset method. Valid values: 'email', 'phone'.",
							Required:            true,
						},
						"target": schema.StringAttribute{
							MarkdownDescription: "The target for the password reset method (email address or phone number).",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "An optional description for this password reset method.",
							Optional:            true,
						},
						"allow_mfa_reset": schema.BoolAttribute{
							MarkdownDescription: "Whether this method can be used to reset multi-factor authentication.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
					},
				},
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

	// Step 1: Create user via API
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

	// Build modify request for initial settings (without 2FA)
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

	// Handle other settings (except 2FA for now)
	if !data.EnableSearchIndexing.IsNull() {
		modifyReq.EnableSearchIndexing = valueBoolPtr(data.EnableSearchIndexing)
		hasModifications = true
	}

	// Apply initial modifications if any
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

	// Step 2: Upsert password reset methods if configured
	if !data.PasswordResetMethods.IsNull() && !data.PasswordResetMethods.IsUnknown() {
		var methods []PasswordResetMethodModel
		diags := data.PasswordResetMethods.ElementsAs(ctx, &methods, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, method := range methods {
			upsertReq := api.UpsertPasswordResetRequest{
				UserName: data.UserName.ValueString(),
				Type:     method.Type.ValueString(),
				Target:   method.Target.ValueString(),
			}

			if !method.Description.IsNull() && !method.Description.IsUnknown() {
				desc := method.Description.ValueString()
				upsertReq.Description = &desc
			}

			if !method.AllowMfaReset.IsNull() && !method.AllowMfaReset.IsUnknown() {
				allow := method.AllowMfaReset.ValueBool()
				upsertReq.AllowMfaReset = &allow
			}

			upsertResp, err := r.client.CreateOrUpdatePasswordResetMethod(ctx, upsertReq)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create password reset method: %s", err))
				return
			}
			defer upsertResp.Body.Close()

			if upsertResp.StatusCode != http.StatusOK {
				resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to create password reset method (status: %d)", upsertResp.StatusCode))
				return
			}
		}
	}

	// Step 3: Enable 2FA if requested (after password reset methods are configured)
	if !data.RequireTwoFactorAuthentication.IsNull() && data.RequireTwoFactorAuthentication.ValueBool() {
		enable2FAReq := api.ModifyUserJSONRequestBody{
			UserName:                       data.UserName.ValueString(),
			RequireTwoFactorAuthentication: valueBoolPtr(data.RequireTwoFactorAuthentication),
		}

		enable2FAResp, err := r.client.ModifyUser(ctx, enable2FAReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to enable 2FA: %s", err))
			return
		}
		defer enable2FAResp.Body.Close()

		if enable2FAResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to enable 2FA (status: %d)", enable2FAResp.StatusCode))
			return
		}
	}

	data.Id = data.UserName

	// Read back the user to get current state
	if _, err := r.readUser(ctx, &data); err != nil {
		resp.Diagnostics.AddWarning("Read Warning", fmt.Sprintf("Created user but unable to read back: %s", err))
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

	// Read user data including password reset methods
	found, err := r.readUser(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read user: %s", err))
		return
	}

	if !found {
		// User was deleted outside of Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	// Clear write-only fields (except password_wo which stays in state)
	data.Password = types.StringNull()
	data.NewUserName = types.StringNull()

	tflog.Trace(ctx, "read purelymail_user resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel
	var state UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1: Disable 2FA first if it's being turned off (before modifying password reset methods)
	if !state.RequireTwoFactorAuthentication.IsNull() && state.RequireTwoFactorAuthentication.ValueBool() &&
		(!data.RequireTwoFactorAuthentication.IsNull() && !data.RequireTwoFactorAuthentication.ValueBool()) {

		disable2FAReq := api.ModifyUserJSONRequestBody{
			UserName:                       state.UserName.ValueString(),
			RequireTwoFactorAuthentication: valueBoolPtr(data.RequireTwoFactorAuthentication),
		}

		disable2FAResp, err := r.client.ModifyUser(ctx, disable2FAReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to disable 2FA: %s", err))
			return
		}
		defer disable2FAResp.Body.Close()

		if disable2FAResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to disable 2FA (status: %d)", disable2FAResp.StatusCode))
			return
		}
	}

	// Step 2: Update basic user settings (username, password, search indexing)
	modifyReq := api.ModifyUserJSONRequestBody{
		UserName: state.UserName.ValueString(),
	}
	hasModifications := false

	// Handle username change
	if !data.NewUserName.IsNull() && data.NewUserName.ValueString() != state.UserName.ValueString() {
		modifyReq.NewUserName = valueStringPtr(data.NewUserName)
		hasModifications = true
	}

	// Handle password update (prefer password_wo if both are set)
	if !data.PasswordWo.IsNull() && !data.PasswordWo.IsUnknown() {
		modifyReq.NewPassword = valueStringPtr(data.PasswordWo)
		hasModifications = true
	} else if !data.Password.IsNull() && !data.Password.IsUnknown() {
		modifyReq.NewPassword = valueStringPtr(data.Password)
		hasModifications = true
	}

	// Handle search indexing
	if !data.EnableSearchIndexing.IsNull() {
		modifyReq.EnableSearchIndexing = valueBoolPtr(data.EnableSearchIndexing)
		hasModifications = true
	}

	if hasModifications {
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
	}

	// Update username if it changed
	if !data.NewUserName.IsNull() && data.NewUserName.ValueString() != state.UserName.ValueString() {
		data.UserName = data.NewUserName
		data.Id = data.NewUserName
	} else {
		data.Id = data.UserName
	}

	// Step 3: Update password reset methods
	// Get current methods from state
	var stateMethods []PasswordResetMethodModel
	if !state.PasswordResetMethods.IsNull() && !state.PasswordResetMethods.IsUnknown() {
		diags := state.PasswordResetMethods.ElementsAs(ctx, &stateMethods, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get desired methods from plan
	var planMethods []PasswordResetMethodModel
	if !data.PasswordResetMethods.IsNull() && !data.PasswordResetMethods.IsUnknown() {
		diags := data.PasswordResetMethods.ElementsAs(ctx, &planMethods, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Delete methods that are no longer in the plan
	stateTargets := make(map[string]bool)
	for _, method := range stateMethods {
		stateTargets[method.Target.ValueString()] = true
	}

	planTargets := make(map[string]bool)
	for _, method := range planMethods {
		planTargets[method.Target.ValueString()] = true
	}

	for _, method := range stateMethods {
		target := method.Target.ValueString()
		if !planTargets[target] {
			// Delete this method
			delResp, err := r.client.DeletePasswordResetMethod(ctx, api.DeletePasswordResetRequest{
				UserName: data.UserName.ValueString(),
				Target:   target,
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete password reset method: %s", err))
				return
			}
			defer delResp.Body.Close()

			if delResp.StatusCode != http.StatusOK {
				resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete password reset method (status: %d)", delResp.StatusCode))
				return
			}
		}
	}

	// Upsert methods in the plan
	for _, method := range planMethods {
		upsertReq := api.UpsertPasswordResetRequest{
			UserName: data.UserName.ValueString(),
			Type:     method.Type.ValueString(),
			Target:   method.Target.ValueString(),
		}

		if !method.Description.IsNull() && !method.Description.IsUnknown() {
			desc := method.Description.ValueString()
			upsertReq.Description = &desc
		}

		if !method.AllowMfaReset.IsNull() && !method.AllowMfaReset.IsUnknown() {
			allow := method.AllowMfaReset.ValueBool()
			upsertReq.AllowMfaReset = &allow
		}

		upsertResp, err := r.client.CreateOrUpdatePasswordResetMethod(ctx, upsertReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to upsert password reset method: %s", err))
			return
		}
		defer upsertResp.Body.Close()

		if upsertResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to upsert password reset method (status: %d)", upsertResp.StatusCode))
			return
		}
	}

	// Step 4: Enable 2FA if it's being turned on (after password reset methods are configured)
	if (!state.RequireTwoFactorAuthentication.IsNull() && !state.RequireTwoFactorAuthentication.ValueBool() &&
		!data.RequireTwoFactorAuthentication.IsNull() && data.RequireTwoFactorAuthentication.ValueBool()) ||
		(state.RequireTwoFactorAuthentication.IsNull() &&
			!data.RequireTwoFactorAuthentication.IsNull() && data.RequireTwoFactorAuthentication.ValueBool()) {

		enable2FAReq := api.ModifyUserJSONRequestBody{
			UserName:                       data.UserName.ValueString(),
			RequireTwoFactorAuthentication: valueBoolPtr(data.RequireTwoFactorAuthentication),
		}

		enable2FAResp, err := r.client.ModifyUser(ctx, enable2FAReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to enable 2FA: %s", err))
			return
		}
		defer enable2FAResp.Body.Close()

		if enable2FAResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to enable 2FA (status: %d)", enable2FAResp.StatusCode))
			return
		}
	}

	// Read back the user to get current state
	if _, err := r.readUser(ctx, &data); err != nil {
		resp.Diagnostics.AddWarning("Read Warning", fmt.Sprintf("Updated user but unable to read back: %s", err))
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

	// Note: Password reset methods are automatically deleted when the user is deleted
	// No need to explicitly delete them

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

// readUser reads the user data from the API including password reset methods
// Returns (found bool, error)
func (r *UserResource) readUser(ctx context.Context, data *UserResourceModel) (bool, error) {
	// Read user from API
	httpResp, err := r.client.GetUser(ctx, api.GetUserJSONRequestBody{
		UserName: data.UserName.ValueString(),
	})
	if err != nil {
		return false, fmt.Errorf("unable to read user: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound || httpResp.StatusCode == 404 {
		return false, nil
	}

	if httpResp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API returned status %d", httpResp.StatusCode)
	}

	// Decode user response
	var getUserResp api.GetUserResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&getUserResp); err != nil {
		return false, fmt.Errorf("unable to decode user response: %w", err)
	}

	// Update state from API response
	if getUserResp.Result != nil {
		if getUserResp.Result.EnableSearchIndexing != nil {
			data.EnableSearchIndexing = types.BoolValue(*getUserResp.Result.EnableSearchIndexing)
		} else {
			data.EnableSearchIndexing = types.BoolValue(false)
		}
		if getUserResp.Result.RequireTwoFactorAuthentication != nil {
			data.RequireTwoFactorAuthentication = types.BoolValue(*getUserResp.Result.RequireTwoFactorAuthentication)
		} else {
			data.RequireTwoFactorAuthentication = types.BoolValue(false)
		}
	} else {
		data.EnableSearchIndexing = types.BoolValue(false)
		data.RequireTwoFactorAuthentication = types.BoolValue(false)
	}

	// Read password reset methods
	listResp, err := r.client.ListPasswordResetMethods(ctx, api.ListPasswordResetRequest{
		UserName: data.UserName.ValueString(),
	})
	if err != nil {
		return false, fmt.Errorf("unable to list password reset methods: %w", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode == http.StatusOK {
		var listPasswordResetResp api.ListPasswordResetResponse
		if err := json.NewDecoder(listResp.Body).Decode(&listPasswordResetResp); err == nil {
			if listPasswordResetResp.Result != nil && listPasswordResetResp.Result.Users != nil {
				// Convert API response to Terraform model
				methodModels := []PasswordResetMethodModel{}
				for _, method := range *listPasswordResetResp.Result.Users {
					model := PasswordResetMethodModel{}
					if method.Type != nil {
						model.Type = types.StringValue(*method.Type)
					}
					if method.Target != nil {
						model.Target = types.StringValue(*method.Target)
					}
					if method.Description != nil {
						model.Description = types.StringValue(*method.Description)
					} else {
						model.Description = types.StringNull()
					}
					if method.AllowMfaReset != nil {
						model.AllowMfaReset = types.BoolValue(*method.AllowMfaReset)
					} else {
						model.AllowMfaReset = types.BoolValue(false)
					}
					methodModels = append(methodModels, model)
				}

				// Convert to types.List
				elementType := types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"type":            types.StringType,
						"target":          types.StringType,
						"description":     types.StringType,
						"allow_mfa_reset": types.BoolType,
					},
				}

				if len(methodModels) > 0 {
					elements := []attr.Value{}
					for _, method := range methodModels {
						obj, diags := types.ObjectValue(
							elementType.AttrTypes,
							map[string]attr.Value{
								"type":            method.Type,
								"target":          method.Target,
								"description":     method.Description,
								"allow_mfa_reset": method.AllowMfaReset,
							},
						)
						if diags.HasError() {
							return false, fmt.Errorf("unable to create object value for password reset method")
						}
						elements = append(elements, obj)
					}

					listValue, diags := types.ListValue(elementType, elements)
					if diags.HasError() {
						return false, fmt.Errorf("unable to create list value for password reset methods")
					}
					data.PasswordResetMethods = listValue
				} else {
					// Empty list
					data.PasswordResetMethods = types.ListNull(elementType)
				}
			} else {
				// No password reset methods
				elementType := types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"type":            types.StringType,
						"target":          types.StringType,
						"description":     types.StringType,
						"allow_mfa_reset": types.BoolType,
					},
				}
				data.PasswordResetMethods = types.ListNull(elementType)
			}
		}
	}

	return true, nil
}
