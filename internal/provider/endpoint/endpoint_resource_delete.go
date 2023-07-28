package endpoint

import (
	"context"
	"strings"

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
	if err != nil && !StatusCodeIsOk(err.Error()) {
		resp.Diagnostics.AddError(
			"Error deleting queue",
			"Could not delete queue, unexpected error: "+err.Error(),
		)
		return
	}

	err = r.client.DeleteEndpoint(azureContext, model)
	if err != nil && !StatusCodeIsOk(err.Error()) {
		resp.Diagnostics.AddError(
			"Error deleting subscription",
			"Could not delete subscription, unexpected error: "+err.Error(),
		)
		return
	}
}

func StatusCodeIsOk(errorMessage string) bool {
	return strings.Contains(errorMessage, "ERROR CODE: 404")
}
