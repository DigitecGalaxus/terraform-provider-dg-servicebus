package asb

import (
	"context"
)

func (w *AsbClientWrapper) GetFullyQualifiedName(ctx context.Context, entityName string) (string, error) {
	return runWithRetryIncrementalBackOff(
		ctx,
		"Getting namespace properties",
		func() (string, error) {
			response, err := w.Client.GetNamespaceProperties(ctx, nil)
			
			if err != nil {
				return "", err
			}

			return "sb://" + response.Name + ".servicebus.windows.net/" + entityName, nil
		},
	)
}
