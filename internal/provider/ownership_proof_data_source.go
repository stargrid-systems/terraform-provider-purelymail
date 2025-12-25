package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &OwnershipProofDataSource{}

func NewOwnershipProofDataSource() datasource.DataSource {
	return &OwnershipProofDataSource{}
}

// OwnershipProofDataSource implements the purelymail_ownership_proof data source.
type OwnershipProofDataSource struct {
	client *api.Client
}

// OwnershipProofDataSourceModel is the state model.
type OwnershipProofDataSourceModel struct {
	Code types.String `tfsdk:"code"`
	Id   types.String `tfsdk:"id"`
}

func (d *OwnershipProofDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ownership_proof"
}

func (d *OwnershipProofDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves the Purelymail ownership proof DNS TXT record value.",
		Attributes: map[string]schema.Attribute{
			"code": schema.StringAttribute{
				MarkdownDescription: "The TXT record value to prove ownership.",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier for this data source instance (same as code).",
				Computed:            true,
			},
		},
	}
}

func (d *OwnershipProofDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *OwnershipProofDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state OwnershipProofDataSourceModel

	httpResp, err := d.client.GetOwnershipCode(ctx, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get ownership code: %s", err))
		return
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unexpected status code: %d", httpResp.StatusCode))
		return
	}

	var decoded api.GetOwnershipCodeResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&decoded); err != nil {
		resp.Diagnostics.AddError("Decode Error", fmt.Sprintf("Unable to decode response: %s", err))
		return
	}
	if decoded.Result == nil || decoded.Result.Code == nil {
		resp.Diagnostics.AddError("Response Error", "Missing ownership code in response")
		return
	}

	state.Code = types.StringValue(*decoded.Result.Code)
	state.Id = types.StringValue(*decoded.Result.Code)

	tflog.Trace(ctx, "read purelymail_ownership_proof data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
