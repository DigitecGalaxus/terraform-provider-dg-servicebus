package endpoint

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/exp/slices"
)

func (r *endpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan endpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planModel := plan.ToAsbModel()

	previousState := endpointResourceModel{}
	req.State.Get(ctx, &previousState)

	if plan.ShouldCreateQueue.ValueBool() {
		err := r.client.CreateEndpointQueue(ctx, planModel.EndpointName, planModel.QueueOptions)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating queue",
				"Queue creation failed with error: "+err.Error(),
			)
			return
		}
	}

	if plan.ShouldCreateEndpoint.ValueBool() {
		err := r.client.CreateEndpointWithDefaultRule(ctx, planModel)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating endpoint",
				"Endpoint creation failed with error: "+err.Error(),
			)
			return
		}
	}

	err := r.UpdateSubscriptions(
		ctx,
		previousState,
		plan,
		resp,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating subscriptions",
			"Subscription update failed with error: "+err.Error(),
		)
		return
	}

	if plan.ShouldUpdateSubscriptions.ValueBool() {
		err := r.updateMalformedSubscriptions(ctx, previousState, plan)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating subscriptions",
				"Subscription update failed with error: "+err.Error(),
			)
			return
		}
	}

	plan.QueueExists = types.BoolValue(true)
	plan.EndpointExists = types.BoolValue(true)
	plan.ShouldCreateQueue = types.BoolValue(false)
	plan.ShouldCreateEndpoint = types.BoolValue(false)
	plan.HasMalformedFilters = types.BoolValue(false)
	plan.ShouldUpdateSubscriptions = types.BoolValue(false)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *endpointResource) UpdateSubscriptions(
	ctx context.Context,
	previousState endpointResourceModel,
	plan endpointResourceModel,
	resp *resource.UpdateResponse,
) error {
	planModel := plan.ToAsbModel()

	tflog.Info(ctx, fmt.Sprintf("Previous state: %s", strings.Join(previousState.Subscriptions, ", ")))
	tflog.Info(ctx, fmt.Sprintf("Plan: %s", strings.Join(plan.Subscriptions, ", ")))

	// This is deliberately done in this order, such that if subscriptions are replaced,
	// the new subscriptions are created before the old ones are deleted, thus avoiding
	// the Endpoint missing events for a short period of time.
	for _, planSubscription := range plan.Subscriptions {
		tflog.Info(ctx, fmt.Sprintf("Checking subscription create %s", planSubscription))
		shouldBeCreated := !slices.Contains(previousState.Subscriptions, planSubscription)
		if !shouldBeCreated {
			// Exists and should stay like that
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Checking subscription exists %s", planSubscription))
		subscriptionExists := r.client.EndpointSubscriptionExists(ctx, planModel, planSubscription)
		if subscriptionExists {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Subscription %v rule already exists", planSubscription),
				"This suggests that the subscription rule may have been created manually. We just add it to the state.",
			)
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Creating subscription %s", planSubscription))
		err := r.client.CreateEndpointSubscription(ctx, planModel, planSubscription)
		if err == nil {
			continue
		}

		return err
	}

	for _, previousSubscription := range previousState.Subscriptions {
		tflog.Info(ctx, fmt.Sprintf("Checking subscription delete %s", previousSubscription))
		shouldBeDeleted := !slices.Contains(plan.Subscriptions, previousSubscription)
		if !shouldBeDeleted {
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Deleting subscription %s", previousSubscription))
		err := r.client.DeleteEndpointSubscription(ctx, planModel, previousSubscription)
		if err == nil {
			return nil
		}

		subscriptionExists := r.client.EndpointSubscriptionExists(ctx, planModel, previousSubscription)
		if !subscriptionExists {
			// Deals with an edge case where the subscription was not correctly identified
			// by the read operation, due to a failed update before this one.
			tflog.Info(ctx, fmt.Sprintf(
				"Subscription %s was already deleted by a previous update operation that was not persisted in state",
				previousSubscription,
			))
			continue
		}

		return err
	}

	return nil
}

func (r *endpointResource) updateMalformedSubscriptions(
	ctx context.Context,
	state endpointResourceModel,
	plan endpointResourceModel,
) error {
	azureSubscription, err := r.client.GetEndpointSubscriptions(ctx, plan.ToAsbModel())
	if err != nil {
		return err
	}

	for _, subscription := range azureSubscription {
		tflog.Info(ctx, fmt.Sprintf("Checking subscription update %s", subscription.Name))
		subscriptionName := asb.TryGetFullSubscriptionNameFromRuleName(state.Subscriptions, subscription.Name)
		if subscriptionName == nil {
			tflog.Info(ctx, fmt.Sprintf("Subscription %s is not managed by Terraform", subscription.Name))
			// Subscription is not (yet) managed by Terraform
			// This likely happens when the subscription was created in this update run,
			// and the previous state does not contain it yet.
			continue
		}

		if asb.IsSubscriptionFilterCorrect(subscription.Filter, *subscriptionName) {
			tflog.Info(ctx, fmt.Sprintf("Subscription %s is correct", subscription.Name))
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Subscription %s is incorrect", subscription.Name))
		err := r.client.EnsureEndpointSubscriptionFilterCorrect(ctx, plan.ToAsbModel(), *subscriptionName)
		if err != nil {
			return err
		}
	}

	return nil
}
