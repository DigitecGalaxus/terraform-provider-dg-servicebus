package endpoint

import (
	"context"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ resource.Resource                = &endpointResource{}
	_ resource.ResourceWithConfigure   = &endpointResource{}
	_ resource.ResourceWithImportState = &endpointResource{}
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

	r.client = &asb.AsbClientWrapper{
		Client: req.ProviderData.(*az.Client),
	}
}

func (r *endpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}
