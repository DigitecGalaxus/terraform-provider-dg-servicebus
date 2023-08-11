package asb

const MAX_DELIVERY_COUNT = int32(2147483647)

type EndpointModel struct {
	EndpointName     string
	TopicName        string
	ResourceGroup    string
	Subscriptions    []string
	AdditionalQueues []string
	QueueOptions     EndpointQueueOptions
}

type EndpointQueueOptions struct {
	EnablePartitioning *bool
	MaxSizeInMegabytes *int32
}
