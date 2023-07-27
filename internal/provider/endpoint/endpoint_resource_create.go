package endpoint

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *endpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := plan.ToAsbModel()

	err := r.client.CreateEndpointQueue(ctx, model.EndpointName, model.QueueOptions)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating queue",
			"Could not create queue, unexpected error: "+err.Error(),
		)
		return
	}

	for _, queue := range model.AdditionalQueues {
		err := r.client.CreateEndpointQueue(ctx, queue, model.QueueOptions)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating additional queue",
				fmt.Sprintf("Could not create queue %s, unexpected error: %q", queue, err.Error()),
			)
			return
		}
	}

	err = r.client.CreateEndpointWithDefaultRule(ctx, model.TopicName, model.EndpointName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating subscription",
			"Could not create subscription, unexpected error: "+err.Error(),
		)
		return
	}

	for i := 0; i < len(plan.Subscriptions); i++ {
		err := r.client.CreateEndpointSubscription(ctx, model, plan.Subscriptions[i])
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating rule",
				"Could not create rule, unexpected error: "+err.Error(),
			)
			return
		}
	}

	plan.QueueExists = types.BoolValue(true)
	plan.EndpointExists = types.BoolValue(true)
	plan.ShouldCreateEndpoint = types.BoolValue(false)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}