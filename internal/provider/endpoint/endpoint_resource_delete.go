package endpoint

import (
	"context"
	"errors"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func (r *endpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := plan.ToAsbModel()

	err := r.client.DeleteEndpointQueue(ctx, model)
	if err != nil && !statusCodeIsOk(err) {
		resp.Diagnostics.AddError(
			"Error deleting queue",
			"Could not delete queue, unexpected error: "+err.Error(),
		)
		return
	}

	err = r.client.DeleteEndpoint(ctx, model)
	if err != nil && !statusCodeIsOk(err) {
		resp.Diagnostics.AddError(
			"Error deleting subscription",
			"Could not delete subscription, unexpected error: "+err.Error(),
		)
		return
	}

	for _, queue := range plan.AdditionalQueues {
		err := r.client.DeleteAdditionalQueue(ctx, queue)
		if err != nil && !statusCodeIsOk(err) {
			resp.Diagnostics.AddError(
				"Error deleting queue",
				"Could not delete queue, unexpected error: "+err.Error(),
			)
			return
		}
	}
}


func statusCodeIsOk(err error) bool {
	var respError *azcore.ResponseError
	switch {
	case errors.As(err, &respError):
		if respError.StatusCode == http.StatusNotFound {
			return true
		}
	default:
		return false
	}
	return false
}
