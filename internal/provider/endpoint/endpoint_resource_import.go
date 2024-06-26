package endpoint

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func (r *endpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")
	hasNonEmptySubscriptionArgument := len(idParts) == 3 && idParts[2] != ""
	hasNoSubscriptionArgument := len(idParts) == 2
	hasEmptyIdValues := len(idParts) >= 2 && idParts[0] == "" && idParts[1] == ""

	if !(hasNonEmptySubscriptionArgument || hasNoSubscriptionArgument) || hasEmptyIdValues {
		resp.Diagnostics.AddWarning("Foo", fmt.Sprintf("%t %t %t", hasNonEmptySubscriptionArgument, hasNoSubscriptionArgument, hasEmptyIdValues))
		resp.Diagnostics.AddError(
			"Unexpected import Identifier",
			fmt.Sprintf(
				"Expected two or three identifier in format: topic_name,endpoint_name,<SubscriptionOne;SubscriptionTwo>. Got %q",
				req.ID,
			),
		)

		return
	}

	model := asb.AsbEndpointModel{TopicName: idParts[0], EndpointName: idParts[1]}

	subscriptionNames := []string{}
	if hasNonEmptySubscriptionArgument {
		subscriptionNames = strings.Split(idParts[2], ";")
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("topic_name"), model.TopicName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_name"), model.EndpointName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("queue_options"), &endpointResourceQueueOptionsModel{})...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subscriptions"), subscriptionNames)...)
}
