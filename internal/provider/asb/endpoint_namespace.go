package asb

import (
	"context"
)

func (w *AsbClientWrapper) GetFullyQualifiedName(azureContext context.Context, entityName string) (string, error) {
	response, err := w.Client.GetNamespaceProperties(azureContext, nil)
	if err != nil {
		return "", err
	}

	return "sb://" + response.Name + ".servicebus.windows.net/" + entityName, nil
}
