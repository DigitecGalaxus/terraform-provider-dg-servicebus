package endpoint

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type shouldCreateEndpointIfSubscriberAddedModifier struct{}

func (m shouldCreateEndpointIfSubscriberAddedModifier) Description(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfSubscriberAddedModifier) MarkdownDescription(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfSubscriberAddedModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var state endpointResourceModel
	req.State.Get(ctx, &state)

	var plan endpointResourceModel
	req.Plan.Get(ctx, &plan)

	previousSubscriberLen := len(state.Subscriptions)
	afterSubscriberLen := len(plan.Subscriptions)

	if previousSubscriberLen == 0 && afterSubscriberLen > 0 {
		resp.PlanValue = types.BoolValue(true)
	}
}

type shouldCreateQueueIfNotExistsModifier struct{}

func (m shouldCreateQueueIfNotExistsModifier) Description(_ context.Context) string {
	return "Checks in plan if a Queue should be created."
}

func (m shouldCreateQueueIfNotExistsModifier) MarkdownDescription(_ context.Context) string {
	return "Checks in plan if a Queue should be created."
}

func (m shouldCreateQueueIfNotExistsModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var state endpointResourceModel
	req.State.Get(ctx, &state)

	queueExists := state.QueueExists.ValueBool()
	if !queueExists {
		resp.PlanValue = types.BoolValue(true)
	}
}

type shouldCreateEndpointIfNotExistsModifier struct{}

func (m shouldCreateEndpointIfNotExistsModifier) Description(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfNotExistsModifier) MarkdownDescription(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfNotExistsModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var state endpointResourceModel
	req.State.Get(ctx, &state)

	endpointExists := state.EndpointExists.ValueBool()
	if !endpointExists {
		resp.PlanValue = types.BoolValue(true)
	}
}