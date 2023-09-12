package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/exp/slices"
)

func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	previousState := state

	state.ShouldUpdateSubscriptions = types.BoolValue(false)
	state.HasMalformedFilters = types.BoolValue(false)

	if !r.syncQueueState(ctx, &previousState, &state, resp) {
		return
	}

	if !r.syncSubscriptionState(ctx, &previousState, &state, resp) {
		return
	}

	if !r.updateAdditionalQueueState(ctx, &state, resp) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) syncQueueState(
	ctx context.Context,
	previousState *endpointResourceModel,
	updatedState *endpointResourceModel,
	resp *resource.ReadResponse,
) bool {
	queue, err := r.client.GetEndpointQueue(ctx, previousState.ToAsbModel())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Queue",
			"Could not get Queue, unexpected error: "+err.Error(),
		)
		return false
	}

	queueExistsInAsb := queue != nil
	terraformPreviouslyCreatedQueue := previousState.QueueExists.ValueBool()

	if terraformPreviouslyCreatedQueue {
		if !queueExistsInAsb {
			tflog.Info(ctx, "Queue exists in Terraform state but not in Azure Service Bus. It will be recreated on next apply.")
			updatedState.ShouldCreateQueue = types.BoolValue(true)
			updatedState.QueueExists = types.BoolValue(false)
			return true
		}

		applyAsbQueueStateToState(updatedState, queue)
		return true
	}

	if !queueExistsInAsb {
		return true // Wasn't created yet, what we expect
	}

	tflog.Info(
		ctx,
		"Queue exists in Azure Service Bus but not in Terraform state. Importing queue into Terraform state.",
	)

	applyAsbQueueStateToState(updatedState, queue)
	return true
}

func applyAsbQueueStateToState(
	state *endpointResourceModel,
	queue *admin.GetQueueResponse,
) {
	state.QueueExists = types.BoolValue(true)
	state.ShouldCreateQueue = types.BoolValue(false)

	maxQueueSizeInMb := *queue.QueueProperties.MaxSizeInMegabytes
	partitioningIsEnabled := *queue.QueueProperties.EnablePartitioning
	if partitioningIsEnabled {
		maxQueueSizeInMb = maxQueueSizeInMb / 16
	}

	state.QueueOptions.MaxSizeInMegabytes = types.Int64Value(int64(maxQueueSizeInMb))
	state.QueueOptions.EnablePartitioning = types.BoolValue(partitioningIsEnabled)
}

func (r *endpointResource) syncSubscriptionState(
	ctx context.Context,
	previousState *endpointResourceModel,
	updatedState *endpointResourceModel,
	resp *resource.ReadResponse,
) bool {
	endpointExists, err := r.client.EndpointExists(ctx, previousState.ToAsbModel())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Endpoint",
			"Could not read if an Endpoint exists, unexpected error: "+err.Error(),
		)

		return false
	}

	terraformPreviouslyCreatedEndpoint := previousState.EndpointExists.ValueBool()

	if terraformPreviouslyCreatedEndpoint {
		if !endpointExists {
			tflog.Info(ctx, "Endpoint exists in Terraform state but not in Azure Service Bus. It will be recreated on next apply.")
			updatedState.ShouldCreateEndpoint = types.BoolValue(true)
			updatedState.EndpointExists = types.BoolValue(false)
			return true
		}

		updatedState.ShouldCreateEndpoint = types.BoolValue(false)
		updatedState.EndpointExists = types.BoolValue(true)
		return r.updateEndpointSubscriptionState(ctx, updatedState)
	}

	if !endpointExists {
		updatedState.EndpointExists = types.BoolValue(false)
		updatedState.ShouldCreateEndpoint = types.BoolValue(true)
		return true // Wasn't created yet, what we expect
	}

	tflog.Info(
		ctx,
		"Endpoint exists in Azure Service Bus but not in Terraform state. Importing endpoint into Terraform state.",
	)

	updatedState.EndpointExists = types.BoolValue(true)
	updatedState.ShouldCreateEndpoint = types.BoolValue(false)

	return r.updateEndpointSubscriptionState(ctx, updatedState)
}

func (r *endpointResource) updateEndpointSubscriptionState(
	ctx context.Context,
	updatedState *endpointResourceModel,
) bool {
	azureSubscriptions, err := r.client.GetEndpointSubscriptions(ctx, updatedState.ToAsbModel())
	if err != nil {
		return false
	}

	updatedSubscriptionState := []string{}
	for _, azureSubscription := range azureSubscriptions {
		subscriptionName := asb.TryGetFullSubscriptionNameFromRuleName(updatedState.Subscriptions, azureSubscription.Name)
		if subscriptionName == nil {
			tflog.Warn(ctx, fmt.Sprintf("Subscription %s not found in state", azureSubscription.Name))
			// Add to the state, which will delete the resource on apply
			updatedSubscriptionState = append(updatedSubscriptionState, azureSubscription.Name)
			continue
		}

		if !asb.IsSubscriptionFilterCorrect(azureSubscription.Filter, *subscriptionName) {
			tflog.Warn(ctx, fmt.Sprintf("Subscription %s has bad filter", azureSubscription.Name))
			updatedState.HasMalformedFilters = types.BoolValue(true)
		}

		tflog.Warn(ctx, fmt.Sprintf("Subscription %s is in state as %s", azureSubscription.Name, *subscriptionName))
		updatedSubscriptionState = append(updatedSubscriptionState, *subscriptionName)
	}

	updatedState.Subscriptions = updatedSubscriptionState
	return true
}

func (r *endpointResource) updateAdditionalQueueState(ctx context.Context, state *endpointResourceModel, resp *resource.ReadResponse) bool {
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
