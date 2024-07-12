package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ resource.Resource                 = &endpointResource{}
	_ resource.ResourceWithConfigure    = &endpointResource{}
	_ resource.ResourceWithImportState  = &endpointResource{}
	_ resource.ResourceWithUpgradeState = &endpointResource{}
)

func NewEndpointResource() resource.Resource {
	return &endpointResource{}
}

type endpointResource struct {
	client *asb.AsbClientWrapper
}

func (r *endpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*az.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configuration Type",
			fmt.Sprintf("Expected *azservicebus.Client, got %T", req.ProviderData),
		)
		return
	}

	r.client = &asb.AsbClientWrapper{
		Client: client,
	}
}

func (r *endpointResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (r *endpointResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = NewSchemaV1()
}
