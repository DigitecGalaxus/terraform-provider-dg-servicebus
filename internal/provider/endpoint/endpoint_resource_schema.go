package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// endpointResourceModel maps the resource schema data.
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
				Description: "Debugging attribute to check if the queue exists.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint_exists": schema.BoolAttribute{
				Computed: true,
				Description: "Debugging attribute to check if the endpoint exists.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_create_queue": schema.BoolAttribute{
				Computed: true,
				Description: "Debugging attribute to check if the queue should be created.",
				PlanModifiers: []planmodifier.Bool{
					shouldCreateQueueIfNotExistsModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"should_create_endpoint": schema.BoolAttribute{
				Computed: true,
				Description: "Debugging attribute to check if the endpoint should be created.",
				PlanModifiers: []planmodifier.Bool{
					shouldCreateEndpointIfNotExistsModifier{},
					shouldCreateEndpointIfSubscriberAddedModifier{},
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

type shouldCreateEndpointIfSubscriberAddedModifier struct{}

func (m shouldCreateEndpointIfSubscriberAddedModifier) Description(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfSubscriberAddedModifier) MarkdownDescription(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfSubscriberAddedModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var state endpointResourceModel
	req.State.Get(ctx, &state)

	var plan endpointResourceModel
	req.Plan.Get(ctx, &plan)

	previousSubscriberLen := len(state.Subscriptions)
	afterSubscriberLen := len(plan.Subscriptions)

	if previousSubscriberLen == 0 && afterSubscriberLen > 0 {
		resp.PlanValue = types.BoolValue(true)
	}
}

type shouldCreateEndpointIfNotExistsModifier struct{}

func (m shouldCreateEndpointIfNotExistsModifier) Description(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfNotExistsModifier) MarkdownDescription(_ context.Context) string {
	return "Checks in plan if an Endpoint should be created."
}

func (m shouldCreateEndpointIfNotExistsModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var state endpointResourceModel
	req.State.Get(ctx, &state)

	endpointExists := state.EndpointExists.ValueBool()
	if !endpointExists {
		resp.PlanValue = types.BoolValue(true)
	}
}

type shouldCreateQueueIfNotExistsModifier struct{}

func (m shouldCreateQueueIfNotExistsModifier) Description(_ context.Context) string {
	return "Checks in plan if a Queue should be created."
}

func (m shouldCreateQueueIfNotExistsModifier) MarkdownDescription(_ context.Context) string {
	return "Checks in plan if a Queue should be created."
}

func (m shouldCreateQueueIfNotExistsModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var state endpointResourceModel
	req.State.Get(ctx, &state)

	queueExists := state.QueueExists.ValueBool()
	if !queueExists {
		resp.PlanValue = types.BoolValue(true)
	}
}

func intOneOfValues(values []int64) intOneOfValidator {
	return intOneOfValidator{
		values: values,
	}
}

type intOneOfValidator struct {
	values []int64
}

func (v intOneOfValidator) Description(ctx context.Context) string {
	return fmt.Sprintf("Value must be one of the following: %v", v.values)
}

func (v intOneOfValidator) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("Value must be one of the following: %v", v.values)
}

func (v intOneOfValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	intValue := req.ConfigValue.ValueInt64()

	for _, value := range v.values {
		if value == intValue {
			return
		}
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid value",
		fmt.Sprintf("Value must be one of the following: %v", v.values),
	)
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

func arrayContains(array []string, value string) bool {
	for _, item := range array {
		if item == value {
			return true
		}
	}

	return false
}

func getUniqueElements(array []string) []string {
	seen := make(map[string]bool)
	unique := []string{}
	for _, item := range array {
		if !seen[item] {
			seen[item] = true
			unique = append(unique, item)
		}
	}

	return unique
}
