package provider

import (
	"context"
	"fmt"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)


var (
	_ datasource.DataSource = &endpointDataSource{}
	_ datasource.DataSourceWithConfigure = &endpointDataSource{}
)

func NewEndpointDataSource() datasource.DataSource {
	return &endpointDataSource{}
}

type endpointDataSource struct{
	client *asb.AsbClientWrapper
}

func (d *endpointDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
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

type endpointDataModel struct {
	EndpointName     types.String                      `tfsdk:"endpoint_name"`
	TopicName        types.String                      `tfsdk:"topic_name"`
	Subscriptions    []string                          `tfsdk:"subscriptions"`
}

func (d *endpointDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (d *endpointDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_name": schema.StringAttribute{
				Required: true,
			},
			"topic_name": schema.StringAttribute{
				Required: true,
			},
			"subscriptions": schema.ListAttribute{
				Computed: true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *endpointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state endpointDataModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	endpointState := endpointDataModel{
		TopicName: state.TopicName,
		EndpointName: state.EndpointName,
	}

	pager := d.client.Client.NewListRulesPager(
		state.TopicName.ValueString(),
		state.EndpointName.ValueString(),
		&az.ListRulesOptions{MaxPageSize: 10},
	)


	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to advance rules page: %v", err.Error(),
			)
		}
		for _, rule := range page.Rules {
			endpointState.Subscriptions = append(endpointState.Subscriptions, rule.Name)
		}
	}

	state = endpointState

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}