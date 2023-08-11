package endpoint

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &endpointDataSource{}
	_ datasource.DataSourceWithConfigure = &endpointDataSource{}
)

func NewEndpointDataSource() datasource.DataSource {
	return &endpointDataSource{}
}

type endpointDataSource struct {
	client *asb.AsbClientWrapper
}

func (d *endpointDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		resp.Diagnostics.AddError(
			"Provider Data not set",
			"Endpoint cannot be read, Provider Data is missing",
		)
		return
	}

	client, ok := req.ProviderData.(*az.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data source Configuration Type",
			fmt.Sprintf("Expected *azservicebus.Client, got %T", req.ProviderData),
		)

		return
	}

	d.client = &asb.AsbClientWrapper{
		Client: client,
	}
}

type endpointDataSourceModel struct {
	EndpointName  types.String                         `tfsdk:"endpoint_name"`
	TopicName     types.String                         `tfsdk:"topic_name"`
	Subscriptions []string                             `tfsdk:"subscriptions"`
	QueueOptions  *endpointDataSourceQueueOptionsModel `tfsdk:"queue_options"`
}

type endpointDataSourceQueueOptionsModel struct {
	EnablePartitioning types.Bool  `tfsdk:"enable_partitioning"`
	MaxSizeInMegabytes types.Int64 `tfsdk:"max_size_in_megabytes"`
}

func (d *endpointDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (d *endpointDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_name": schema.StringAttribute{
				Required: true,
				Description: "The name of the endpoint.",
			},
			"topic_name": schema.StringAttribute{
				Required: true,
				Description: "The name of the topic, in which the endpoint is created",
			},
			"subscriptions": schema.ListAttribute{
				Computed:    true,
				Description: "A list of all subscriptions the endpoint has",
				ElementType: types.StringType,
			},
			"queue_options": schema.SingleNestedAttribute{
				Computed: true,
				Description: "The configuration used when creating any queues for that endpoint",
				Attributes: map[string]schema.Attribute{
					"enable_partitioning": schema.BoolAttribute{
						Computed: true,
					},
					"max_size_in_megabytes": schema.Int64Attribute{
						Computed: true,
					},
				},
			},
		},
	}
}

func (model endpointDataSourceModel) ToAsbModel() asb.EndpointModel {
	return asb.EndpointModel{
		EndpointName:  model.EndpointName.ValueString(),
		TopicName:     model.TopicName.ValueString(),
		Subscriptions: model.Subscriptions,
		QueueOptions: asb.EndpointQueueOptions{
			EnablePartitioning: model.QueueOptions.EnablePartitioning.ValueBoolPointer(),
			MaxSizeInMegabytes: to.Ptr(int32(model.QueueOptions.MaxSizeInMegabytes.ValueInt64())),
		},
	}
}

func (d *endpointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state endpointDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	state.QueueOptions = &endpointDataSourceQueueOptionsModel{}

	model := state.ToAsbModel()

	subscriptions, err := d.client.GetEndpointSubscriptions(ctx, model)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting Subscriptions",
			"Could not get Subscriptions, unexpected error: "+err.Error(),
		)
		return
	}

	subscriptionNames := make([]string, 0, len(subscriptions))
	for subscription := range subscriptions {
		subscriptionNames = append(subscriptionNames, subscription)
	}

	state.Subscriptions = subscriptionNames

	queue, err := d.client.GetEndpointQueue(ctx, model)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting Queue",
			"Could not get Queue, unexpected error: "+err.Error(),
		)
		return
	}

	if queue == nil {
		resp.Diagnostics.AddError(
			"Queue does not exist",
			fmt.Sprintf("No Queue for Endpoint %s exist", state.EndpointName.ValueString()),
		)
		return
	}

	state.QueueOptions.EnablePartitioning = types.BoolValue(*queue.EnablePartitioning)
	state.QueueOptions.MaxSizeInMegabytes = types.Int64Value(int64(*queue.MaxSizeInMegabytes))

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
