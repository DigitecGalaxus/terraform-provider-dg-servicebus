package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointQueue(
	azureContext context.Context,
	model EndpointModel,
) error {
	_, err := w.Client.CreateQueue(
		azureContext,
		model.EndpointName,
		&az.CreateQueueOptions{
			Properties: &az.QueueProperties{
				EnablePartitioning:      model.QueueOptions.EnablePartitioning,
				MaxSizeInMegabytes:      model.QueueOptions.MaxSizeInMegabytes,
				MaxDeliveryCount:        to.Ptr(MAX_DELIVERY_COUNT),
				LockDuration:            to.Ptr("PT5M"),
				EnableBatchedOperations: to.Ptr(true),
			},
		},
	)

	return err
}

func (w *AsbClientWrapper) DeleteEndpointQueue(
	azureContext context.Context,
	model EndpointModel,
) error {
	_, err := w.Client.DeleteQueue(
		azureContext,
		model.EndpointName,
		nil,
	)

	return err
}

func (w *AsbClientWrapper) GetEndpointQueue(
	azureContext context.Context,
	model EndpointModel,
) (*az.GetQueueResponse, error) {
	return w.Client.GetQueue(
		azureContext,
		model.EndpointName,
		nil,
	)
}
