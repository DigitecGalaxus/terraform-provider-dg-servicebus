package asb

import (
	"context"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

type Subscription struct {
	Name string
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
				Name: rule.Name,
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
			Name: &subscriptionName,
			Filter: &az.SQLFilter{
				Expression: MakeSubscriptionFilter(subscriptionName),
			},
		},
	)

	return err
}

func MakeSubscriptionFilter(subscriptionName string) string {
	return "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionName + "%'"
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
		subscriptionName,
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
		subscriptionName,
		nil,
	)

	return err
}
