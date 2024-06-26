package endpoint

import (
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewSchemaV0() schema.Schema {
	return schema.Schema{
		Description: "The Endpoint resource allows consumers to create and manage an NServiceBus Endpoint. " +
			"When initially creating the Endpoint, a default deny-all rule ensures that no invalid messages are received, before the configured subscription rules have been applied.",

		Attributes: map[string]schema.Attribute{
			"endpoint_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the endpoint to create.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"topic_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the topic to create the endpoint on.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subscriptions": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "The list of subscriptions to create on the endpoint.",
			},
			"additional_queues": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Additional queues to create for the endpoint.",
			},
			"queue_options": schema.SingleNestedAttribute{
				Required:    true,
				Description: "The options for the queue, which is created for the endpoint.",
				Attributes: map[string]schema.Attribute{
					"enable_partitioning": schema.BoolAttribute{
						Required: true,
					},
					"max_size_in_megabytes": schema.Int64Attribute{
						Required: true,
						Validators: []validator.Int64{
							intOneOfValues([]int64{1024, 2048, 3072, 4096, 5120}),
						},
					},
					"max_message_size_in_kilobytes": schema.Int64Attribute{
						Required: true,
					},
				},
			},
			"queue_exists": schema.BoolAttribute{
				Computed:    true,
				Description: "Internal attribute used to track whether the queue exists.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint_exists": schema.BoolAttribute{
				Computed:    true,
				Description: "Internal attribute used to track whether the endpoint exists.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"has_malformed_filters": schema.BoolAttribute{
				Computed:    true,
				Description: "Internal attribute used to track whether the endpoint has malformed filters.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_create_queue": schema.BoolAttribute{
				Computed:    true,
				Description: "Internal attribute used to track whether the queue should be created.",
				PlanModifiers: []planmodifier.Bool{
					shouldCreateQueueIfNotExistsModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_create_endpoint": schema.BoolAttribute{
				Computed:    true,
				Description: "Internal attribute used to track whether the endpoint should be created.",
				PlanModifiers: []planmodifier.Bool{
					shouldCreateEndpointIfNotExistsModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_update_subscriptions": schema.BoolAttribute{
				Computed:    true,
				Description: "Internal attribute used to track whether the subscriptions should be updated.",
				PlanModifiers: []planmodifier.Bool{
					shouldUpdateMalformedEndpointSubscriptionModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

type endpointResourceModelV0 struct {
	EndpointName              types.String                      `tfsdk:"endpoint_name"`
	TopicName                 types.String                      `tfsdk:"topic_name"`
	Subscriptions             []string                          `tfsdk:"subscriptions"`
	AdditionalQueues          []string                          `tfsdk:"additional_queues"`
	QueueOptions              endpointResourceQueueOptionsModel `tfsdk:"queue_options"`
	QueueExists               types.Bool                        `tfsdk:"queue_exists"`
	HasMalformedFilters       types.Bool                        `tfsdk:"has_malformed_filters"`
	EndpointExists            types.Bool                        `tfsdk:"endpoint_exists"`
	ShouldCreateQueue         types.Bool                        `tfsdk:"should_create_queue"`
	ShouldCreateEndpoint      types.Bool                        `tfsdk:"should_create_endpoint"`
	ShouldUpdateSubscriptions types.Bool                        `tfsdk:"should_update_subscriptions"`
}

func (model endpointResourceModelV0) ToAsbModel() asb.AsbEndpointModelV0 {
	return asb.AsbEndpointModelV0{
		EndpointName:     model.EndpointName.ValueString(),
		TopicName:        model.TopicName.ValueString(),
		Subscriptions:    model.Subscriptions,
		AdditionalQueues: model.AdditionalQueues,
		QueueOptions: asb.AsbEndpointQueueOptions{
			EnablePartitioning:        model.QueueOptions.EnablePartitioning.ValueBoolPointer(),
			MaxSizeInMegabytes:        to.Ptr(int32(model.QueueOptions.MaxSizeInMegabytes.ValueInt64())),
			MaxMessageSizeInKilobytes: to.Ptr(model.QueueOptions.MaxMessageSizeInKilobytes.ValueInt64()),
		},
	}
}
