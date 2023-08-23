package endpoint

// import (
// 	"context"
// 	"strings"
// 	"terraform-provider-dg-servicebus/internal/provider/asb"

// 	"github.com/hashicorp/terraform-plugin-framework/resource"
// 	"golang.org/x/exp/slices"
// )

// func (e *endpointResource) ModifyPlan(
// 	ctx context.Context,
// 	req resource.ModifyPlanRequest,
// 	resp *resource.ModifyPlanResponse,
// ) {
// 	resourceWillBeDestroyed := req.Plan.Raw.IsNull()
// 	if resourceWillBeDestroyed {
// 		return
// 	}

// 	var state endpointResourceModel
// 	req.State.Get(ctx, &state)

// 	var config endpointResourceModel
// 	req.Config.Get(ctx, &config)

// 	var plan endpointResourceModel
// 	req.Plan.Get(ctx, &plan)

// 	updatedPlan := addRemoteSubscriptionsToPlan(ctx, e.client, &config, &state, &plan, resp)
// 	resp.Plan.Set(ctx, &updatedPlan)
// }

// func addRemoteSubscriptionsToPlan(
// 	ctx context.Context,
// 	client *asb.AsbClientWrapper,
// 	config *endpointResourceModel,
// 	state *endpointResourceModel,
// 	plan *endpointResourceModel,
// 	resp *resource.ModifyPlanResponse,
// ) *endpointResourceModel {
// 	isFirstPlanAfterImport := len(state.Subscriptions) == 0 && len(plan.Subscriptions) == 0 && len(config.Subscriptions) > 0
// 	if !isFirstPlanAfterImport {
// 		return plan
// 	}

// 	resp.Diagnostics.AddWarning(
// 		"Subscriptions cannot be imported",
// 		"Subscriptions are only imported after an apply (resource update), which does not make any changes to the remote resource.",
// 	)

// 	getFullSubscriptionNameBySuffixInConfig := func(subscriptionSuffix string) *string {
// 		for _, subscription := range config.Subscriptions {
// 			if strings.HasSuffix(subscription, subscriptionSuffix) {
// 				return &subscription
// 			}
// 		}

// 		return nil
// 	}

// 	remoteSubscriptions, err := client.GetEndpointSubscriptions(ctx, state.ToAsbModel())
// 	if err != nil {
// 		resp.Diagnostics.AddError(
// 			"Failed to get remote subscriptions",
// 			"Failed to get remote subscriptions: "+err.Error(),
// 		)

// 		return plan
// 	}

// 	planSubscriptions := plan.Subscriptions
// 	for _, remoteSubscription := range remoteSubscriptions {
// 		fullRemoteName := getFullSubscriptionNameBySuffixInConfig(remoteSubscription.Name)
// 		if fullRemoteName == nil {
// 			// The subscription is not managed by terraform or should no longer exist
// 			// omit it here such that it will be deleted on the next apply
// 			continue
// 		}

// 		if !slices.Contains(planSubscriptions, *fullRemoteName) {
// 			planSubscriptions = append(planSubscriptions, *fullRemoteName)
// 		}
// 	}

// 	plan.Subscriptions = planSubscriptions
// 	return plan
// }
