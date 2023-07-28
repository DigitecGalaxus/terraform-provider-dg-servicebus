package endpoint

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"terraform-provider-dg-servicebus/internal/provider/asb"
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
		r.client.CreateEndpointQueue(ctx, planModel.EndpointName, planModel.QueueOptions)
	}

	if plan.ShouldCreateEndpoint.ValueBool() {
		r.client.CreateEndpointWithDefaultRule(ctx, planModel)
	}

	if plan.EndpointName.ValueString() != previousState.EndpointName.ValueString() {
		r.client.DeleteEndpointQueue(ctx, previousStateModel)
		r.client.CreateEndpointQueue(ctx, planModel.EndpointName, planModel.QueueOptions)

		r.client.DeleteEndpoint(ctx, previousStateModel)
		r.client.CreateEndpointWithDefaultRule(ctx, planModel)
	}

	r.UpdateSubscriptions(ctx, previousStateModel, planModel)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) UpdateSubscriptions(
	ctx context.Context,
	previousState asb.EndpointModel,
	plan asb.EndpointModel,
) error {
	subscriptions := getUniqueElements(append(plan.Subscriptions, previousState.Subscriptions...))

	for _, subscription := range subscriptions {
		shouldBeDeleted := !arrayContains(plan.Subscriptions, subscription)
		if shouldBeDeleted {
			err := r.client.DeleteEndpointSubscription(ctx, plan, subscription)
			if err != nil {
				return err
			}
			continue
		}

		shouldBeCreated := !arrayContains(previousState.Subscriptions, subscription)
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
