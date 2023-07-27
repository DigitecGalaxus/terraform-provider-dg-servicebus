package endpoint

import (
	"context"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func (r *endpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan endpointResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	previousState := endpointResourceModel{}
	req.State.Get(ctx, &previousState)
	azureContext := context.Background()

	previousStateModel := previousState.ToAsbModel()
	planModel := plan.ToAsbModel()

	if plan.EndpointName.ValueString() != previousState.EndpointName.ValueString() {
		r.client.DeleteEndpointQueue(azureContext, previousStateModel)
		r.client.CreateEndpointQueue(azureContext, planModel.EndpointName, planModel.QueueOptions)

		r.client.DeleteEndpoint(azureContext, previousStateModel.TopicName, previousStateModel.EndpointName)
		r.client.CreateEndpointWithDefaultRule(azureContext, planModel.TopicName, planModel.EndpointName)
	}

	r.UpdateSubscriptions(azureContext, previousStateModel, planModel)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) UpdateSubscriptions(
	azureContext context.Context,
	previousState asb.EndpointModel,
	plan asb.EndpointModel,
) error {
	subscriptions := getUniqueElements(append(plan.Subscriptions, previousState.Subscriptions...))

	for _, subscription := range subscriptions {
		shouldBeDeleted := !arrayContains(plan.Subscriptions, subscription)
		if shouldBeDeleted {
			err := r.client.DeleteEndpointSubscription(azureContext, plan, subscription)
			if err != nil {
				return err
			}
			continue
		}

		shouldBeCreated := !arrayContains(previousState.Subscriptions, subscription)
		if shouldBeCreated {
			err := r.client.CreateEndpointSubscription(azureContext, plan, subscription)
			if err != nil {
				return err
			}
		}

		// Exists and should stay like that
	}

	return nil
}