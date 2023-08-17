package asb

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"regexp"
	"strings"

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
	subscriptionFilterRegex := regexp.MustCompile("\\[NServiceBus.EnclosedMessageTypes\\] LIKE '%(.*)%'")

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

			matches := subscriptionFilterRegex.FindStringSubmatch(sqlFilter.Expression)
			if len(matches) == 0 {
				continue
			}

			subscriptionName := matches[1]
			subscription := Subscription{
				Name:   subscriptionName,
				Filter: sqlFilter.Expression,
			}

			subscriptions[subscriptionName] = subscription
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
			Name: to.Ptr(cropSubscriptionNameToMaxLength(subscriptionName)),
			Filter: &az.SQLFilter{
				Expression: MakeSubscriptionFilter(subscriptionName),
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
		cropSubscriptionNameToMaxLength(subscriptionName),
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
		cropSubscriptionNameToMaxLength(subscriptionName),
		nil,
	)

	return err
}

func cropSubscriptionNameToMaxLength(subscriptionName string) string {
	subscriptionName = strings.Trim(subscriptionName, " ")
	if len(subscriptionName) < 50 {
		return subscriptionName
	}

	return subscriptionName[len(subscriptionName)-50:]
}

func MakeSubscriptionFilter(subscriptionName string) string {
	return "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionName + "%'"
}
