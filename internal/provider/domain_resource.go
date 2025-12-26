package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

// DomainResource defines the resource implementation.
type DomainResource struct {
	client *api.Client
}

// DomainResourceModel describes the resource data model.
type DomainResourceModel struct {
	Name                  types.String `tfsdk:"name"`
	AllowAccountReset     types.Bool   `tfsdk:"allow_account_reset"`
	SymbolicSubaddressing types.Bool   `tfsdk:"symbolic_subaddressing"`
	RecheckDns            types.Bool   `tfsdk:"recheck_dns"`
	IsShared              types.Bool   `tfsdk:"is_shared"`
	DnsSummary            types.Object `tfsdk:"dns_summary"`
	Id                    types.String `tfsdk:"id"`
}

// DnsSummaryModel describes the DNS summary nested object.
type DnsSummaryModel struct {
	PassesMx    types.Bool `tfsdk:"passes_mx"`
	PassesSpf   types.Bool `tfsdk:"passes_spf"`
	PassesDkim  types.Bool `tfsdk:"passes_dkim"`
	PassesDmarc types.Bool `tfsdk:"passes_dmarc"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Purelymail domain. Requires DNS ownership verification.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The domain name (e.g., example.com).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allow_account_reset": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow password reset via email for this domain.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"symbolic_subaddressing": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable symbolic subaddressing (e.g., user+tag@domain.com).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"recheck_dns": schema.BoolAttribute{
				MarkdownDescription: "Set to true to force DNS recheck on update. Not stored in state.",
				Optional:            true,
			},
			"is_shared": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a shared domain.",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"dns_summary": schema.SingleNestedAttribute{
				MarkdownDescription: "DNS verification status for the domain.",
				Computed:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"passes_mx": schema.BoolAttribute{
						MarkdownDescription: "Whether MX records are correctly configured.",
						Computed:            true,
					},
					"passes_spf": schema.BoolAttribute{
						MarkdownDescription: "Whether SPF records are correctly configured.",
						Computed:            true,
					},
					"passes_dkim": schema.BoolAttribute{
						MarkdownDescription: "Whether DKIM records are correctly configured.",
						Computed:            true,
					},
					"passes_dmarc": schema.BoolAttribute{
						MarkdownDescription: "Whether DMARC records are correctly configured.",
						Computed:            true,
					},
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The domain identifier (same as name).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add domain via API
	addReq := api.AddDomainRequest{
		DomainName: data.Name.ValueString(),
	}

	httpResp, err := r.client.AddDomain(ctx, addReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add domain: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to add domain (status: %d)", httpResp.StatusCode))
		return
	}

	// Set ID
	data.Id = types.StringValue(data.Name.ValueString())

	// Update settings if non-default values were provided
	if !data.AllowAccountReset.IsNull() || !data.SymbolicSubaddressing.IsNull() {
		if err := r.updateDomainSettings(ctx, &data); err != nil {
			resp.Diagnostics.AddError("Update Error", fmt.Sprintf("Domain created but failed to update settings: %s", err))
			return
		}
	}

	// Read back domain info to get DNS summary
	if err := r.readDomain(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read domain after creation: %s", err))
		return
	}

	tflog.Trace(ctx, "created purelymail_domain resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readDomain(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read domain: %s", err))
		return
	}

	tflog.Trace(ctx, "read purelymail_domain resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update domain settings
	if err := r.updateDomainSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Update Error", fmt.Sprintf("Unable to update domain settings: %s", err))
		return
	}

	// Read back domain info
	if err := r.readDomain(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read domain after update: %s", err))
		return
	}

	tflog.Trace(ctx, "updated purelymail_domain resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteReq := api.DeleteDomainRequest{
		Name: data.Name.ValueString(),
	}

	httpResp, err := r.client.DeleteDomain(ctx, deleteReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete domain: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete domain (status: %d)", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted purelymail_domain resource")
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// updateDomainSettings updates the domain settings via API.
func (r *DomainResource) updateDomainSettings(ctx context.Context, data *DomainResourceModel) error {
	updateReq := api.UpdateDomainSettingsRequest{
		Name: data.Name.ValueString(),
	}

	if !data.AllowAccountReset.IsNull() {
		allowReset := data.AllowAccountReset.ValueBool()
		updateReq.AllowAccountReset = &allowReset
	}

	if !data.SymbolicSubaddressing.IsNull() {
		symbolicSub := data.SymbolicSubaddressing.ValueBool()
		updateReq.SymbolicSubaddressing = &symbolicSub
	}

	if !data.RecheckDns.IsNull() && data.RecheckDns.ValueBool() {
		recheckDns := true
		updateReq.RecheckDns = &recheckDns
	}

	httpResp, err := r.client.UpdateDomainSettings(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("client error: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", httpResp.StatusCode)
	}

	return nil
}

// readDomain reads domain information from the API.
func (r *DomainResource) readDomain(ctx context.Context, data *DomainResourceModel) error {
	includeShared := false
	listReq := api.ListDomainsRequest{
		IncludeShared: &includeShared,
	}

	httpResp, err := r.client.ListDomains(ctx, listReq)
	if err != nil {
		return fmt.Errorf("unable to list domains: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", httpResp.StatusCode)
	}

	var listResp api.ListDomainsResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&listResp); err != nil {
		return fmt.Errorf("unable to decode response: %w", err)
	}

	if listResp.Result == nil {
		return fmt.Errorf("no result in response")
	}
	if listResp.Result.Domains == nil {
		return fmt.Errorf("no domains field in result")
	}
	if len(*listResp.Result.Domains) == 0 {
		return fmt.Errorf("domains list is empty")
	}

	// Find the domain
	var foundDomain *api.ApiDomainInfo
	targetName := data.Name.ValueString()
	for _, domain := range *listResp.Result.Domains {
		if domain.Name != nil && *domain.Name == targetName {
			domainCopy := domain
			foundDomain = &domainCopy
			break
		}
	}

	if foundDomain == nil {
		return fmt.Errorf("domain not found: %s", targetName)
	}

	// Update the model
	data.Id = types.StringValue(targetName)

	if foundDomain.AllowAccountReset != nil {
		data.AllowAccountReset = types.BoolValue(*foundDomain.AllowAccountReset)
	} else {
		data.AllowAccountReset = types.BoolValue(false)
	}

	if foundDomain.SymbolicSubaddressing != nil {
		data.SymbolicSubaddressing = types.BoolValue(*foundDomain.SymbolicSubaddressing)
	} else {
		data.SymbolicSubaddressing = types.BoolValue(false)
	}

	if foundDomain.IsShared != nil {
		data.IsShared = types.BoolValue(*foundDomain.IsShared)
	} else {
		data.IsShared = types.BoolValue(false)
	}

	// Update DNS summary
	if foundDomain.DnsSummary != nil {
		dnsSummary := DnsSummaryModel{
			PassesMx:    types.BoolValue(foundDomain.DnsSummary.PassesMx != nil && *foundDomain.DnsSummary.PassesMx),
			PassesSpf:   types.BoolValue(foundDomain.DnsSummary.PassesSpf != nil && *foundDomain.DnsSummary.PassesSpf),
			PassesDkim:  types.BoolValue(foundDomain.DnsSummary.PassesDkim != nil && *foundDomain.DnsSummary.PassesDkim),
			PassesDmarc: types.BoolValue(foundDomain.DnsSummary.PassesDmarc != nil && *foundDomain.DnsSummary.PassesDmarc),
		}
		dnsSummaryObj, diags := types.ObjectValueFrom(ctx, data.DnsSummary.AttributeTypes(ctx), dnsSummary)
		if diags.HasError() {
			return fmt.Errorf("unable to convert DNS summary to object")
		}
		data.DnsSummary = dnsSummaryObj
	}

	return nil
}
