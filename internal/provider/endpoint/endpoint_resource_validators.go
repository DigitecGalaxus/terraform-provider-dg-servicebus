package endpoint

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func intOneOfValues(values []int64) intOneOfValidator {
	return intOneOfValidator{
		values: values,
	}
}

type intOneOfValidator struct {
	values []int64
}

func (v intOneOfValidator) Description(ctx context.Context) string {
	return fmt.Sprintf("Value must be one of the following: %v", v.values)
}

func (v intOneOfValidator) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("Value must be one of the following: %v", v.values)
}

func (v intOneOfValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	intValue := req.ConfigValue.ValueInt64()

	for _, value := range v.values {
		if value == intValue {
			return
		}
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid value",
		fmt.Sprintf("Value must be one of the following: %v", v.values),
	)
}