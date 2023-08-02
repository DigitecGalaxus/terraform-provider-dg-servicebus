package provider

import (
	"context"
	"os"
	"terraform-provider-dg-servicebus/internal/provider/endpoint"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &DgServicebusProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &DgServicebusProvider{
			version: version,
		}
	}
}

type DgServicebusProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type DgServicebusProviderModel struct {
	Hostname     types.String `tfsdk:"azure_servicebus_hostname"`
	TenantId     types.String `tfsdk:"tenant_id"`
	ClientId     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

// Metadata returns the provider type name.
func (p *DgServicebusProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "dgservicebus"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *DgServicebusProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"azure_servicebus_hostname": schema.StringAttribute{
				Required:    true,
				Sensitive:   false,
				Description: "The hostname of the Azure Service Bus instance",
			},
			"tenant_id": schema.StringAttribute{
				Optional: true,
				Sensitive: false,
				Description: "The Tenant ID which should be used. This can also be sourced from the `DG_SERVICEBUS_TENANTID` Environment Variable.",
			},
			"client_id": schema.StringAttribute{
				Optional: true,
				Sensitive: false,
				Description: "The Client ID which should be used. This can also be sourced from the `DG_SERVICEBUS_CLIENTID` Environment Variable.",
			},
			"client_secret": schema.StringAttribute{
				Optional: true,
				Sensitive: true,
				Description: "The Client Secret which should be used. This can also be sourced from the `DG_SERVICEBUS_CLIENTSECRET` Environment Variable.",
			},
		},
	}
}

func (p *DgServicebusProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring HashiCups client")

	// Retrieve provider data from configuration
	var config DgServicebusProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.ClientId.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Unknown Client Id",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the Client Id. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_HOST environment variable.",
		)
	}

	if config.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Unknown Client Secret",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the Client Secret. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_HOST environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	tenantId := os.Getenv("DG_SERVICEBUS_TENANTID")
	clientId := os.Getenv("DG_SERVICEBUS_CLIENTID")
	clientSecret := os.Getenv("DG_SERVICEBUS_CLIENTSECRET")
	

	if !config.TenantId.IsNull() {
		tenantId = config.TenantId.ValueString()
	}

	if !config.ClientId.IsNull() {
		clientId = config.ClientId.ValueString()
	}

	if !config.ClientSecret.IsNull() {
		clientSecret = config.ClientSecret.ValueString()
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "dgservicebus_client_secret", clientSecret)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "dgservicebus_client_secret")

	tflog.Debug(ctx, "Creating Azure Authenticaion Credential")

	var credential azcore.TokenCredential
	var err error

	if tenantId != "" && clientId != "" && clientSecret != "" {
		credential, err = azidentity.NewClientSecretCredential(tenantId, clientId, clientSecret, nil)
	} else {
		credential, err = azidentity.NewDefaultAzureCredential(nil)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Azure Client",
			"Authentication failed. Either provide the credentials 'tenant_id', 'client_id' and 'client_secret' "+
				"or log in your shell with the Azure CLI 'az login'"+
				"Azure Client Error: "+err.Error(),
		)
		return
	}

	client, err := azservicebus.NewClient(config.Hostname.ValueString(), credential, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"An error occurred while configuring the provider",
			"Could not create Azure Service Bus client: "+err.Error(),
		)

		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured HashiCups client", map[string]any{"success": true})
}

func (p *DgServicebusProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		endpoint.NewEndpointDataSource,
	}
}

func (p *DgServicebusProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		endpoint.NewEndpointResource,
	}
}
