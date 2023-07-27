package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointQueue(
	ctx context.Context,
	queueName string,
	queueOptions EndpointQueueOptions,
) error {
	_, err := w.Client.CreateQueue(
		ctx,
		queueName,
		&az.CreateQueueOptions{
			Properties: &az.QueueProperties{
				EnablePartitioning:      queueOptions.EnablePartitioning,
				MaxSizeInMegabytes:      queueOptions.MaxSizeInMegabytes,
				MaxDeliveryCount:        to.Ptr(MAX_DELIVERY_COUNT),
				LockDuration:            to.Ptr("PT5M"),
				EnableBatchedOperations: to.Ptr(true),
			},
		},
	)

	return err
}

func (w *AsbClientWrapper) DeleteEndpointQueue(
	ctx context.Context,
	model EndpointModel,
) error {
	_, err := w.Client.DeleteQueue(
		ctx,
		model.EndpointName,
		nil,
	)

	return err
}

func (w *AsbClientWrapper) GetEndpointQueue(
	azureContext context.Context,
	endpointName string,
) (*az.GetQueueResponse, error) {
	return w.Client.GetQueue(
		azureContext,
		endpointName,
		nil,
	)
}
