package asb

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"math"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

type Subscription struct {
	Name   string
	Filter string
}

const MAX_RULE_NAME_LENGTH = 50
const SHA_1_BYTE_LENGTH = 20
const SUBSCRIPTION_NAME_IDENTIFIER_LENGTH = SHA_1_BYTE_LENGTH / 2
const SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR = "--"

func (w *AsbClientWrapper) GetEndpointSubscriptions(
	ctx context.Context,
	model EndpointModel,
) ([]Subscription, error) {
	subscriptions := []Subscription{}
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

			subscriptions = append(subscriptions, subscription)
		}
	}

	return subscriptions, nil
}

func (w *AsbClientWrapper) CreateEndpointSubscription(
	ctx context.Context,
	model EndpointModel,
	subscriptionName string,
) error {

	// Retry 3 times as the create rule operation can fail with a 400 error,
	// which is a transient error, if another operation is in progress
	return runWithRetryIncrementalBackOff(
		ctx,
		"Creating subscription rule "+subscriptionName,
		func() error {
		_, err := w.Client.CreateRule(
			ctx,
			model.TopicName,
			model.EndpointName,
			&az.CreateRuleOptions{
				Name: to.Ptr(w.encodeSubscriptionRuleName(ctx, model, subscriptionName)),
				Filter: &az.SQLFilter{
					Expression: makeSubscriptionFilter(subscriptionName),
				},
			},
		)

		return err
	})
}

func (w *AsbClientWrapper) EndpointSubscriptionExists(
	ctx context.Context,
	model EndpointModel,
	subscriptionName string,
) bool {
	rule, err := w.Client.GetRule(
		ctx,
		model.TopicName,
		model.EndpointName,
		w.encodeSubscriptionRuleName(ctx, model, subscriptionName),
		nil,
	)

	return err != nil && rule != nil
}

func (w *AsbClientWrapper) getEndpointSubscriptionRaw(
	ctx context.Context,
	model EndpointModel,
	subscriptionName string,
) (*az.GetRuleResponse, error) {
	rule, err := w.Client.GetRule(
		ctx,
		model.TopicName,
		model.EndpointName,
		subscriptionName,
		nil,
	)

	return rule, err
}

func (w *AsbClientWrapper) DeleteEndpointSubscription(
	ctx context.Context,
	model EndpointModel,
	subscriptionName string,
) error {
	ruleName := w.encodeSubscriptionRuleName(ctx, model, subscriptionName)

	tflog.Info(ctx, "Deleting subscription rule "+ruleName)
	
	// Retry 3 times as the delete rule operation can fail with a 409 conflict error
	// if another operation is in progress
	return runWithRetryIncrementalBackOff(
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

func (w *AsbClientWrapper) EnsureEndpointSubscriptionFilterCorrect(
	ctx context.Context,
	model EndpointModel,
	subscriptionName string,
) error {
	subscriptionFilter := makeSubscriptionFilter(subscriptionName)

	tflog.Info(ctx, "Updating subscription rule "+subscriptionName+" with filter "+subscriptionFilter)

	// Retry 3 times as the create rule operation can fail with a 400 error,
	// which is a transient error, if another operation is in progress
	return runWithRetryIncrementalBackOff(
		ctx,
		"Updating subscription rule "+subscriptionName+" with filter "+subscriptionFilter,
		func() error {
		_, err := w.Client.UpdateRule(
			ctx,
			model.TopicName,
			model.EndpointName,
			az.RuleProperties{
				Name: w.encodeSubscriptionRuleName(ctx, model, subscriptionName),
				Filter: &az.SQLFilter{
					Expression: subscriptionFilter,
				},
			},
		)

		return err
	})
}

func runWithRetryIncrementalBackOff(
	ctx context.Context,
	actionMessage string,
	fun func() error,
) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun()
		if err == nil {
			return nil
		}

		tflog.Info(ctx, actionMessage+" failed with error "+err.Error()+", retrying")

		backOff := time.Second * time.Duration(math.Pow(2, float64(i)))
		time.Sleep(backOff)
	}

	return err
}

func IsSubscriptionFilterCorrect(filter string, subscriptionName string) bool {
	return filter == makeSubscriptionFilter(subscriptionName)
}

func TryGetFullSubscriptionNameFromRuleName(knownSubscriptionNames []string, ruleName string) *string {
	for _, subscription := range knownSubscriptionNames {
		if ruleName == getRuleNameWithUniqueIdentifier(subscription) {
			return &subscription
		}
	}

	return nil
}

func (w *AsbClientWrapper) encodeSubscriptionRuleName(
	ctx context.Context,
	model EndpointModel,
	subscriptionName string,
) string {
	if len(subscriptionName) < MAX_RULE_NAME_LENGTH {
		return subscriptionName
	}

	existingSubscription, err := w.getEndpointSubscriptionRaw(ctx, model, subscriptionName)
	if err == nil || existingSubscription != nil {
		return subscriptionName
	}

	return getRuleNameWithUniqueIdentifier(subscriptionName)
}

func getRuleNameWithUniqueIdentifier(subscriptionName string) string {
	if len(subscriptionName) < MAX_RULE_NAME_LENGTH {
		return subscriptionName
	}

	identifier := getUniqueSubscriptionIdentifier(subscriptionName)

	// We try to ensure that the rule name is unique, but still traceable to the subscription name
	ruleNameLength := MAX_RULE_NAME_LENGTH - len(identifier) - len(SUBSCRIPTION_NAME_IDENTIFIER_SEPARATOR)
	croppedSubscriptionName := cropStringToLength(subscriptionName, ruleNameLength)
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

func makeSubscriptionFilter(subscriptionName string) string {
	return "[NServiceBus.EnclosedMessageTypes] LIKE '%" + subscriptionName + "%'"
}
