package endpoint

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func (*endpointResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := NewSchemaV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: udpateStateFromV0ToV1,
		},
	}
}

func udpateStateFromV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	// Convert the state from the prior schema version to the current schema version.
	// This example demonstrates how to convert the state from the prior schema version to the current schema version.
	// The conversion is specific to the schema changes between the prior and current versions

	var priorState endpointResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &priorState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptions := make([]SubscriptionModel, 0)
	for _, subscription := range priorState.Subscriptions {
		subscriptions = append(subscriptions, SubscriptionModel{
			Filter:     basetypes.NewStringValue(subscription),
			FilterType: basetypes.NewStringValue("sql"),
		})
	}

	currrentState := endpointResourceModel{
		EndpointName:              priorState.EndpointName,
		TopicName:                 priorState.TopicName,
		QueueOptions:              priorState.QueueOptions,
		Subscriptions:             subscriptions,
		AdditionalQueues:          priorState.AdditionalQueues,
		QueueExists:               priorState.QueueExists,
		EndpointExists:            priorState.EndpointExists,
		HasMalformedFilters:       priorState.HasMalformedFilters,
		ShouldCreateQueue:         priorState.ShouldCreateQueue,
		ShouldCreateEndpoint:      priorState.ShouldCreateEndpoint,
		ShouldUpdateSubscriptions: priorState.ShouldUpdateSubscriptions,
	}

	diags := resp.State.Set(ctx, currrentState)
	resp.Diagnostics.Append(diags...)
}
