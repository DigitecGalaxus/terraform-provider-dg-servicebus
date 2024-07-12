package asb

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

type AsbSubscriptionRule struct {
	Name       string // The name of the rule
	Filter     string // The filter of the rule. When sql this is the complete Filter expression, when correlation this is the value application property "Dg.CorrelationFilterType"
	FilterType string // The type of the filter. Can be "sql" or "correlation"
}

const MAX_RULE_NAME_LENGTH = 50
const SHA_1_BYTE_LENGTH = 20
const SUBSCRIPTION_NAME_IDENTIFIER_LENGTH = SHA_1_BYTE_LENGTH / 2
const SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR = "--"
const CORRELATIONFILTER_HEADER = "Dg.MessageTypeFullName"

func (w *AsbClientWrapper) GetAsbSubscriptionsRules(
	ctx context.Context,
	model AsbEndpointModel,
) ([]AsbSubscriptionRule, error) {
	subscriptions := []AsbSubscriptionRule{}
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

			subscription, err := convertToAsbSubscriptionRule(rule)
			if err != nil {
				tflog.Error(ctx, "Error converting subscription rule: "+err.Error())
				continue
			}

			subscriptions = append(subscriptions, *subscription)
		}
	}

	return subscriptions, nil
}

func convertToAsbSubscriptionRule(rule az.RuleProperties) (*AsbSubscriptionRule, error) {
	if ruleFilter, ok := rule.Filter.(*az.CorrelationFilter); ok {
		ruleFilterValue, ok := ruleFilter.ApplicationProperties[CORRELATIONFILTER_HEADER].(string)
		if !ok {
			return nil, fmt.Errorf("rule filter could not be converted to string")
		}

		return &AsbSubscriptionRule{
			Name:       rule.Name,
			Filter:     ruleFilterValue,
			FilterType: "correlation",
		}, nil
	}

	if ruleFilter, ok := rule.Filter.(*az.SQLFilter); ok {
		if !ok {
			return nil, fmt.Errorf("rule filter could not be converted to string")
		}
		return &AsbSubscriptionRule{
			Name:       rule.Name,
			Filter:     ruleFilter.Expression,
			FilterType: "sql",
		}, nil
	}

	return nil, fmt.Errorf("invalid subscription filter type")
}

func (w *AsbClientWrapper) CreateAsbSubscriptionRule(
	ctx context.Context,
	model AsbEndpointModel,
	subscription AsbSubscriptionModel,
) error {

	// Retry 3 times as the create rule operation can fail with a 400 error,
	// which is a transient error, if another operation is in progress
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Creating subscription rule "+subscription.Filter+" with filter type "+subscription.FilterType,
		func() error {
			_, err := w.Client.CreateRule(
				ctx,
				model.TopicName,
				model.EndpointName,
				&az.CreateRuleOptions{
					Name:   to.Ptr(w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscription.Filter)),
					Filter: createSubscriptionRule(subscription.FilterType, subscription.Filter),
				},
			)

			return err
		})
}

func createSubscriptionRule(subscriptionFilterType, subscriptionFilterValue string) az.RuleFilter {
	switch subscriptionFilterType {
	case "correlation":
		return makeSubscriptionCorrelationRuleFilter(subscriptionFilterValue)
	case "sql":
		return makeSubscriptionSqlRuleFilter(subscriptionFilterValue)
	default:
		tflog.Error(context.Background(), "Invalid subscription filter type: "+subscriptionFilterType)
		return nil
	}
}

func (w *AsbClientWrapper) GetAsbSubscriptionRule(
	ctx context.Context,
	model AsbEndpointModel,
	subscriptionRuleValue string,
) (*AsbSubscriptionRule, error) {
	tflog.Info(ctx, "Get the subscription rule "+subscriptionRuleValue)
	rule, err := w.Client.GetRule(
		ctx,
		model.TopicName,
		model.EndpointName,
		w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscriptionRuleValue),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if rule == nil {
		return nil, fmt.Errorf("rule not found")
	}

	return convertToAsbSubscriptionRule(rule.RuleProperties)
}

func (w *AsbClientWrapper) DeleteAsbSubscriptionRule(
	ctx context.Context,
	model AsbEndpointModel,
	subscription AsbSubscriptionModel,
) error {
	ruleName := w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscription.Filter)

	tflog.Info(ctx, "Deleting subscription rule "+ruleName)

	// Retry 3 times as the delete rule operation can fail with a 409 conflict error
	// if another operation is in progress
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Deleting subscription rule "+ruleName,
		func() error {
			_, err := w.Client.DeleteRule(
				ctx,
				model.TopicName,
				model.EndpointName,
				ruleName,
				nil,
			)

			return err
		})
}

func (w *AsbClientWrapper) UpdateAsbSubscriptionRule(
	ctx context.Context,
	model AsbEndpointModel,
	subscriptionModel AsbSubscriptionModel,
) error {
	tflog.Info(ctx, "Updating subscription rule value "+subscriptionModel.Filter+" and filter type "+subscriptionModel.FilterType)

	// Retry 3 times as the create rule operation can fail with a 400 error,
	// which is a transient error, if another operation is in progress
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Updating subscription rule "+subscriptionModel.Filter+" with filter type"+subscriptionModel.FilterType,
		func() error {
			_, err := w.Client.UpdateRule(
				ctx,
				model.TopicName,
				model.EndpointName,
				az.RuleProperties{
					Name:   w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscriptionModel.Filter),
					Filter: createSubscriptionRule(subscriptionModel.FilterType, subscriptionModel.Filter),
				},
			)

			return err
		})
}

func IsAsbSubscriptionRuleCorrect(asbRule AsbSubscriptionRule, subscrioption AsbSubscriptionModel) bool {
	switch subscrioption.FilterType {
	case "correlation":
		return asbRule.Filter == subscrioption.Filter
	case "sql":
		return asbRule.Filter == makeSubscriptionSqlRuleFilter(subscrioption.Filter).Expression
	default:
		tflog.Error(context.Background(), "Invalid subscription filter type: "+subscrioption.FilterType)
		return true
	}
}

func GetSubscriptionFilterValueForAsbRuleName(knownSubscriptionFilterValues []string, rule AsbSubscriptionRule) int {
	for index, subscriptionFilterValue := range knownSubscriptionFilterValues {
		if rule.Name == getRuleNameWithUniqueIdentifier(subscriptionFilterValue) {
			return index
		}
	}

	return -1
}

func (w *AsbClientWrapper) encodeAsbSubscriptionRuleNameFromFitlerValue(
	subscriptionFilterValue string,
) string {
	return getRuleNameWithUniqueIdentifier(subscriptionFilterValue)
}

func getRuleNameWithUniqueIdentifier(subscriptionFilterValue string) string {
	if len(subscriptionFilterValue) <= MAX_RULE_NAME_LENGTH {
		return subscriptionFilterValue
	}

	identifier := getUniqueSubscriptionIdentifier(subscriptionFilterValue)

	// We try to ensure that the rule name is unique, but still traceable to the subscription name
	ruleNameLength := MAX_RULE_NAME_LENGTH - len(identifier) - len(SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR)
	croppedSubscriptionName := cropFilterValueToLength(subscriptionFilterValue, ruleNameLength)
	return croppedSubscriptionName + SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR + identifier
}

func getUniqueSubscriptionIdentifier(subscriptionFilterValue string) string {
	hash := sha1.New()
	io.WriteString(hash, subscriptionFilterValue) // nolint: errcheck

	identifierHash := hash.Sum(nil)

	// Half the length of the hash should be enough to make it unique
	identifierHash = identifierHash[:SUBSCRIPTION_NAME_IDENTIFIER_LENGTH]
	return base64.RawURLEncoding.EncodeToString(identifierHash)
}

func cropFilterValueToLength(subscriptionFilterValue string, length int) string {
	if len(subscriptionFilterValue) < length {
		return subscriptionFilterValue
	}

	subscriptionFilterValue = strings.Trim(subscriptionFilterValue, " ")
	if len(subscriptionFilterValue) < length {
		return subscriptionFilterValue
	}

	return subscriptionFilterValue[len(subscriptionFilterValue)-length:]
}

func makeSubscriptionSqlRuleFilter(subscriptionFilterValue string) *az.SQLFilter {
	return &az.SQLFilter{
		Expression: "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionFilterValue + "%'",
	}
}

func makeSubscriptionCorrelationRuleFilter(subscriptionFilterValue string) *az.CorrelationFilter {
	return &az.CorrelationFilter{
		ApplicationProperties: map[string]interface{}{
			CORRELATIONFILTER_HEADER: subscriptionFilterValue,
		},
	}
}
