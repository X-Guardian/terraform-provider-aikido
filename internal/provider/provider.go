package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ provider.Provider = &AikidoProvider{}

// AikidoProvider defines the provider implementation.
type AikidoProvider struct {
	version string
}

// AikidoProviderModel describes the provider data model.
type AikidoProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Region       types.String `tfsdk:"region"`
	ApiUrl       types.String `tfsdk:"api_url"`
}

var regionURLs = map[string]string{
	"eu": "https://app.aikido.dev",
	"us": "https://app.us.aikido.dev",
	"me": "https://app.me.aikido.dev",
}

func (p *AikidoProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "aikido"
	resp.Version = p.version
}

func (p *AikidoProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Aikido provider allows you to manage resources in [Aikido Security](https://www.aikido.dev/) via the management API.",
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				MarkdownDescription: "The OAuth2 client ID for the Aikido API. Can also be set via the `AIKIDO_CLIENT_ID` environment variable.",
				Optional:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "The OAuth2 client secret for the Aikido API. Can also be set via the `AIKIDO_CLIENT_SECRET` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The Aikido region. Valid values: `eu`, `us`, `me`. Defaults to `eu`. Can also be set via the `AIKIDO_REGION` environment variable.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("eu", "us", "me"),
				},
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "Override the Aikido API base URL. Takes precedence over `region`. Can also be set via the `AIKIDO_API_URL` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *AikidoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data AikidoProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve client_id: config > env
	clientID := data.ClientID.ValueString()
	if clientID == "" {
		clientID = os.Getenv("AIKIDO_CLIENT_ID")
	}
	if clientID == "" {
		resp.Diagnostics.AddError(
			"Missing Aikido Client ID",
			"The provider requires a client_id to be set in the provider configuration or via the AIKIDO_CLIENT_ID environment variable.",
		)
	}

	// Resolve client_secret: config > env
	clientSecret := data.ClientSecret.ValueString()
	if clientSecret == "" {
		clientSecret = os.Getenv("AIKIDO_CLIENT_SECRET")
	}
	if clientSecret == "" {
		resp.Diagnostics.AddError(
			"Missing Aikido Client Secret",
			"The provider requires a client_secret to be set in the provider configuration or via the AIKIDO_CLIENT_SECRET environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve base URL: api_url config > AIKIDO_API_URL env > region config > AIKIDO_REGION env > default "eu"
	baseURL := data.ApiUrl.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("AIKIDO_API_URL")
	}
	if baseURL == "" {
		region := data.Region.ValueString()
		if region == "" {
			region = os.Getenv("AIKIDO_REGION")
		}
		if region == "" {
			region = "eu"
		}
		var ok bool
		baseURL, ok = regionURLs[region]
		if !ok {
			resp.Diagnostics.AddError(
				"Invalid Aikido Region",
				"The region must be one of: eu, us, me. Got: "+region,
			)
			return
		}
	}

	aikidoClient := client.NewAikidoClient(baseURL, clientID, clientSecret)

	resp.DataSourceData = aikidoClient
	resp.ResourceData = aikidoClient
}

func (p *AikidoProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTeamResource,
		NewTeamMembershipResource,
		NewTeamResourceLinkResource,
		NewCodeRepoConfigResource,
		NewCloudAWSResource,
		NewCloudAzureResource,
		NewCloudGCPResource,
		NewCloudKubernetesResource,
		NewContainerConfigResource,
	}
}

func (p *AikidoProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTeamsDataSource,
		NewUsersDataSource,
		NewCodeReposDataSource,
		NewCloudsDataSource,
		NewContainersDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AikidoProvider{
			version: version,
		}
	}
}
