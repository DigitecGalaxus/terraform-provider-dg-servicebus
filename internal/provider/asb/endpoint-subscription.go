package asb

import (
	"context"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointSubscription(
	azureContext context.Context,
	plan EndpointModel,
	subscriptionName string,
) error {
	_, err := w.Client.CreateRule(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		&az.CreateRuleOptions{
			Name: &subscriptionName,
			Filter: &az.SQLFilter{
				Expression: "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionName + "%'",
			},
		},
	)

	return err
}

func (w *AsbClientWrapper) EndpointSubscriptionExists(
	azureContext context.Context,
	plan EndpointModel,
	subscriptionName string,
) bool {
	_, err := w.Client.GetRule(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		subscriptionName,
		nil,
	)

	return err == nil
}

func (w *AsbClientWrapper) DeleteEndpointSubscription(
	azureContext context.Context,
	plan EndpointModel,
	subscriptionName string,
) error {
	_, err := w.Client.DeleteRule(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		subscriptionName,
		nil,
	)

	return err
}
