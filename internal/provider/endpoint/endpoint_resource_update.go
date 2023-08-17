package endpoint

import (
	"context"
	"strings"
	"terraform-provider-dg-servicebus/internal/provider/asb"
	"time"

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

	time.Sleep(5 * time.Second)

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

	err := r.UpdateSubscriptions(ctx, previousState, plan)
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
	subscriptions := getUniqueElements(append(plan.Subscriptions, previousState.Subscriptions...))

	for _, subscription := range subscriptions {
		shouldBeDeleted := !slices.Contains(plan.Subscriptions, subscription)
		if shouldBeDeleted {
			err := r.client.DeleteEndpointSubscription(ctx, plan.ToAsbModel(), subscription)
			if err != nil {
				return err
			}
			continue
		}

		shouldBeCreated := !slices.Contains(previousState.Subscriptions, subscription)
		if shouldBeCreated {
			err := r.client.CreateEndpointSubscription(ctx, plan.ToAsbModel(), subscription)
			if err != nil {
				return err
			}
		}

		// Exists and should stay like that
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

	getFullSubscriptionNameBySuffixInState := func(subscriptionSuffix string) *string {
		for _, subscription := range state.Subscriptions {
			if strings.HasSuffix(subscription, subscriptionSuffix) {
				return &subscription
			}
		}

		return nil
	}

	for _, subscription := range azureSubscription {
		subscriptionName := getFullSubscriptionNameBySuffixInState(subscription.Name)
		if subscriptionName == nil {
			err := r.client.DeleteEndpointSubscription(ctx, plan.ToAsbModel(), subscription.Name)
			if err != nil {
				return err
			}
			continue
		}
		if asb.IsFilterCorrect(subscription.Filter, *subscriptionName) {
			continue
		}

		err := r.client.EnsureEndpointSubscriptionFilterCorrect(ctx, plan.ToAsbModel(), *subscriptionName)
		if err != nil {
			return err
		}
	}

	return nil
}
