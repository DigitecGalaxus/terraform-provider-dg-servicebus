package endpoint

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func isValidCorrelationFilter() SubscriptionFilterValidator {
	return SubscriptionFilterValidator{}
}

type SubscriptionFilterValidator struct {
}

func (v SubscriptionFilterValidator) Description(ctx context.Context) string {
	return "Check if the subscriptions values are well formated filter for the defined subscription filter type."
}

func (v SubscriptionFilterValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v SubscriptionFilterValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// If the value is unknown or null, there is nothing to validate.
	var config endpointResourceModel
	req.Config.Get(ctx, &config)

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	stringValue := req.ConfigValue.ValueString()

	switch config.SubscriptionFilterType.ValueString() {
	case "sql":
		if validate_sql_filter(stringValue) {
			return
		}
		resp.Diagnostics.AddAttributeError(
			req.Path,
			fmt.Sprintf("Invalid sql filter value %v", stringValue),
			"Value must be an fully qualified name of the endpoint. Example: 'MyNamespace.MyClass'",
		)
	case "correlation":
		if validate_correlation_filter(stringValue) {
			return
		}
		resp.Diagnostics.AddAttributeError(
			req.Path,
			fmt.Sprintf("Invalid correlation filter value %v", stringValue),
			"Value must be an assembly qualified name of the endpoint. Example: 'MyNamespace.MyClass, MyAssembly, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null'",
		)
	}
}

func validate_sql_filter(stringValue string) bool {
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`).MatchString(stringValue)
}

func validate_correlation_filter(stringValue string) bool {
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\.]*, [a-zA-Z_][a-zA-Z0-9_\.]*, Version=1\.0\.0\.0, Culture=neutral, PublicKeyToken=null$`).MatchString(stringValue)
}
