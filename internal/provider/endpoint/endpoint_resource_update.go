package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/hashicorp/terraform-plugin-framework/resource"
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
		err := r.updateMalformedSubscriptions(ctx, plan)
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
	resp *resource.UpdateResponse,
) error {
	planModel := plan.ToAsbModel()

	// tflog.Info(ctx, fmt.Sprintf("Previous state: %s", strings.Join(previousState.Subscriptions, ", ")))
	// tflog.Info(ctx, fmt.Sprintf("Plan: %s", strings.Join(plan.Subscriptions, ", ")))

	// This is deliberately done in this order, such that if subscriptions are replaced,
	// the new subscriptions are created before the old ones are deleted, thus avoiding
	// the Endpoint missing events for a short period of time.

	// Create subscriptions that are not in the previous state
	for _, planSubscription := range plan.Subscriptions {
		tflog.Info(ctx, fmt.Sprintf("Checking subscription create %s", planSubscription))
		hasChanged := !slices.Contains(previousState.Subscriptions, planSubscription)
		if !hasChanged {
			// Exists and should stay like that
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Checking subscription exists %s", planSubscription))
		// Rule does not exist, create it
		rule, err := r.client.GetAsbSubscriptionRule(ctx, planModel, planSubscription.Filter.ValueString())
		if err != nil {
			tflog.Info(ctx, fmt.Sprintf("Creating subscription %s", planSubscription))
			err := r.client.CreateAsbSubscriptionRule(ctx, planModel, planSubscription.ToAsbModel())
			if err == nil {
				continue
			}
		}

		if rule.FilterType != planSubscription.FilterType.ValueString() {
			// Rule exists, update it
			tflog.Info(ctx, fmt.Sprintf("Updating subscription function filter type %s", planSubscription))
			r.client.UpdateAsbSubscriptionRule(ctx, planModel, planSubscription.ToAsbModel())
			return err
		}

		tflog.Error(ctx, "Subscription already exists and nothing changed. This should not happen.")
	}

	// Delete subscriptions that are not in the plan
	planSubscriptionFilterValues := []string{}
	for _, subscription := range plan.Subscriptions {
		planSubscriptionFilterValues = append(planSubscriptionFilterValues, subscription.Filter.ValueString())
	}

	for _, previousSubscription := range previousState.Subscriptions {
		tflog.Info(ctx, fmt.Sprintf("Checking subscription delete %s", previousSubscription))
		// The rule name is created from the filter value, so we can use it to check if the rule exists
		shouldBeDeleted := !slices.Contains(planSubscriptionFilterValues, previousSubscription.Filter.ValueString())
		if !shouldBeDeleted {
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Deleting subscription %s", previousSubscription))
		err := r.client.DeleteAsbSubscriptionRule(ctx, planModel, previousSubscription.ToAsbModel())
		if err != nil {
			return err
		}

		_, err = r.client.GetAsbSubscriptionRule(ctx, planModel, previousSubscription.Filter.ValueString())
		if err != nil {
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
	plan endpointResourceModel,
) error {
	azureSubscription, err := r.client.GetAsbSubscriptionsRules(ctx, plan.ToAsbModel())
	if err != nil {
		return err
	}

	plannedSubscriptions := []string{}
	for _, subscription := range plan.Subscriptions {
		plannedSubscriptions = append(plannedSubscriptions, subscription.Filter.ValueString())
	}

	for _, subscription := range azureSubscription {
		tflog.Info(ctx, fmt.Sprintf("Checking subscription update %s", subscription.Name))

		index := asb.GetSubscriptionFilterValueForAsbRuleName(plannedSubscriptions, subscription)
		if index < 0 {
			// This most likely means that the subscription will be deleted
			tflog.Info(ctx, fmt.Sprintf("Subscription %s not found in plan", subscription.Name))
			continue
		}

		plannedSubscription := plan.Subscriptions[index]

		if asb.IsAsbSubscriptionRuleCorrect(subscription, plannedSubscription.ToAsbModel()) {
			tflog.Info(ctx, fmt.Sprintf("Subscription %s is correct", subscription.Name))
			continue
		}

		tflog.Info(ctx, fmt.Sprintf("Subscription %s is incorrect. Updating.", subscription.Name))
		err := r.client.UpdateAsbSubscriptionRule(ctx, plan.ToAsbModel(), plannedSubscription.ToAsbModel())
		if err != nil {
			return err
		}
	}

	return nil
}
