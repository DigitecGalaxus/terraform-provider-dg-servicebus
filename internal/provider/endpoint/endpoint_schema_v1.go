package endpoint

import (
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewSchemaV1() schema.Schema {
	return schema.Schema{
		Version: 1,

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
			"subscriptions": schema.SetNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"filter": schema.StringAttribute{
							Required:    true,
							Description: "The filter for the subscription.",
							Validators: []validator.String{
								isValidCorrelationFilter(),
							},
						},
						"filter_type": schema.StringAttribute{
							Required:    true,
							Description: "The filter type for the subscription.",
							Validators: []validator.String{
								stringvalidator.OneOf("correlation", "sql"),
							},
						},
					},
				},
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

type endpointResourceModel struct {
	EndpointName              types.String                      `tfsdk:"endpoint_name"`
	TopicName                 types.String                      `tfsdk:"topic_name"`
	Subscriptions             []SubscriptionModel               `tfsdk:"subscriptions"`
	AdditionalQueues          []string                          `tfsdk:"additional_queues"`
	QueueOptions              endpointResourceQueueOptionsModel `tfsdk:"queue_options"`
	QueueExists               types.Bool                        `tfsdk:"queue_exists"`
	HasMalformedFilters       types.Bool                        `tfsdk:"has_malformed_filters"`
	EndpointExists            types.Bool                        `tfsdk:"endpoint_exists"`
	ShouldCreateQueue         types.Bool                        `tfsdk:"should_create_queue"`
	ShouldCreateEndpoint      types.Bool                        `tfsdk:"should_create_endpoint"`
	ShouldUpdateSubscriptions types.Bool                        `tfsdk:"should_update_subscriptions"`
}

type SubscriptionModel struct {
	Filter     types.String `tfsdk:"filter"`
	FilterType types.String `tfsdk:"filter_type"`
}

func (sm *SubscriptionModel) ToAsbModel() asb.AsbSubscriptionModel {
	return asb.AsbSubscriptionModel{
		Filter:     sm.Filter.ValueString(),
		FilterType: sm.FilterType.ValueString(),
	}
}

type endpointResourceQueueOptionsModel struct {
	EnablePartitioning        types.Bool  `tfsdk:"enable_partitioning"`
	MaxSizeInMegabytes        types.Int64 `tfsdk:"max_size_in_megabytes"`
	MaxMessageSizeInKilobytes types.Int64 `tfsdk:"max_message_size_in_kilobytes"`
}

func (model endpointResourceModel) ToAsbModel() asb.AsbEndpointModel {
	subscriptions := make([]asb.AsbSubscriptionModel, len(model.Subscriptions))
	for i, subscription := range model.Subscriptions {
		subscriptions[i] = subscription.ToAsbModel()
	}

	return asb.AsbEndpointModel{
		EndpointName:     model.EndpointName.ValueString(),
		TopicName:        model.TopicName.ValueString(),
		Subscriptions:    subscriptions,
		AdditionalQueues: model.AdditionalQueues,
		QueueOptions: asb.AsbEndpointQueueOptions{
			EnablePartitioning:        model.QueueOptions.EnablePartitioning.ValueBoolPointer(),
			MaxSizeInMegabytes:        to.Ptr(int32(model.QueueOptions.MaxSizeInMegabytes.ValueInt64())),
			MaxMessageSizeInKilobytes: to.Ptr(model.QueueOptions.MaxMessageSizeInKilobytes.ValueInt64()),
		},
	}
}
