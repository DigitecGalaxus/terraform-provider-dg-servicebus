package provider

import (
	"context"
	"fmt"
	"terraform-provider-nservicebus/internal/provider/asb"

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
	_ resource.ResourceWithConfigure = &endpointResource{}
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
			},
			"resource_group": schema.StringAttribute{
				Required:    true,
				Description: "",
			},
			"subscriptions": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "",
			},
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
		},
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

// type destroyResourceWhenArrayElementIsRemoved struct {}
//
// func (m *destroyResourceWhenArrayElementIsRemoved) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
// 	resp.
// }

// endpointResourceModel maps the resource schema data.
type endpointResourceModel struct {
	EndpointName     types.String                      `tfsdk:"endpoint_name"`
	TopicName        types.String                      `tfsdk:"topic_name"`
	ResourceGroup    types.String                      `tfsdk:"resource_group"`
	Subscriptions    []string                          `tfsdk:"subscriptions"`
	AdditionalQueues []string                          `tfsdk:"additional_queues"`
	QueueOptions     endpointResourceQueueOptionsModel `tfsdk:"queue_options"`
}

type endpointResourceQueueOptionsModel struct {
	EnablePartitioning types.Bool  `tfsdk:"enable_partitioning"`
	MaxSizeInMegabytes types.Int64 `tfsdk:"max_size_in_megabytes"`
}

func (model endpointResourceModel) ToAsbModel() asb.EndpointModel {
	return asb.EndpointModel{
		EndpointName:     model.EndpointName.ValueString(),
		TopicName:        model.TopicName.ValueString(),
		ResourceGroup:    model.ResourceGroup.ValueString(),
		Subscriptions:    model.Subscriptions,
		AdditionalQueues: model.AdditionalQueues,
		QueueOptions: asb.EndpointQueueOptions{
			EnablePartitioning: model.QueueOptions.EnablePartitioning.ValueBoolPointer(),
			MaxSizeInMegabytes: to.Ptr(int32(model.QueueOptions.MaxSizeInMegabytes.ValueInt64())),
		},
	}
}

func (r *endpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (r *endpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := plan.ToAsbModel()
	azureContext := context.Background()

	err := r.client.CreateEndpointQueue(azureContext, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating queue",
			"Could not create queue, unexpected error: "+err.Error(),
		)
		return
	}

	err = r.client.CreateEndpointWithDefaultRule(azureContext, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating subscription",
			"Could not create subscription, unexpected error: "+err.Error(),
		)
		return
	}

	for i := 0; i < len(plan.Subscriptions); i++ {
		err := r.client.CreateEndpointSubscription(azureContext, model, plan.Subscriptions[i])
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating rule",
				"Could not create rule, unexpected error: "+err.Error(),
			)
			return
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	azureContext := context.Background()
	response, err := r.client.Client.GetSubscription(
		azureContext,
		state.TopicName.ValueString(),
		state.EndpointName.ValueString(),
		nil,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading service bus subscription",
			"Error: "+err.Error(),
		)
		return
	}

	endpoint_exists := response != nil
	if !endpoint_exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// Assert queue exists

	// Assert rules exist

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *endpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan endpointResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	previousState := endpointResourceModel{}
	req.State.Get(ctx, &previousState)
	azureContext := context.Background()

	previousStateModel := previousState.ToAsbModel()
	planModel := plan.ToAsbModel()

	if plan.EndpointName.ValueString() != previousState.EndpointName.ValueString() {
		r.client.DeleteEndpointQueue(azureContext, previousStateModel)
		r.client.CreateEndpointQueue(azureContext, planModel)

		r.client.DeleteEndpoint(azureContext, previousStateModel)
		r.client.CreateEndpointWithDefaultRule(azureContext, planModel)
	}

	r.UpdateSubscriptions(azureContext, previousStateModel, planModel)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *endpointResource) UpdateSubscriptions(
	azureContext context.Context,
	previousState asb.EndpointModel,
	plan asb.EndpointModel,
) error {
	subscriptions := getUniqueElements(append(plan.Subscriptions, previousState.Subscriptions...))

	for _, subscription := range subscriptions {
		shouldBeDeleted := !arrayContains(plan.Subscriptions, subscription)
		if shouldBeDeleted {
			err := r.client.DeleteEndpointSubscription(azureContext, plan, subscription)
			if err != nil {
				return err
			}
			continue
		}

		shouldBeCreated := !arrayContains(previousState.Subscriptions, subscription)
		if shouldBeCreated {
			err := r.client.CreateEndpointSubscription(azureContext, plan, subscription)
			if err != nil {
				return err
			}
		}

		// Exists and should stay like that
	}

	return nil
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

func (r *endpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan endpointResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := plan.ToAsbModel()
	azureContext := context.Background()

	err := r.client.DeleteEndpointQueue(azureContext, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting queue",
			"Could not delete queue, unexpected error: "+err.Error(),
		)
		return
	}

	err = r.client.DeleteEndpoint(azureContext, model)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting subscription",
			"Could not delete subscription, unexpected error: "+err.Error(),
		)
		return
	}
}
