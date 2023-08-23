package endpoint

import (
	"context"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/hashicorp/terraform-plugin-framework/resource"
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

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *endpointResource) UpdateSubscriptions(
	ctx context.Context,
	previousState endpointResourceModel,
	plan endpointResourceModel,
) error {
	planModel := plan.ToAsbModel()

	// This is deliberately done in this order, such that if subscriptions are replaced,
	// the new subscriptions are created before the old ones are deleted, thus avoiding
	// the Endpoint missing events for a short period of time.
	for _, planSubscription := range plan.Subscriptions {
		shouldBeCreated := !slices.Contains(previousState.Subscriptions, planSubscription)
		if !shouldBeCreated {
			// Exists and should stay like that
			continue
		}

		err := r.client.CreateEndpointSubscription(ctx, planModel, planSubscription)
		if err != nil {
			return err
		}
	}

	for _, previousSubscriptions := range previousState.Subscriptions {
		shouldBeDeleted := !slices.Contains(plan.Subscriptions, previousSubscriptions)
		if shouldBeDeleted {
			err := r.client.DeleteEndpointSubscription(ctx, planModel, previousSubscriptions)
			if err != nil {
				return err
			}

			continue
		}
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
		subscriptionName := asb.TryGetFullSubscriptionNameFromRuleName(state.Subscriptions, subscription.Name)
		if subscriptionName == nil {
			// Subscription is not managed by Terraform - delete it
			err := r.client.DeleteEndpointSubscription(ctx, plan.ToAsbModel(), subscription.Name)
			if err != nil {
				return err
			}

			continue
		}

		if asb.IsSubscriptionFilterCorrect(subscription.Filter, *subscriptionName) {
			continue
		}

		err := r.client.EnsureEndpointSubscriptionFilterCorrect(ctx, plan.ToAsbModel(), *subscriptionName)
		if err != nil {
			return err
		}
	}

	return nil
}
