package endpoint

import (
	"context"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)



var (
	_ resource.Resource                = &endpointResource{}
	_ resource.ResourceWithConfigure   = &endpointResource{}
	_ resource.ResourceWithImportState = &endpointResource{}
)

func NewEndpointResource() resource.Resource {
	return &endpointResource{}
}

type endpointResource struct {
	client *asb.AsbClientWrapper
}

func (r *endpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = &asb.AsbClientWrapper{
		Client: req.ProviderData.(*az.Client),
	}
}

func (r *endpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

type endpointResourceModel struct {
	EndpointName         types.String                      `tfsdk:"endpoint_name"`
	TopicName            types.String                      `tfsdk:"topic_name"`
	Subscriptions        []string                          `tfsdk:"subscriptions"`
	AdditionalQueues     []string                          `tfsdk:"additional_queues"`
	QueueOptions         endpointResourceQueueOptionsModel `tfsdk:"queue_options"`
	QueueExists          types.Bool                        `tfsdk:"queue_exists"`
	EndpointExists       types.Bool                        `tfsdk:"endpoint_exists"`
	ShouldCreateQueue    types.Bool                        `tfsdk:"should_create_queue"`
	ShouldCreateEndpoint types.Bool                        `tfsdk:"should_create_endpoint"`
}

type endpointResourceQueueOptionsModel struct {
	EnablePartitioning types.Bool  `tfsdk:"enable_partitioning"`
	MaxSizeInMegabytes types.Int64 `tfsdk:"max_size_in_megabytes"`
}

func (r *endpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
			"subscriptions": schema.ListAttribute{
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
				Required: true,
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
				},
			},
			"queue_exists": schema.BoolAttribute{
				Computed: true,
				Description: "Internal attribute used to track whether the queue exists.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint_exists": schema.BoolAttribute{
				Computed: true,
				Description: "Internal attribute used to track whether the endpoint exists.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_create_queue": schema.BoolAttribute{
				Computed: true,
				Description: "Internal attribute used to track whether the queue should be created.",
				PlanModifiers: []planmodifier.Bool{
					shouldCreateQueueIfNotExistsModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_create_endpoint": schema.BoolAttribute{
				Computed: true,
				Description: "Internal attribute used to track whether the endpoint should be created.",
				PlanModifiers: []planmodifier.Bool{
					shouldCreateEndpointIfNotExistsModifier{},
					shouldCreateEndpointIfSubscriberAddedModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (model endpointResourceModel) ToAsbModel() asb.EndpointModel {
	return asb.EndpointModel{
		EndpointName:     model.EndpointName.ValueString(),
		TopicName:        model.TopicName.ValueString(),
		Subscriptions:    model.Subscriptions,
		AdditionalQueues: model.AdditionalQueues,
		QueueOptions: asb.EndpointQueueOptions{
			EnablePartitioning: model.QueueOptions.EnablePartitioning.ValueBoolPointer(),
			MaxSizeInMegabytes: to.Ptr(int32(model.QueueOptions.MaxSizeInMegabytes.ValueInt64())),
		},
	}
}
