package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

// endpointResourceModel maps the resource schema data.
type endpointResourceModel struct {
	EndpointName     types.String                      `tfsdk:"endpoint_name"`
	TopicName        types.String                      `tfsdk:"topic_name"`
	Subscriptions    []string                          `tfsdk:"subscriptions"`
	AdditionalQueues []string                          `tfsdk:"additional_queues"`
	QueueOptions     endpointResourceQueueOptionsModel `tfsdk:"queue_options"`
	QueueExists     types.Bool                        `tfsdk:"queue_exists"`
	EndpointExists     types.Bool           `tfsdk:"endpoint_exists"`
	ShouldCreateQueue     types.Bool                        `tfsdk:"should_create_queue"`
	ShouldCreateEndpoint     types.Bool                        `tfsdk:"should_create_endpoint"`
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
				Description: "",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"topic_name": schema.StringAttribute{
				Required:    true,
				Description: "",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subscriptions": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "",			},
			"additional_queues": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "",
			},
			"queue_options": schema.SingleNestedAttribute{
				Required: true,
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
			},
			"endpoint_exists": schema.BoolAttribute{
				Computed: true,
			},
			"should_create_queue": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					shouldCreateQueueIfNotExistsModifier{},
				},
			},
			"should_create_endpoint": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					shouldCreateEndpointIfNotExistsModifier{},
				},
			},
		},
	}
}

// uignoreListOrderMopdifierimplements the plan modifier.
type shouldCreateEndpointIfNotExistsModifier struct{}

// Description returns a human-readable description of the plan modifier.
func (m shouldCreateEndpointIfNotExistsModifier) Description(_ context.Context) string {
	return "Once set, the value of this attribute in state will not change."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m shouldCreateEndpointIfNotExistsModifier) MarkdownDescription(_ context.Context) string {
	return "Once set, the value of this attribute in state will not change."
}

// PlanModifyBool implements the plan modification logic.
func (m shouldCreateEndpointIfNotExistsModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// Do nothing if there is no state value.
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

// uignoreListOrderMopdifierimplements the plan modifier.
type shouldCreateQueueIfNotExistsModifier struct{}

// Description returns a human-readable description of the plan modifier.
func (m shouldCreateQueueIfNotExistsModifier) Description(_ context.Context) string {
	return "Once set, the value of this attribute in state will not change."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m shouldCreateQueueIfNotExistsModifier) MarkdownDescription(_ context.Context) string {
	return "Once set, the value of this attribute in state will not change."
}

// PlanModifyBool implements the plan modification logic.
func (m shouldCreateQueueIfNotExistsModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// Do nothing if there is no state value.
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

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v intOneOfValidator) Description(ctx context.Context) string {
	return fmt.Sprintf("Value must be one of the following: %v", v.values)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v intOneOfValidator) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("Value must be one of the following: %v", v.values)
}

// Validate runs the main validation logic of the validator, reading configuration data out of `req` and updating `resp` with diagnostics.
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
