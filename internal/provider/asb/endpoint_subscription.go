package asb

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

type AsbSubscriptionRule struct {
	Name       string
	Filter     string
	FilterType string
}

const MAX_RULE_NAME_LENGTH = 50
const SHA_1_BYTE_LENGTH = 20
const SUBSCRIPTION_NAME_IDENTIFIER_LENGTH = SHA_1_BYTE_LENGTH / 2
const SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR = "--"

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

			switch rule.Filter.(type) {
			case *az.CorrelationFilter:
				correlationFilter, ok := rule.Filter.(*az.CorrelationFilter)
				if !ok {
					continue
				}
				filter, ok := correlationFilter.ApplicationProperties["NServiceBus.EnclosedMessageTypes"].(string)
				if !ok {
					continue
				}
				subscription := AsbSubscriptionRule{
					Name:       rule.Name,
					Filter:     filter,
					FilterType: "correlation",
				}
				subscriptions = append(subscriptions, subscription)
			case *az.SQLFilter:
				sqlFilter, ok := rule.Filter.(*az.SQLFilter)
				if !ok {
					continue
				}
				subscription := AsbSubscriptionRule{
					Name:       rule.Name,
					Filter:     sqlFilter.Expression,
					FilterType: "sql",
				}

				subscriptions = append(subscriptions, subscription)
			default:
				continue
			}
		}
	}

	return subscriptions, nil
}

func (w *AsbClientWrapper) CreateAsbSubscriptionRule(
	ctx context.Context,
	model AsbEndpointModel,
	subscriptionFilterValue string,
	subscriptionFilterType string,
) error {

	// Retry 3 times as the create rule operation can fail with a 400 error,
	// which is a transient error, if another operation is in progress
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Creating subscription rule "+subscriptionFilterValue,
		func() error {
			_, err := w.Client.CreateRule(
				ctx,
				model.TopicName,
				model.EndpointName,
				&az.CreateRuleOptions{
					Name:   to.Ptr(w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscriptionFilterValue)),
					Filter: createSubscriptionRule(subscriptionFilterType, subscriptionFilterValue),
				},
			)

			return err
		})
}

func createSubscriptionRule(subscriptionFilterType, subscriptionFilterValue string) az.RuleFilter {
	switch subscriptionFilterType {
	case "correlation":
		return &az.CorrelationFilter{
			ApplicationProperties: map[string]interface{}{
				"NServiceBus.EnclosedMessageTypes": subscriptionFilterValue,
			},
		}
	case "sql":
		return &az.SQLFilter{
			Expression: makeSubscriptionSqlRuleFilter(subscriptionFilterValue),
		}
	default:
		tflog.Error(context.Background(), "Invalid subscription filter type: "+subscriptionFilterType)
		return nil
	}
}

func (w *AsbClientWrapper) AsbSubscriptionRuleExists(
	ctx context.Context,
	model AsbEndpointModel,
	subscriptionRuleValue string,
) bool {
	tflog.Info(ctx, "Checking if subscription rule "+subscriptionRuleValue+" exists")
	rule, err := w.Client.GetRule(
		ctx,
		model.TopicName,
		model.EndpointName,
		w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscriptionRuleValue),
		nil,
	)

	if err != nil {
		return false
	}

	return rule != nil
}

func (w *AsbClientWrapper) DeleteAsbSubscriptionRule(
	ctx context.Context,
	model AsbEndpointModel,
	subscriptionFilterValue string,
) error {
	ruleName := w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscriptionFilterValue)

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

func (w *AsbClientWrapper) EnsureAsbSubscriptionRuleIsCorrect(
	ctx context.Context,
	model AsbEndpointModel,
	subscriptionFilterValue string,
	subscriptionFilterType string,
) error {
	tflog.Info(ctx, "Ensuring subscription rule value "+subscriptionFilterValue+" for filter type "+subscriptionFilterType+" is correct")

	// Retry 3 times as the create rule operation can fail with a 400 error,
	// which is a transient error, if another operation is in progress
	return runWithRetryIncrementalBackOffVoid(
		ctx,
		"Updating subscription rule "+subscriptionFilterValue+" with filter type"+subscriptionFilterType,
		func() error {
			_, err := w.Client.UpdateRule(
				ctx,
				model.TopicName,
				model.EndpointName,
				az.RuleProperties{
					Name:   w.encodeAsbSubscriptionRuleNameFromFitlerValue(subscriptionFilterValue),
					Filter: createSubscriptionRule(subscriptionFilterType, subscriptionFilterValue),
				},
			)

			return err
		})
}

func IsAsbSubscriptionRuleCorrect(currentFilter string, subscriptionFilterValue string, subscriptionFilterType string) bool {
	switch subscriptionFilterType {
	case "correlation":
		return currentFilter == subscriptionFilterValue
	case "sql":
		return currentFilter == makeSubscriptionSqlRuleFilter(subscriptionFilterValue)
	default:
		tflog.Error(context.Background(), "Invalid subscription filter type: "+subscriptionFilterType)
		return true
	}
}

func GetSubscriptionFilterValueForAsbRuleName(knownSubscriptionFilterValues []string, ruleName string) *string {
	for _, subscriptionFilterValue := range knownSubscriptionFilterValues {
		if ruleName == getRuleNameWithUniqueIdentifier(subscriptionFilterValue) {
			return &subscriptionFilterValue
		}
	}

	return nil
}

func (w *AsbClientWrapper) encodeAsbSubscriptionRuleNameFromFitlerValue(
	subscriptionName string,
) string {
	return getRuleNameWithUniqueIdentifier(subscriptionName)
}

func getRuleNameWithUniqueIdentifier(subscriptionName string) string {
	replacer := strings.NewReplacer(",", "", " ", "_", "=", "_")
	validSubscriptionName := replacer.Replace(subscriptionName)
	if len(validSubscriptionName) <= MAX_RULE_NAME_LENGTH {
		return validSubscriptionName
	}

	identifier := getUniqueSubscriptionIdentifier(validSubscriptionName)

	// We try to ensure that the rule name is unique, but still traceable to the subscription name
	ruleNameLength := MAX_RULE_NAME_LENGTH - len(identifier) - len(SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR)
	croppedSubscriptionName := cropStringToLength(validSubscriptionName, ruleNameLength)
	return croppedSubscriptionName + SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR + identifier
}

func getUniqueSubscriptionIdentifier(subscriptionName string) string {
	hash := sha1.New()
	io.WriteString(hash, subscriptionName) // nolint: errcheck

	identifierHash := hash.Sum(nil)

	// Half the length of the hash should be enough to make it unique
	identifierHash = identifierHash[:SUBSCRIPTION_NAME_IDENTIFIER_LENGTH]
	return base64.RawURLEncoding.EncodeToString(identifierHash)
}

func cropStringToLength(subscriptionName string, length int) string {
	if len(subscriptionName) < length {
		return subscriptionName
	}

	subscriptionName = strings.Trim(subscriptionName, " ")
	if len(subscriptionName) < length {
		return subscriptionName
	}

	return subscriptionName[len(subscriptionName)-length:]
}

func makeSubscriptionSqlRuleFilter(subscriptionFilterValue string) string {
	return "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionFilterValue + "%'"
}
