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
	endpointName := previousState.EndpointName.ValueString()

	if terraformPreviouslyCreatedQueue {
		if !queueExistsInAsb {
			resp.Diagnostics.AddWarning(fmt.Sprintf("The queue for endpoint %v exists in Terraform state but not in Azure Service Bus.", endpointName),
				"This could indicate that someone manually deleted it. It will be recreated on the next apply.")
			updatedState.ShouldCreateQueue = types.BoolValue(true)
			updatedState.QueueExists = types.BoolValue(false)
			return true
		}

		applyAsbQueueStateToState(updatedState, queue)
		return true
	}

	if !queueExistsInAsb {
		updatedState.QueueExists = types.BoolValue(false)
		updatedState.ShouldCreateQueue = types.BoolValue(true)
		return true // Wasn't created yet, what we expect
	}

	resp.Diagnostics.AddWarning(
		fmt.Sprintf("Queue for endpoint %v exists in Azure Service Bus but not in Terraform state", endpointName),
		"This suggests that the queue may have been created manually or that the endpoint already exists, possibly deployed in another infrastructure deployment. "+
			"If you did not intend to import this endpoint, you can remove it from the Terraform state using `terraform state rm`, or you can contact the platform for support.",
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

	tflog.Info(
		context.Background(),
		fmt.Sprintf("Queue %s has partitioning enabled: %t with max size %d", state.EndpointName, partitioningIsEnabled, maxQueueSizeInMb),
	)

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
			resp.Diagnostics.AddWarning(fmt.Sprintf("Endpoint %v exists in Terraform state but not in Azure Service Bus.", previousState.EndpointName.ValueString()),
				"This could indicate that someone manually deleted it. It will be recreated on next apply.")
			updatedState.ShouldCreateEndpoint = types.BoolValue(true)
			updatedState.EndpointExists = types.BoolValue(false)
			return true
		}

		updatedState.ShouldCreateEndpoint = types.BoolValue(false)
		updatedState.EndpointExists = types.BoolValue(true)
		return r.updateEndpointSubscriptionState(ctx, updatedState, resp)
	}

	if !endpointExists {
		updatedState.EndpointExists = types.BoolValue(false)
		updatedState.ShouldCreateEndpoint = types.BoolValue(true)
		return true // Wasn't created yet, what we expect
	}

	resp.Diagnostics.AddWarning(
		fmt.Sprintf("Endpoint %v exists in Azure Service Bus but not in Terraform state", previousState.EndpointName.ValueString()),
		"This suggests that the endpoint may have been created manually or that the endpoint already exists, possibly deployed in another infrastructure deployment. "+
			"If you did not intend to import this endpoint, you can remove it from the Terraform state using `terraform state rm`, or you can contact the platform for support.",
	)

	updatedState.EndpointExists = types.BoolValue(true)
	updatedState.ShouldCreateEndpoint = types.BoolValue(false)

	return r.updateEndpointSubscriptionState(ctx, updatedState, resp)
}

func (r *endpointResource) updateEndpointSubscriptionState(
	ctx context.Context,
	updatedState *endpointResourceModel,
	resp *resource.ReadResponse,
) bool {
	azureSubscriptions, err := r.client.GetEndpointSubscriptions(ctx, updatedState.ToAsbModel())
	if err != nil {
		return false
	}

	updatedSubscriptionState := []string{}
	for _, azureSubscription := range azureSubscriptions {
		subscriptionName := asb.TryGetFullSubscriptionNameFromRuleName(updatedState.Subscriptions, azureSubscription.Name)
		if subscriptionName == nil {
			resp.Diagnostics.AddWarning(fmt.Sprintf("Subscription %v not found in state for endpoint %v", azureSubscription.Name, updatedState.EndpointName),
				"This could indicate that someone manually added it to the state. When an item is manually added to the state, it will be deleted on the next apply.",
			)
			// Add to the state, which will delete the resource on apply
			updatedSubscriptionState = append(updatedSubscriptionState, azureSubscription.Name)
			continue
		}

		if !asb.IsSubscriptionFilterCorrect(azureSubscription.Filter, *subscriptionName) {
			resp.Diagnostics.AddWarning(fmt.Sprintf("Cannot parse rule '%v' in Subscription %v for endpoint %v", azureSubscription.Filter, azureSubscription.Name, updatedState.EndpointName),
				"This could indicate that someone manually added the rule it. It will be added to the state as is.",
			)
			updatedState.HasMalformedFilters = types.BoolValue(true)
		}

		tflog.Info(ctx, fmt.Sprintf("Subscription %s is in state as %s", azureSubscription.Name, *subscriptionName))
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
