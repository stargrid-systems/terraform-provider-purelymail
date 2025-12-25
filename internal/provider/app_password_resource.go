// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppPasswordResource{}

func NewAppPasswordResource() resource.Resource {
	return &AppPasswordResource{}
}

// AppPasswordResource defines the resource implementation.
type AppPasswordResource struct {
	client *api.Client
}

// AppPasswordResourceModel describes the resource data model.
type AppPasswordResourceModel struct {
	UserHandle  types.String `tfsdk:"user_handle"`
	Name        types.String `tfsdk:"name"`
	AppPassword types.String `tfsdk:"app_password"`
	Id          types.String `tfsdk:"id"`
}

func (r *AppPasswordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_password"
}

func (r *AppPasswordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Purelymail app password. App passwords are alternative credentials for email clients.",

		Attributes: map[string]schema.Attribute{
			"user_handle": schema.StringAttribute{
				MarkdownDescription: "The user handle (email address or username) for which to create the app password.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Optional name/description for the app password.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_password": schema.StringAttribute{
				MarkdownDescription: "The generated app password (sensitive, only available after creation).",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The app password identifier (same as app_password).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AppPasswordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AppPasswordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppPasswordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create app password via API
	createReq := api.CreateAppPassword{
		UserHandle: data.UserHandle.ValueString(),
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		name := data.Name.ValueString()
		createReq.Name = &name
	}

	httpResp, err := r.client.CreateAppPassword(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create app password: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to create app password (status: %d)", httpResp.StatusCode))
		return
	}

	var createResp api.CreateAppPasswordResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&createResp); err != nil {
		resp.Diagnostics.AddError("Decode Error", fmt.Sprintf("Unable to decode response: %s", err))
		return
	}

	if createResp.Result == nil || createResp.Result.AppPassword == nil {
		resp.Diagnostics.AddError("API Error", "App password not returned in response")
		return
	}

	// Set the app password and ID
	data.AppPassword = types.StringValue(*createResp.Result.AppPassword)
	data.Id = types.StringValue(*createResp.Result.AppPassword)

	// Set default name if not provided
	if data.Name.IsNull() {
		data.Name = types.StringValue("")
	}

	tflog.Trace(ctx, "created purelymail_app_password resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppPasswordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppPasswordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// App passwords cannot be read back from the API - they're write-only
	// We just keep what's in state
	tflog.Trace(ctx, "read purelymail_app_password resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppPasswordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// App passwords don't support updates - all fields require replacement
	resp.Diagnostics.AddError("Update Not Supported", "App passwords cannot be updated. All changes require replacement.")
}

func (r *AppPasswordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppPasswordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete app password via API
	deleteReq := api.DeleteAppPasswordRequest{
		UserName:    data.UserHandle.ValueString(),
		AppPassword: data.AppPassword.ValueString(),
	}

	httpResp, err := r.client.DeleteAppPassword(ctx, deleteReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete app password: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete app password (status: %d)", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted purelymail_app_password resource")
}
