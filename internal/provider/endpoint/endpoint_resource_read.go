package endpoint

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// model := state.ToAsbModel()

	state.QueueExists = types.BoolValue(true)
	state.EndpointExists = types.BoolValue(true)
	state.ShouldCreateQueue = types.BoolValue(false)
	state.ShouldCreateEndpoint = types.BoolValue(false)

	// Assert queue exists
	queue, err := r.client.GetEndpointQueue(ctx, state.EndpointName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Queue",
			"Could not get Queue, unexpected error: "+err.Error(),
		)
		return
	}

	// When queue doesnt exist anymore we need to reset the resource. Delete the Endpoint if it exists.
	if queue == nil {
		state.QueueExists = types.BoolValue(false)
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Endpoint Queue %s is missing", state.EndpointName.ValueString()),
			fmt.Sprintf("Endpoint Queue %s does not exists on Azure, whole resource will be recreated at apply", state.EndpointName.ValueString()),
		)
	}

	endpoint_exists, err := r.client.EndpointExists(
		ctx,
		state.TopicName.ValueString(),
		state.EndpointName.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Endpoint",
			"Could not read if an Endpoint exists, unexpected error: "+err.Error(),
		)
		return
	}
	if !endpoint_exists {
		state.EndpointExists = types.BoolValue(false)
	}

	// We only need an Endpoint if there are Subscriptions
	// endpoint_has_subscriptions := len(state.Subscriptions) > 0
	// if endpoint_has_subscriptions {
	// 	// Recreate Endpoint if it doesnt exists anymore and reset subscriber state
	// 	// if !endpoint_exists {
	// 	// 	resp.Diagnostics.AddWarning(
	// 	// 		"Endpoint is missing",
	// 	// 		fmt.Sprintf("Endpoint %s does not exists on Azure, recreating...", state.EndpointName.ValueString()),
	// 	// 	)
	// 	// 	err := r.client.CreateEndpointWithDefaultRule(ctx, state.TopicName.ValueString(), state.EndpointName.ValueString())
	// 	// 	if err != nil {
	// 	// 		resp.Diagnostics.AddError(
	// 	// 			"Error creating Endpoint",
	// 	// 			"Could not create Endpoint, unexpected error: "+err.Error(),
	// 	// 		)
	// 	// 		return
	// 	// 	}
	// 	// 	state.Subscriptions = []string{}
	// 	// 	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	// 	// 	return
	// 	// }

	// 	if endpoint_exists {
	// 		// Assert all Subscriptions exist
	// 		actualSubscriptions, err := r.client.GetEndpointSubscriptions(
	// 			ctx,
	// 			state.TopicName.ValueString(),
	// 			state.EndpointName.ValueString(),
	// 		)

	// 		if err != nil {
	// 			resp.Diagnostics.AddError(
	// 				"Error reading Subscriptions",
	// 				"Could not get Subscriptions, unexpected error: "+err.Error(),
	// 			)
	// 			return
	// 		}
	// 		actualSubscriptionNames := make([]string, 0, len(actualSubscriptions))
	// 		for k := range actualSubscriptions {
	// 			actualSubscriptionNames = append(actualSubscriptionNames, k)
	// 		}
			
	// 		combinedSubscriptions := getUniqueElements(append(state.Subscriptions, actualSubscriptionNames...))

	// 		foundSubscriptions := []string{}

	// 		for _, subscription := range combinedSubscriptions {
	// 			existsInState := slices.Contains(state.Subscriptions, subscription)
	// 			actuallyExists := slices.Contains(actualSubscriptionNames, subscription)
	// 			if (existsInState && actuallyExists) || (!existsInState && actuallyExists) {
	// 				subscriptionFilter := actualSubscriptions[subscription].Filter
	// 				if subscriptionFilter != asb.MakeSubscriptionFilter(subscription) {
	// 					r.client.DeleteEndpointSubscription(ctx, model, subscription)
	// 					continue
	// 				}
	// 				foundSubscriptions = append(foundSubscriptions, subscription)
	// 			}
	// 		}

	// 		state.Subscriptions = foundSubscriptions
	// 	}
	// }

	// Set refreshed state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}
