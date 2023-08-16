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
	previousStateModel := previousState.ToAsbModel()

	if plan.ShouldCreateQueue.ValueBool() {
		err := r.client.CreateEndpointQueue(ctx, planModel.EndpointName, planModel.QueueOptions)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating queue",
				"Queue creation failed with error: " + err.Error(),
			)
			return
		}
	}

	if plan.ShouldCreateEndpoint.ValueBool() {
		err := r.client.CreateEndpointWithDefaultRule(ctx, planModel)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating endpoint",
				"Endpoint creation failed with error: " + err.Error(),
			)
			return
		}
	}

	err := r.UpdateSubscriptions(ctx, previousStateModel, planModel)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating subscriptions",
			"Subscription update failed with error: " + err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *endpointResource) UpdateSubscriptions(
	ctx context.Context,
	previousState asb.EndpointModel,
	plan asb.EndpointModel,
) error {
	subscriptions := getUniqueElements(append(plan.Subscriptions, previousState.Subscriptions...))

	for _, subscription := range subscriptions {
		shouldBeDeleted := !slices.Contains(plan.Subscriptions, subscription)
		if shouldBeDeleted {
			err := r.client.DeleteEndpointSubscription(ctx, plan, subscription)
			if err != nil {
				return err
			}
			continue
		}

		shouldBeCreated := !slices.Contains(previousState.Subscriptions, subscription)
		if shouldBeCreated {
			err := r.client.CreateEndpointSubscription(ctx, plan, subscription)
			if err != nil {
				return err
			}
		}

		// Exists and should stay like that
	}

	return nil
}
