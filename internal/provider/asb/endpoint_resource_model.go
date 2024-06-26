package asb

const MAX_DELIVERY_COUNT = int32(2147483647)

// Curren version
type AsbEndpointModel struct {
	EndpointName     string
	TopicName        string
	ResourceGroup    string
	Subscriptions    []AsbSubscriptionModel
	AdditionalQueues []string
	QueueOptions     AsbEndpointQueueOptions
}

type AsbSubscriptionModel struct {
	Filter     string
	FilterType string
}

type AsbEndpointQueueOptions struct {
	EnablePartitioning        *bool
	MaxSizeInMegabytes        *int32
	MaxMessageSizeInKilobytes *int64
}

// Previous version
type AsbEndpointModelV0 struct {
	EndpointName     string
	TopicName        string
	ResourceGroup    string
	Subscriptions    []string
	AdditionalQueues []string
	QueueOptions     AsbEndpointQueueOptions
}
