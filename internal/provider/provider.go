package provider

import (
	"context"
	"os"

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
			path.Root("host"),
			"Unknown HashiCups API Host",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the HashiCups API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_HOST environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	accessToken := os.Getenv("XXXX")

	if !config.AccessToken.IsNull() {
		accessToken = config.AccessToken.ValueString()
	}

	// if accessToken == "" {
	// 	resp.Diagnostics.AddAttributeError(
	// 		path.Root("accessToken"),
	// 		"Missing HashiCups API Host",
	// 		"The provider cannot create the HashiCups API client as there is a missing or empty value for the HashiCups API host. "+
	// 			"Set the host value in the configuration or use the HASHICUPS_HOST environment variable. "+
	// 			"If either is already set, ensure the value is not empty.",
	// 	)
	// }

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "dgservicebus_access_token", accessToken)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "dgservicebus_access_token")

	tflog.Debug(ctx, "Creating Azure Authenticaion Credential")

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create HashiCups API Client",
			"An unexpected error occurred when creating the HashiCups API client. "+
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
}

func (p *DgServicebusProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEndpointResource,
	}
}
