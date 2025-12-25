// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ ephemeral.EphemeralResource = &AppPasswordEphemeralResource{}

func NewAppPasswordEphemeralResource() ephemeral.EphemeralResource {
	return &AppPasswordEphemeralResource{}
}

// AppPasswordEphemeralResource defines the ephemeral resource implementation.
type AppPasswordEphemeralResource struct {
	client *api.Client
}

// AppPasswordEphemeralResourceModel describes the ephemeral resource data model.
type AppPasswordEphemeralResourceModel struct {
	UserHandle  types.String `tfsdk:"user_handle"`
	Name        types.String `tfsdk:"name"`
	AppPassword types.String `tfsdk:"app_password"`
}

func (e *AppPasswordEphemeralResource) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_password"
}

func (e *AppPasswordEphemeralResource) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ephemeral app password for Purelymail. Creates a temporary app password that exists only for the duration of the Terraform operation.",

		Attributes: map[string]schema.Attribute{
			"user_handle": schema.StringAttribute{
				MarkdownDescription: "The user handle (email address or username) for which to create the app password.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Optional name/description for the app password.",
				Optional:            true,
			},
			"app_password": schema.StringAttribute{
				MarkdownDescription: "The generated app password (sensitive).",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (e *AppPasswordEphemeralResource) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	e.client = client
}

func (e *AppPasswordEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data AppPasswordEphemeralResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
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

	httpResp, err := e.client.CreateAppPassword(ctx, createReq)
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

	// Set the app password
	data.AppPassword = types.StringValue(*createResp.Result.AppPassword)

	// Store the app password for cleanup in RenewAt
	resp.Private.SetKey(ctx, "app_password", []byte(data.AppPassword.ValueString()))
	resp.Private.SetKey(ctx, "user_handle", []byte(data.UserHandle.ValueString()))

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}

func (e *AppPasswordEphemeralResource) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	// Retrieve stored credentials
	appPassword, diags := req.Private.GetKey(ctx, "app_password")
	resp.Diagnostics.Append(diags...)

	userHandle, diags := req.Private.GetKey(ctx, "user_handle")
	resp.Diagnostics.Append(diags...)

	if len(appPassword) == 0 || len(userHandle) == 0 {
		// Nothing to clean up
		return
	}

	// Delete app password via API
	deleteReq := api.DeleteAppPasswordRequest{
		UserName:    string(userHandle),
		AppPassword: string(appPassword),
	}

	httpResp, err := e.client.DeleteAppPassword(ctx, deleteReq)
	if err != nil {
		resp.Diagnostics.AddWarning("Client Error", fmt.Sprintf("Unable to delete ephemeral app password: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddWarning("API Error", fmt.Sprintf("Failed to delete ephemeral app password (status: %d)", httpResp.StatusCode))
		return
	}
}
