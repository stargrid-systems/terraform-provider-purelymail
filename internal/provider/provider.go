// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Ensure PurelymailProvider satisfies various provider interfaces.
var _ provider.Provider = &PurelymailProvider{}
var _ provider.ProviderWithFunctions = &PurelymailProvider{}
var _ provider.ProviderWithEphemeralResources = &PurelymailProvider{}
var _ provider.ProviderWithActions = &PurelymailProvider{}

// PurelymailProvider defines the provider implementation.
type PurelymailProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// PurelymailProviderModel describes the provider data model.
type PurelymailProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	ApiToken types.String `tfsdk:"api_token"`
}

func (p *PurelymailProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "purelymail"
	resp.Version = p.version
}

func (p *PurelymailProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "API endpoint URL. Defaults to https://purelymail.com",
				Optional:            true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "API authentication token. Required for API access.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *PurelymailProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data PurelymailProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	httpClient := http.DefaultClient

	baseURL := api.ServerUrlHttpspurelymailCom
	if !data.Endpoint.IsNull() && !data.Endpoint.IsUnknown() {
		baseURL = data.Endpoint.ValueString()
	}

	// Build client with optional authentication
	clientOpts := []api.ClientOption{
		api.WithHTTPClient(httpClient),
	}

	if !data.ApiToken.IsNull() && !data.ApiToken.IsUnknown() {
		apiToken := data.ApiToken.ValueString()
		apiKeyProvider, err := securityprovider.NewSecurityProviderApiKey("header", "Purelymail-Api-Token", apiToken)
		if err != nil {
			resp.Diagnostics.AddError("Security Provider Error", err.Error())
			return
		}
		clientOpts = append(clientOpts, api.WithRequestEditorFn(apiKeyProvider.Intercept))
	}

	client, err := api.NewClient(baseURL, clientOpts...)
	if err != nil {
		resp.Diagnostics.AddError("Client Initialization Error", err.Error())
		return
	}
	resp.DataSourceData = client
	resp.ResourceData = client
	resp.EphemeralResourceData = client
}

func (p *PurelymailProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserResource,
		NewRoutingRuleResource,
		NewAppPasswordResource,
		NewDomainResource,
	}
}

func (p *PurelymailProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewAppPasswordEphemeralResource,
	}
}

func (p *PurelymailProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOwnershipProofDataSource,
	}
}

func (p *PurelymailProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func (p *PurelymailProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PurelymailProvider{
			version: version,
		}
	}
}
