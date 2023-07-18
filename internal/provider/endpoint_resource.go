package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/hashicorp-demoapp/hashicups-client-go"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
		_ resource.ResourceWithConfigure = &endpointResource{}
)

// NewOrderResource is a helper function to simplify the provider implementation.
func NewOrderResource() resource.Resource {
    return &endpointResource{}
}

// endpointResource is the resource implementation.
// endpointResource is the resource implementation.
type endpointResource struct {
	client *azservicebus.Client
}

func (r *endpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
			return
	}


	azidentity.AzureCLICredential{}

	client, ok := req.ProviderData.(*hashicups.Client)

	if !ok {
			resp.Diagnostics.AddError(
					"Unexpected Data Source Configure Type",
					fmt.Sprintf("Expected *hashicups.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
			)

			return
	}

	r.client = client
}

func (r *endpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
			Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
							Computed: true,
					},
					"last_updated": schema.StringAttribute{
							Computed: true,
					},
					"endpoint_name": schema.StringAttribute{
						Required: true,
						Description: "",
					},
					"environment": schema.StringAttribute{
						Required: true,
						Description: "",
					},
					"module_name": schema.StringAttribute{
						Required: true,
						Description: "",
					},
					"product_group": schema.StringAttribute{
						Required: true,
						Description: "",
					},
					"subscriptions": schema.ListAttribute{
							Required: true,
							ElementType: types.StringType,
							Description: "",
					},
			},
	}
}

// orderResourceModel maps the resource schema data.
type orderResourceModel struct {
	ID          types.String     `tfsdk:"id"`
	Items       []orderItemModel `tfsdk:"items"`
	LastUpdated types.String     `tfsdk:"last_updated"`
}

// orderItemModel maps order item data.
type orderItemModel struct {
	Coffee   orderItemCoffeeModel `tfsdk:"coffee"`
	Quantity types.Int64          `tfsdk:"quantity"`
}

// orderItemCoffeeModel maps coffee order item data.
type orderItemCoffeeModel struct {
	ID          types.Int64   `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Teaser      types.String  `tfsdk:"teaser"`
	Description types.String  `tfsdk:"description"`
	Price       types.Float64 `tfsdk:"price"`
	Image       types.String  `tfsdk:"image"`
}

// Metadata returns the resource type name.
func (r *endpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_order"
}

// Create a new resource.
func (r *endpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan orderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
			return
	}

	// Generate API request body from plan
	var items []hashicups.OrderItem
	for _, item := range plan.Items {
			items = append(items, hashicups.OrderItem{
					Coffee: hashicups.Coffee{
							ID: int(item.Coffee.ID.ValueInt64()),
					},
					Quantity: int(item.Quantity.ValueInt64()),
			})
	}

	// Create new order
	order, err := r.client.CreateOrder(items)
	if err != nil {
			resp.Diagnostics.AddError(
					"Error creating order",
					"Could not create order, unexpected error: "+err.Error(),
			)
			return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ID = types.StringValue(strconv.Itoa(order.ID))
	for orderItemIndex, orderItem := range order.Items {
			plan.Items[orderItemIndex] = orderItemModel{
					Coffee: orderItemCoffeeModel{
							ID:          types.Int64Value(int64(orderItem.Coffee.ID)),
							Name:        types.StringValue(orderItem.Coffee.Name),
							Teaser:      types.StringValue(orderItem.Coffee.Teaser),
							Description: types.StringValue(orderItem.Coffee.Description),
							Price:       types.Float64Value(orderItem.Coffee.Price),
							Image:       types.StringValue(orderItem.Coffee.Image),
					},
					Quantity: types.Int64Value(int64(orderItem.Quantity)),
			}
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
			return
	}
}
// Read resource information.
func (r *endpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
			var state orderResourceModel
			diags := req.State.Get(ctx, &state)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
					return
			}
	
			// Get refreshed order value from HashiCups
			order, err := r.client.GetOrder(state.ID.ValueString())
			if err != nil {
					resp.Diagnostics.AddError(
							"Error Reading HashiCups Order",
							"Could not read HashiCups order ID "+state.ID.ValueString()+": "+err.Error(),
					)
					return
			}
	
			// Overwrite items with refreshed state
			state.Items = []orderItemModel{}
			for _, item := range order.Items {
					state.Items = append(state.Items, orderItemModel{
							Coffee: orderItemCoffeeModel{
									ID:          types.Int64Value(int64(item.Coffee.ID)),
									Name:        types.StringValue(item.Coffee.Name),
									Teaser:      types.StringValue(item.Coffee.Teaser),
									Description: types.StringValue(item.Coffee.Description),
									Price:       types.Float64Value(item.Coffee.Price),
									Image:       types.StringValue(item.Coffee.Image),
							},
							Quantity: types.Int64Value(int64(item.Quantity)),
					})
			}
	
			// Set refreshed state
			diags = resp.State.Set(ctx, &state)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
					return
			}
	}
// Update updates the resource and sets the updated Terraform state on success.
func (r *endpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *endpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}