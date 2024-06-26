package endpoint

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *endpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := plan.ToAsbModel()

	var err error

	// Abort if subscription exists
	endpointExists, err := r.client.EndpointExists(ctx, model)
	if err != nil {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Checking if subscription %v exists failed. Let's assume the endpoint does not exist.", model.EndpointName),
			err.Error())
		endpointExists = false
	}
	if endpointExists {
		resp.Diagnostics.AddError("Cannot create endpoint",
			fmt.Sprintf("Subscription %v already existis on topic %v for endpoint %v", model.EndpointName, model.TopicName, model.EndpointName))
		return
	}

	// Only create queue if not existing
	queueExists, err := r.client.QueueExists(ctx, model.EndpointName)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"The existing queue check failed. Let's assume the queue does not exist.",
			err.Error())
		queueExists = false
	}
	if !queueExists {
		if !r.createEndpointQueue(ctx, model, resp) {
			return
		}
	} else {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Queue %v for endpoint %v already exists.", model.EndpointName, model.EndpointName),
			"This suggests that the queue may have been created manually or that the endpoint already exists, possibly deployed in another infrastructure deployment."+
				"If you did not intend to import this endpoint, you can remove it from the Terraform state using `terraform state rm` command, or you can contact the platform for support.",
		)
	}

	// Create additional queues without takeover
	if !r.createAdditionalQueues(ctx, model, resp) {
		return
	}

	// Create subscription and rules
	err = r.client.CreateEndpointWithDefaultRule(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating subscription",
			"Could not create subscription, unexpected error: "+err.Error(),
		)
		return
	}

	for i := 0; i < len(plan.Subscriptions); i++ {
		err := r.client.CreateAsbSubscriptionRule(ctx, model, plan.Subscriptions[i], plan.SubscriptionFilterType.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating rule",
				"Could not create rule, unexpected error: "+err.Error(),
			)
			return
		}
	}

	// Update state
	plan.QueueExists = types.BoolValue(true)
	plan.EndpointExists = types.BoolValue(true)
	plan.ShouldCreateQueue = types.BoolValue(false)
	plan.ShouldCreateEndpoint = types.BoolValue(false)
	plan.HasMalformedFilters = types.BoolValue(false)
	plan.ShouldUpdateSubscriptions = types.BoolValue(false)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) createEndpointQueue(ctx context.Context, model asb.AsbEndpointModel, resp *resource.CreateResponse) bool {
	err := r.client.CreateEndpointQueue(ctx, model.EndpointName, model.QueueOptions)
	if err == nil {
		return true
	}

	var respError *azcore.ResponseError
	switch {
	case errors.As(err, &respError):
		if respError.StatusCode == http.StatusConflict {
			resp.Diagnostics.AddError(
				"Resource already exists",
				"This resource already exists and is tracked outside of Terraform. "+
					"To track this resource you have to import it into state with: "+
					"'terraform import dgservicebus_endpoint.<Block label> <topic_name>,<endpoint_name>'",
			)
			return false
		}

		resp.Diagnostics.AddError(
			"Error creating queue",
			"Could not create queue, unexpected error: "+err.Error(),
		)
	default:
		resp.Diagnostics.AddError(
			"Error creating queue",
			"Could not create queue, unexpected error: "+err.Error(),
		)

		return false
	}

	return false
}

func (r *endpointResource) createAdditionalQueues(ctx context.Context, model asb.AsbEndpointModel, resp *resource.CreateResponse) bool {
	for _, queue := range model.AdditionalQueues {
		queueExists, err := r.client.QueueExists(ctx, queue)
		if err != nil {
			resp.Diagnostics.AddWarning(
				"The existing queue check failed. Let's assume the queue does not exist.",
				err.Error())
			queueExists = false
		}

		if queueExists {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Queue %v for endpoint %v already exists.", queue, model.EndpointName),
				"This suggests that the queue may have been created manually or that the endpoint already exists, possibly deployed in another infrastructure deployment."+
					"If you did not intend to import this endpoint, you can remove it from the Terraform state using `terraform state rm` command, or you can contact the platform for support.",
			)
			continue
		}

		err = r.client.CreateEndpointQueue(ctx, queue, model.QueueOptions)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating additional queue",
				fmt.Sprintf("Could not create queue %s, unexpected error: %q", queue, err.Error()),
			)
			return false
		}

	}
	return true
}
