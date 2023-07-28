package endpoint

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/exp/slices"
	"terraform-provider-dg-servicebus/internal/provider/asb"
)

func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	model := state.ToAsbModel()

	// time.Sleep(10 * time.Second)

	state.QueueExists = types.BoolValue(true)
	state.EndpointExists = types.BoolValue(true)
	state.ShouldCreateQueue = types.BoolValue(false)
	state.ShouldCreateEndpoint = types.BoolValue(false)

	queue, err := r.client.GetEndpointQueue(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Queue",
			"Could not get Queue, unexpected error: "+err.Error(),
		)
		return
	}

	if queue == nil {
		state.QueueExists = types.BoolValue(false)
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Endpoint Queue %s is missing", state.EndpointName.ValueString()),
			fmt.Sprintf("Endpoint Queue %s does not exists on Azure, whole resource will be recreated at apply", state.EndpointName.ValueString()),
		)
	} else {
		state.QueueOptions.MaxSizeInMegabytes = types.Int64Value(int64(*queue.MaxSizeInMegabytes))
		state.QueueOptions.EnablePartitioning = types.BoolValue(*queue.EnablePartitioning)
	}

	hasSubscribers := len(model.Subscriptions) > 0

	if hasSubscribers {
		endpointExists, err := r.client.EndpointExists(ctx, model)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading Endpoint",
				"Could not read if an Endpoint exists, unexpected error: "+err.Error(),
			)
			return
		}
		if !endpointExists {
			state.EndpointExists = types.BoolValue(false)
			state.Subscriptions = []string{}
		}

		// There are no subscriptions to check if the endpoint does not exist
		if state.EndpointExists.ValueBool() {
			foundSubscriptions, err := r.CheckSubscriptions(ctx, model)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error reading subscriptions",
					"Unexpected error occured while reading subscriptions, error: "+err.Error(),
				)
				return
			}
			state.Subscriptions = foundSubscriptions
		}
	}

	// Set refreshed state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) CheckSubscriptions(ctx context.Context, model asb.EndpointModel) ([]string, error) {
	actualSubscriptions, err := r.client.GetEndpointSubscriptions(ctx, model)

	if err != nil {
		return []string{}, err
	}

	foundSubscriptions := []string{}
	actualSubscriptionNames := make([]string, 0, len(actualSubscriptions))
	for k := range actualSubscriptions {
		actualSubscriptionNames = append(actualSubscriptionNames, k)
	}
	combinedSubscriptions := getUniqueElements(append(model.Subscriptions, actualSubscriptionNames...))

	for _, subscription := range combinedSubscriptions {
		existsInState := slices.Contains(model.Subscriptions, subscription)
		actuallyExists := slices.Contains(actualSubscriptionNames, subscription)
		if (existsInState && actuallyExists) || (!existsInState && actuallyExists) {
			subscriptionFilter := actualSubscriptions[subscription].Filter
			if subscriptionFilter != asb.MakeSubscriptionFilter(subscription) {
				err := r.client.DeleteEndpointSubscription(ctx, model, subscription)
				if err != nil {
					return []string{}, err
				}
				continue
			}
			foundSubscriptions = append(foundSubscriptions, subscription)
		}
	}

	return foundSubscriptions, nil
}
