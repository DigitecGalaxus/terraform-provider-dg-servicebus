package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointWithDefaultRule(
	azureContext context.Context,
	topicName, endpointName string,
) error {
	queueName, err := w.GetFullyQualifiedName(azureContext, endpointName)
	if err != nil {
		return err
	}

	_, err = w.Client.CreateSubscription(
		azureContext,
		topicName,
		endpointName,
		&az.CreateSubscriptionOptions{
			Properties: &az.SubscriptionProperties{
				ForwardTo:                                       to.Ptr(queueName),
				MaxDeliveryCount:                                to.Ptr(MAX_DELIVERY_COUNT),
				EnableBatchedOperations:                         to.Ptr(true),
				LockDuration:                                    to.Ptr("PT5M"),
				DeadLetteringOnMessageExpiration:                to.Ptr(false),
				EnableDeadLetteringOnFilterEvaluationExceptions: to.Ptr(false),
				RequiresSession:                                 to.Ptr(false),
				DefaultRule:                                     &az.RuleProperties {
					Filter: &az.FalseFilter{},
				},
			},
		})

	return err
}

func (w *AsbClientWrapper) DeleteEndpoint(
	azureContext context.Context,
	topicName, endpointName string,
) error {
	_, err := w.Client.DeleteSubscription(
		azureContext,
		topicName,
		endpointName,
		nil,
	)

	return err
}

func (w *AsbClientWrapper) EndpointExists(ctx context.Context, topicName string, endpointName string) (bool, error) {
	subscription, err := w.Client.GetSubscription(ctx, topicName, endpointName, nil)
	if err != nil {
		return false, err
	}
	if subscription == nil {
		return false, nil
	}
	return true, nil
}
