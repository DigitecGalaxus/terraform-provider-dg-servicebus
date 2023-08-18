package asb

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

type Subscription struct {
	Name   string
	Filter string
}

func (w *AsbClientWrapper) GetEndpointSubscriptions(
	ctx context.Context,
	model EndpointModel,
) (map[string]Subscription, error) {
	subscriptions := map[string]Subscription{}

	pager := w.Client.NewListRulesPager(
		model.TopicName,
		model.EndpointName,
		nil,
	)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, rule := range page.Rules {
			if rule.Name == "$Default" {
				continue
			}

			sqlFilter, ok := rule.Filter.(*az.SQLFilter)
			if !ok {
				continue
			}

			subscription := Subscription{
				Name:   rule.Name,
				Filter: sqlFilter.Expression,
			}

			subscriptions[rule.Name] = subscription
		}
	}

	return subscriptions, nil
}

func (w *AsbClientWrapper) CreateEndpointSubscription(
	azureContext context.Context,
	plan EndpointModel,
	subscriptionName string,
) error {
	_, err := w.Client.CreateRule(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		&az.CreateRuleOptions{
			Name: to.Ptr(CropSubscriptionNameToMaxLength(subscriptionName)),
			Filter: &az.SQLFilter{
				Expression: makeSubscriptionFilter(subscriptionName),
			},
		},
	)

	return err
}

func (w *AsbClientWrapper) EndpointSubscriptionExists(
	azureContext context.Context,
	plan EndpointModel,
	subscriptionName string,
) bool {
	_, err := w.Client.GetRule(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		CropSubscriptionNameToMaxLength(subscriptionName),
		nil,
	)

	return err == nil
}

func (w *AsbClientWrapper) DeleteEndpointSubscription(
	azureContext context.Context,
	plan EndpointModel,
	subscriptionName string,
) error {
	_, err := w.Client.DeleteRule(
		azureContext,
		plan.TopicName,
		plan.EndpointName,
		CropSubscriptionNameToMaxLength(subscriptionName),
		nil,
	)

	return err
}

func (w *AsbClientWrapper) EnsureEndpointSubscriptionFilterCorrect(
	ctx context.Context,
	plan EndpointModel,
	subscriptionName string,
) error {
	subscriptionFilter := makeSubscriptionFilter(subscriptionName)

	_, err := w.Client.UpdateRule(
		ctx,
		plan.TopicName,
		plan.EndpointName,
		az.RuleProperties{
			Name: CropSubscriptionNameToMaxLength(subscriptionName),
			Filter: &az.SQLFilter{
				Expression: subscriptionFilter,
			},
		},
	)

	return err
}

func IsFilterCorrect(filter string, subscriptionName string) bool {
	return filter == makeSubscriptionFilter(subscriptionName)
}

func CropSubscriptionNameToMaxLength(subscriptionName string) string {
	subscriptionName = strings.Trim(subscriptionName, " ")
	if len(subscriptionName) < 50 {
		return subscriptionName
	}

	return subscriptionName[len(subscriptionName)-50:]
}

func makeSubscriptionFilter(subscriptionName string) string {
	return "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionName + "%'"
}
