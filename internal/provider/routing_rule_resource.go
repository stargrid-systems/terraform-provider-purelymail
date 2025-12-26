// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RoutingRuleResource{}
var _ resource.ResourceWithImportState = &RoutingRuleResource{}

func NewRoutingRuleResource() resource.Resource {
	return &RoutingRuleResource{}
}

// RoutingRuleResource defines the resource implementation.
type RoutingRuleResource struct {
	client *api.Client
}

// RoutingRuleResourceModel describes the resource data model.
type RoutingRuleResourceModel struct {
	Id              types.Int64  `tfsdk:"id"`
	DomainName      types.String `tfsdk:"domain_name"`
	Prefix          types.Bool   `tfsdk:"prefix"`
	MatchUser       types.String `tfsdk:"match_user"`
	TargetAddresses types.List   `tfsdk:"target_addresses"`
	Catchall        types.Bool   `tfsdk:"catchall"`
}

func (r *RoutingRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routing_rule"
}

func (r *RoutingRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Purelymail routing rule for a domain.",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "The routing rule ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"domain_name": schema.StringAttribute{
				MarkdownDescription: "The domain name for this routing rule.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prefix": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a prefix match (true) or exact match (false).",
				Required:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"match_user": schema.StringAttribute{
				MarkdownDescription: "The username/prefix to match for routing.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"target_addresses": schema.ListAttribute{
				MarkdownDescription: "List of target email addresses to route matching emails to.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"catchall": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a catch-all rule.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *RoutingRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RoutingRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoutingRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert target addresses from types.List to []string
	var targetAddresses []string
	resp.Diagnostics.Append(data.TargetAddresses.ElementsAs(ctx, &targetAddresses, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create routing rule via API
	createReq := api.CreateRoutingRequest{
		DomainName:      data.DomainName.ValueString(),
		Prefix:          data.Prefix.ValueBool(),
		MatchUser:       data.MatchUser.ValueString(),
		TargetAddresses: targetAddresses,
	}

	if !data.Catchall.IsNull() {
		catchall := data.Catchall.ValueBool()
		createReq.Catchall = &catchall
	}

	httpResp, err := r.client.CreateRoutingRule(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create routing rule: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to create routing rule (status: %d)", httpResp.StatusCode))
		return
	}

	// List routing rules to find the newly created one
	if err := r.readRoutingRule(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read routing rule after creation: %s", err))
		return
	}

	// Set catchall default if not specified
	if data.Catchall.IsNull() {
		data.Catchall = types.BoolValue(false)
	}

	tflog.Trace(ctx, "created purelymail_routing_rule resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoutingRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoutingRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readRoutingRule(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read routing rule: %s", err))
		return
	}

	tflog.Trace(ctx, "read purelymail_routing_rule resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoutingRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoutingRuleResourceModel
	var state RoutingRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Routing rules don't have an update API, so we need to delete and recreate
	// First, delete the old rule
	deleteReq := api.DeleteRoutingRequest{
		RoutingRuleId: int32(state.Id.ValueInt64()),
	}

	httpResp, err := r.client.DeleteRoutingRule(ctx, deleteReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete routing rule during update: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete routing rule during update (status: %d)", httpResp.StatusCode))
		return
	}

	// Convert target addresses from types.List to []string
	var targetAddresses []string
	resp.Diagnostics.Append(data.TargetAddresses.ElementsAs(ctx, &targetAddresses, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create the new rule
	createReq := api.CreateRoutingRequest{
		DomainName:      data.DomainName.ValueString(),
		Prefix:          data.Prefix.ValueBool(),
		MatchUser:       data.MatchUser.ValueString(),
		TargetAddresses: targetAddresses,
	}

	if !data.Catchall.IsNull() {
		catchall := data.Catchall.ValueBool()
		createReq.Catchall = &catchall
	}

	httpResp, err = r.client.CreateRoutingRule(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create routing rule during update: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to create routing rule during update (status: %d)", httpResp.StatusCode))
		return
	}

	// Clear the old ID so readRoutingRule will search by domain/matchUser/prefix
	data.Id = types.Int64Unknown()

	// Read back the new rule to get the new ID
	if err := r.readRoutingRule(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read routing rule after update: %s", err))
		return
	}

	tflog.Trace(ctx, "updated purelymail_routing_rule resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoutingRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoutingRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteReq := api.DeleteRoutingRequest{
		RoutingRuleId: int32(data.Id.ValueInt64()),
	}

	httpResp, err := r.client.DeleteRoutingRule(ctx, deleteReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete routing rule: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete routing rule (status: %d)", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted purelymail_routing_rule resource")
}

func (r *RoutingRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by ID
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Invalid routing rule ID: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// readRoutingRule reads a routing rule from the API and updates the model.
func (r *RoutingRuleResource) readRoutingRule(ctx context.Context, data *RoutingRuleResourceModel) error {
	httpResp, err := r.client.ListRoutingRules(ctx, api.EmptyRequest{})
	if err != nil {
		return fmt.Errorf("unable to list routing rules: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list routing rules (status: %d)", httpResp.StatusCode)
	}

	var listResp api.ListRoutingResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&listResp); err != nil {
		return fmt.Errorf("unable to decode response: %w", err)
	}

	if listResp.Result == nil || listResp.Result.Rules == nil || len(*listResp.Result.Rules) == 0 {
		return fmt.Errorf("no routing rules found in API response")
	}

	// Find the matching rule
	var foundRule *api.RoutingRule
	if !data.Id.IsNull() && !data.Id.IsUnknown() {
		// Search by ID
		targetId := int32(data.Id.ValueInt64())
		for _, rule := range *listResp.Result.Rules {
			if rule.Id != nil && *rule.Id == targetId {
				ruleCopy := rule
				foundRule = &ruleCopy
				break
			}
		}
	} else {
		// Search by domain, matchUser, and prefix (for newly created rules)
		for i, rule := range *listResp.Result.Rules {
			domainStr := "<nil>"
			matchUserStr := "<nil>"
			prefixStr := "<nil>"
			if rule.DomainName != nil {
				domainStr = *rule.DomainName
			}
			if rule.MatchUser != nil {
				matchUserStr = *rule.MatchUser
			}
			if rule.Prefix != nil {
				prefixStr = fmt.Sprintf("%v", *rule.Prefix)
			}

			tflog.Debug(ctx, "Checking rule", map[string]interface{}{
				"index":         i,
				"domain":        domainStr,
				"match_user":    matchUserStr,
				"prefix":        prefixStr,
				"target_domain": data.DomainName.ValueString(),
				"target_match":  data.MatchUser.ValueString(),
				"target_prefix": data.Prefix.ValueBool(),
			})
			if rule.DomainName != nil && *rule.DomainName == data.DomainName.ValueString() &&
				rule.MatchUser != nil && *rule.MatchUser == data.MatchUser.ValueString() &&
				rule.Prefix != nil && *rule.Prefix == data.Prefix.ValueBool() {
				ruleCopy := rule
				foundRule = &ruleCopy
				tflog.Debug(ctx, "Found matching rule!")
				break
			}
		}
	}

	if foundRule == nil {
		return fmt.Errorf("routing rule not found (searched %d rules, looking for domain=%s, matchUser=%s, prefix=%v, id=%v)",
			len(*listResp.Result.Rules), data.DomainName.ValueString(), data.MatchUser.ValueString(),
			data.Prefix.ValueBool(), data.Id)
	}

	// Update the model
	if foundRule.Id != nil {
		data.Id = types.Int64Value(int64(*foundRule.Id))
	}
	if foundRule.DomainName != nil {
		data.DomainName = types.StringValue(*foundRule.DomainName)
	}
	if foundRule.Prefix != nil {
		data.Prefix = types.BoolValue(*foundRule.Prefix)
	}
	if foundRule.MatchUser != nil {
		data.MatchUser = types.StringValue(*foundRule.MatchUser)
	}
	if foundRule.TargetAddresses != nil {
		targetList, diags := types.ListValueFrom(ctx, types.StringType, *foundRule.TargetAddresses)
		if diags.HasError() {
			return fmt.Errorf("unable to convert target addresses")
		}
		data.TargetAddresses = targetList
	}
	if foundRule.Catchall != nil {
		data.Catchall = types.BoolValue(*foundRule.Catchall)
	} else {
		data.Catchall = types.BoolValue(false)
	}

	return nil
}
