package endpoint

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"strings"
	"terraform-provider-dg-servicebus/internal/provider/asb"
)

func (r *endpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	// time.Sleep(10 * time.Second)

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import Identifier",
			fmt.Sprintf("Expected two identifier in format: topic_name,endpoint_name. Got %q", req.ID),
		)
		return
	}

	model := asb.EndpointModel{TopicName: idParts[0], EndpointName: idParts[1]}

	endpointExists, err := r.client.EndpointExists(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Endpoint",
			"Could not read if an Endpoint exists, unexpected error: "+err.Error(),
		)
		return
	}

	subscriptionNames := []string{}

	if endpointExists {
		subscriptions, err := r.client.GetEndpointSubscriptions(ctx, model)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error importing subscribers",
				"There was an error when trying to import subscribers: "+err.Error(),
			)
		}

		for k := range subscriptions {
			subscriptionNames = append(subscriptionNames, k)
		}
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("topic_name"), model.TopicName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_name"), model.EndpointName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("queue_options"), &endpointResourceQueueOptionsModel{})...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subscriptions"), subscriptionNames)...)
}
