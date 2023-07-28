package endpoint

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"strings"
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
		if strings.Contains(err.Error(), "ERROR CODE: 409") {
			resp.Diagnostics.AddError(
				"Resource already exists",
				"This resource already exists and is tracked outside of Terraform. "+
					"To track this resource you have to import it into state with: "+
					"'terraform import dgservicebus_endpoint.<Block label> <topic_name>,<endpoint_name>'",
			)
			return
		}

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

	err = r.client.CreateEndpointWithDefaultRule(ctx, model)
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
	plan.ShouldCreateQueue = types.BoolValue(false)
	plan.ShouldCreateEndpoint = types.BoolValue(false)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
