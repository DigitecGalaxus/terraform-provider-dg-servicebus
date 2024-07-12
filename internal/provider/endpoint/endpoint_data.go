package endpoint

import (
	"context"
	"fmt"
	"regexp"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
	if req.ProviderData == nil { // If nil will be configured
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
	EndpointName  types.String                          `tfsdk:"endpoint_name"`
	TopicName     types.String                          `tfsdk:"topic_name"`
	Subscriptions []endpointDataSourceSubscriptionModel `tfsdk:"subscriptions"`
	QueueOptions  *endpointDataSourceQueueOptionsModel  `tfsdk:"queue_options"`
}

type endpointDataSourceSubscriptionModel struct {
	Filter     types.String `tfsdk:"filter"`
	FilterType types.String `tfsdk:"filter_type"`
}

func (d *endpointDataSourceSubscriptionModel) ToAsbModel() asb.AsbSubscriptionModel {
	return asb.AsbSubscriptionModel{
		Filter:     d.Filter.ValueString(),
		FilterType: d.FilterType.ValueString(),
	}
}

type endpointDataSourceQueueOptionsModel struct {
	EnablePartitioning        types.Bool  `tfsdk:"enable_partitioning"`
	MaxSizeInMegabytes        types.Int64 `tfsdk:"max_size_in_megabytes"`
	MaxMessageSizeInKilobytes types.Int64 `tfsdk:"max_message_size_in_kilobytes"`
}

func (d *endpointDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (d *endpointDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The Endpoint data source porvides information about an existing Endpoint.",

		Attributes: map[string]schema.Attribute{
			"endpoint_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the endpoint.",
			},
			"topic_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the topic, in which the endpoint is created",
			},
			"subscriptions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"filter": schema.StringAttribute{
							Computed:    true,
							Description: "The filter for the subscription.",
						},
						"filter_type": schema.StringAttribute{
							Computed:    true,
							Description: "The filter type for the subscription.",
						},
					},
				},
			},
			"queue_options": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The configuration used when creating any queues for that endpoint",
				Attributes: map[string]schema.Attribute{
					"enable_partitioning": schema.BoolAttribute{
						Computed: true,
					},
					"max_size_in_megabytes": schema.Int64Attribute{
						Computed: true,
					},
					"max_message_size_in_kilobytes": schema.Int64Attribute{
						Computed: true,
					},
				},
			},
		},
	}
}

func (model endpointDataSourceModel) ToAsbModel() asb.AsbEndpointModel {

	subscriptions := make([]asb.AsbSubscriptionModel, 0, len(model.Subscriptions))
	for _, subscription := range model.Subscriptions {
		subscriptions = append(subscriptions, subscription.ToAsbModel())
	}

	return asb.AsbEndpointModel{
		EndpointName:  model.EndpointName.ValueString(),
		TopicName:     model.TopicName.ValueString(),
		Subscriptions: subscriptions,
		QueueOptions: asb.AsbEndpointQueueOptions{
			EnablePartitioning:        model.QueueOptions.EnablePartitioning.ValueBoolPointer(),
			MaxSizeInMegabytes:        to.Ptr(int32(model.QueueOptions.MaxSizeInMegabytes.ValueInt64())),
			MaxMessageSizeInKilobytes: to.Ptr(model.QueueOptions.MaxMessageSizeInKilobytes.ValueInt64()),
		},
	}
}

func (d *endpointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state endpointDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	state.QueueOptions = &endpointDataSourceQueueOptionsModel{}

	model := state.ToAsbModel()

	asbSubscriptions, err := d.client.GetAsbSubscriptionsRules(ctx, model)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting Subscriptions",
			"Could not get Subscriptions, unexpected error: "+err.Error(),
		)
		return
	}

	subscriptions := make([]endpointDataSourceSubscriptionModel, 0, len(asbSubscriptions))
	for _, asbSubscription := range asbSubscriptions {
		subscriptions = append(subscriptions, endpointDataSourceSubscriptionModel{
			Filter:     convertAsbFilterToEnpointFilter(asbSubscription),
			FilterType: types.StringValue(asbSubscription.FilterType),
		})
	}

	state.Subscriptions = subscriptions

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
	state.QueueOptions.MaxMessageSizeInKilobytes = types.Int64Value(*queue.MaxMessageSizeInKilobytes)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func convertAsbFilterToEnpointFilter(asbSubscription asb.AsbSubscriptionRule) basetypes.StringValue {
	if asbSubscription.FilterType == "correlation" {
		return basetypes.NewStringValue(asbSubscription.Filter)
	}

	foundFilter := regexp.MustCompile(`^\[NServiceBus\.EnclosedMessageTypes\] LIKE '%([a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*)%'$`).FindStringSubmatch(asbSubscription.Filter)
	return basetypes.NewStringValue(foundFilter[1])
}
