package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointWithDefaultRule(
	azureContext context.Context,
	plan EndpointModel,
) error {
	queueName, err := w.GetFullyQualifiedName(azureContext, plan.EndpointName)
	if err != nil {
		return err
	}

	_, err = w.Client.CreateSubscription(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		&az.CreateSubscriptionOptions{
			Properties: &az.SubscriptionProperties{
				ForwardTo:                        to.Ptr(queueName),
				MaxDeliveryCount:                 to.Ptr(MAX_DELIVERY_COUNT),
				EnableBatchedOperations:          to.Ptr(true),
				LockDuration:                     to.Ptr("PT5M"),
				DeadLetteringOnMessageExpiration: to.Ptr(false),
				EnableDeadLetteringOnFilterEvaluationExceptions: to.Ptr(false),
				RequiresSession: to.Ptr(false),
				DefaultRule: &az.RuleProperties{
					Filter: &az.FalseFilter{},
				},
			},
		})

	return err
}

func (w *AsbClientWrapper) DeleteEndpoint(
	azureContext context.Context,
	plan EndpointModel,
) error {
	_, err := w.Client.DeleteSubscription(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		nil,
	)

	return err
}
