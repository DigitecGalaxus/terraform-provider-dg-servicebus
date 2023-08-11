package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/exp/slices"
)

func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	model := state.ToAsbModel()

	state.QueueExists = types.BoolValue(true)
	state.EndpointExists = types.BoolValue(true)
	state.ShouldCreateQueue = types.BoolValue(false)
	state.ShouldCreateEndpoint = types.BoolValue(false)

	var success bool

	success = r.checkEndpointQueue(ctx, model, &state, resp)
	if !success {
		return
	}

	hasSubscribers := len(model.Subscriptions) > 0

	if hasSubscribers {
		success = r.checkEndpoint(ctx, model, &state, resp)
		if !success {
			return
		}

		// There are no subscriptions to check if the endpoint does not exist
		if state.EndpointExists.ValueBool() {
			success = r.checkSubscriptions(ctx, model, &state, resp)
			if !success {
				return
			}
		}
	}

	success = r.checkAdditionalQueues(ctx, &state, resp)
	if !success {
		return
	}

	// Set refreshed state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) checkEndpointQueue(ctx context.Context, model asb.EndpointModel, state *endpointResourceModel, resp *resource.ReadResponse) bool {
	queue, err := r.client.GetEndpointQueue(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Queue",
			"Could not get Queue, unexpected error: "+err.Error(),
		)
		return false
	}

	if queue == nil {
		state.QueueExists = types.BoolValue(false)
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Endpoint Queue %s is missing", state.EndpointName.ValueString()),
			fmt.Sprintf("Endpoint Queue %s does not exists on Azure, whole resource will be recreated at apply", state.EndpointName.ValueString()),
		)
	} else {
		state.QueueOptions.MaxSizeInMegabytes = types.Int64Value(int64(*queue.QueueProperties.MaxSizeInMegabytes))
		state.QueueOptions.EnablePartitioning = types.BoolValue(*queue.QueueProperties.EnablePartitioning)
	}
	return true
}

func (r *endpointResource) checkEndpoint(ctx context.Context, model asb.EndpointModel, state *endpointResourceModel, resp *resource.ReadResponse) bool {
	endpointExists, err := r.client.EndpointExists(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Endpoint",
			"Could not read if an Endpoint exists, unexpected error: "+err.Error(),
		)
		return false
	}
	if !endpointExists {
		state.EndpointExists = types.BoolValue(false)
		state.Subscriptions = []string{}
	}
	return true
}

func (r *endpointResource) checkSubscriptions(ctx context.Context, loadedState asb.EndpointModel, state *endpointResourceModel, resp *resource.ReadResponse) bool {
	azureSubscriptions, err := r.client.GetEndpointSubscriptions(ctx, loadedState)
	if err != nil {
		return false
	}

	subscriptionsInAzureAndState := []string{}
	azureSubscriptionNames := make([]string, 0, len(azureSubscriptions))
	for k := range azureSubscriptions {
		azureSubscriptionNames = append(azureSubscriptionNames, k)
	}
	combinedSubscriptions := getUniqueElements(append(loadedState.Subscriptions, azureSubscriptionNames...))

	for _, subscription := range combinedSubscriptions {
		existsInState := slices.Contains(loadedState.Subscriptions, subscription)
		existsInAzure := slices.Contains(azureSubscriptionNames, subscription)
		if (!existsInState && !existsInAzure) || (existsInState && !existsInAzure) {
			continue
		}

		subscriptionFilter := azureSubscriptions[subscription].Filter
		if subscriptionFilter == asb.MakeSubscriptionFilter(subscription) {
			subscriptionsInAzureAndState = append(subscriptionsInAzureAndState, subscription)
			continue
		}

		// We delete the subscription if the filter is invalid, such that it can be recreated at next update
		err := r.client.DeleteEndpointSubscription(ctx, loadedState, subscription)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading subscriptions",
				"Unexpected error occured while reading subscriptions, error: "+err.Error(),
			)
			return false
		}
	}

	state.Subscriptions = subscriptionsInAzureAndState
	return true
}

func (r *endpointResource) checkAdditionalQueues(ctx context.Context, state *endpointResourceModel, resp *resource.ReadResponse) bool {
	for _, queue := range state.AdditionalQueues {
		queueExists, err := r.client.QueueExists(ctx, queue)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading queue",
				fmt.Sprintf("Could not read if additional queue %s exists, unexpected error: %q", queue, err.Error()),
			)
			return false
		}
		if !queueExists {
			index := slices.Index(state.AdditionalQueues, queue)
			slices.Delete(state.AdditionalQueues, index, index+1)
		}
	}
	return true
}
