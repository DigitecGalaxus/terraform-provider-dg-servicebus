package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointWithDefaultRule(
	azureContext context.Context,
	model AsbEndpointModel,
) error {
	return w.CreateEndpointWithDefaultRuleWithOptinalForwading(
		azureContext,
		model,
		false,
	)
}

func (w *AsbClientWrapper) CreateEndpointWithDefaultRuleWithOptinalForwading(
	azureContext context.Context,
	model AsbEndpointModel,
	disableForward bool,
) error {
	var queueNamePtr *string
	if !disableForward {
		queueName, err := w.GetFullyQualifiedName(azureContext, model.EndpointName)
		if err != nil {
			return err
		}
		queueNamePtr = to.Ptr(queueName)
	} else {
		queueNamePtr = nil
	}

	_, err := w.Client.CreateSubscription(
		azureContext,
		model.TopicName,
		model.EndpointName,
		&az.CreateSubscriptionOptions{
			Properties: &az.SubscriptionProperties{
				ForwardTo:                        queueNamePtr,
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
	model AsbEndpointModel,
) error {
	_, err := w.Client.DeleteSubscription(
		azureContext,
		model.TopicName,
		model.EndpointName,
		nil,
	)

	return err
}

func (w *AsbClientWrapper) EndpointExists(ctx context.Context, model AsbEndpointModel) (bool, error) {
	subscription, err := w.Client.GetSubscription(ctx, model.TopicName, model.EndpointName, nil)
	if err != nil {
		return false, err
	}
	if subscription == nil {
		return false, nil
	}
	return true, nil
}
