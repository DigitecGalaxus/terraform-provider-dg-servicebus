package asb

import (
	az "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

type AsbClientWrapper struct {
	Client *az.Client
}
