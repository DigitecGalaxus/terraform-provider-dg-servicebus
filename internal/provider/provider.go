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
	AccessToken  types.String `tfsdk:"access_token"`
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
			"access_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			"azure_servicebus_hostname": schema.StringAttribute{
				Required:    true,
				Sensitive:   false,
				Description: "",
			},
			"tenant_id": schema.StringAttribute{
				Optional: true,
				Sensitive: false,
				Description: "",
			},
			"client_id": schema.StringAttribute{
				Optional: true,
				Sensitive: false,
				Description: "",
			},
			"client_secret": schema.StringAttribute{
				Optional: true,
				Sensitive: true,
				Description: "",
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

	if config.AccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Unknown Azure Access Token",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the HashiCups API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_HOST environment variable.",
		)
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

	accessToken := os.Getenv("DG_SERVICEBUS_ACCESSTOKEN")
	tenantId := os.Getenv("DG_SERVICEBUS_TENANTID")
	clientId := os.Getenv("DG_SERVICEBUS_CLIENTID")
	clientSecret := os.Getenv("DG_SERVICEBUS_CLIENTSECRET")
	
	if !config.AccessToken.IsNull() {
		accessToken = config.AccessToken.ValueString()
	}

	if !config.TenantId.IsNull() {
		tenantId = config.TenantId.ValueString()
	}

	if !config.ClientId.IsNull() {
		clientId = config.ClientId.ValueString()
	}

	if !config.ClientSecret.IsNull() {
		clientSecret = config.ClientSecret.ValueString()
	}

	if accessToken == "" && (tenantId == "" || clientId == "" || clientSecret == "") {
		resp.Diagnostics.AddError(
			"Missing Access Token or Client Credentials",
			"The provider cannot create the Azure ServiceBus API client as the credentials are not configured correctly."+
				"Set the Access Token or Tenant Id, Client Id and Client Secret in the configuration or use the DG_SERVICEBUS_ACCESSTOKEN or DG_SERVICEBUS_TENANTID, DG_SERVICEBUS_CLIENTID and DG_SERVICEBUS_CLIENTSECRET environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "dgservicebus_access_token", accessToken)
	ctx = tflog.SetField(ctx, "dgservicebus_client_secret", clientSecret)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "dgservicebus_access_token")
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
			"An unexpected error occurred when creating the Azure Servicebus client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"HashiCups Client Error: "+err.Error(),
		)
		return
	}

	client, err := azservicebus.NewClient(config.Hostname.ValueString(), credential, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"An error occurred while configuring the resource",
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
