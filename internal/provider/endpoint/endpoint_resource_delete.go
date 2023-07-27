package endpoint

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func (r *endpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan endpointResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := plan.ToAsbModel()
	azureContext := context.Background()

	err := r.client.DeleteEndpointQueue(azureContext, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting queue",
			"Could not delete queue, unexpected error: "+err.Error(),
		)
		return
	}

	err = r.client.DeleteEndpoint(azureContext, model.TopicName, model.EndpointName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting subscription",
			"Could not delete subscription, unexpected error: "+err.Error(),
		)
		return
	}
}