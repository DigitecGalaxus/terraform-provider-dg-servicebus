package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointWithDefaultRule(
	azureContext context.Context,
	model EndpointModel,
) error {
	queueName, err := w.GetFullyQualifiedName(azureContext, model.EndpointName)
	if err != nil {
		return err
	}

	_, err = w.Client.CreateSubscription(
		azureContext,
		model.TopicName,
		model.EndpointName,
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
	model EndpointModel,
) error {
	_, err := w.Client.DeleteSubscription(
		azureContext,
		model.TopicName,
		model.EndpointName,
		nil,
	)

	return err
}

func (w *AsbClientWrapper) EndpointExists(ctx context.Context, model EndpointModel) (bool, error) {
	subscription, err := w.Client.GetSubscription(ctx, model.TopicName, model.EndpointName, nil)
	if err != nil {
		return false, err
	}
	if subscription == nil {
		return false, nil
	}
	return true, nil
}
