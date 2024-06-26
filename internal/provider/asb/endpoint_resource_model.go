package asb

const MAX_DELIVERY_COUNT = int32(2147483647)

type AsbEndpointModel struct {
	EndpointName           string
	TopicName              string
	ResourceGroup          string
	Subscriptions          []string
	SubscriptionFilterType string
	AdditionalQueues       []string
	QueueOptions           AsbEndpointQueueOptions
}

type AsbEndpointQueueOptions struct {
	EnablePartitioning        *bool
	MaxSizeInMegabytes        *int32
	MaxMessageSizeInKilobytes *int64
}
