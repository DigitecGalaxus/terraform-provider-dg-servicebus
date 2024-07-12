package asb

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

func (w *AsbClientWrapper) CreateEndpointQueue(
	ctx context.Context,
	queueName string,
	queueOptions AsbEndpointQueueOptions,
) error {
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Creating queue"+queueName,
		func() error {
			_, err := w.Client.CreateQueue(
				ctx,
				queueName,
				&az.CreateQueueOptions{
					Properties: &az.QueueProperties{
						EnablePartitioning:        queueOptions.EnablePartitioning,
						MaxSizeInMegabytes:        queueOptions.MaxSizeInMegabytes,
						MaxMessageSizeInKilobytes: queueOptions.MaxMessageSizeInKilobytes,
						MaxDeliveryCount:          to.Ptr(MAX_DELIVERY_COUNT),
						LockDuration:              to.Ptr("PT5M"),
						EnableBatchedOperations:   to.Ptr(true),
					},
				},
			)

			return err
		},
	)
}

func (w *AsbClientWrapper) DeleteEndpointQueue(
	ctx context.Context,
	model AsbEndpointModel,
) error {
	return runWithRetryIncrementalBackOffVoid(
		ctx, "Deleting queue"+model.EndpointName,
		func() error {
			_, err := w.Client.DeleteQueue(
				ctx,
				model.EndpointName,
				nil,
			)

			return err
		},
	)
}

func (w *AsbClientWrapper) DeleteAdditionalQueue(
	ctx context.Context,
	queueName string,
) error {
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Deleting queue"+queueName,
		func() error {
			_, err := w.Client.DeleteQueue(
				ctx,
				queueName,
				nil,
			)

			return err
		},
	)
}

func (w *AsbClientWrapper) GetEndpointQueue(
	ctx context.Context,
	model AsbEndpointModel,
) (*az.GetQueueResponse, error) {
	return runWithRetryIncrementalBackOff(
		ctx,
		"Getting queue"+model.EndpointName,
		func() (*az.GetQueueResponse, error) {
			return w.Client.GetQueue(
				ctx,
				model.EndpointName,
				nil,
			)
		},
	)
}

func (w *AsbClientWrapper) QueueExists(ctx context.Context, queueName string) (bool, error) {
	return runWithRetryIncrementalBackOff(
		ctx,
		"Getting queue"+queueName,
		func() (bool, error) {
			queue, err := w.Client.GetQueue(ctx, queueName, nil)

			if err != nil {
				return false, err
			}

			if queue == nil {
				return false, nil
			}

			return true, nil
		},
	)
}
