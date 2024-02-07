package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"terraform-provider-dg-servicebus/internal/provider/asb"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/stretchr/testify/assert"
	"github.com/xorcare/pointer"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const providerConfig = `
provider "dgservicebus" {
    azure_servicebus_hostname = "DG-PROD-Chabis-Messaging-Testing.servicebus.windows.net"
    tenant_id                 = "35aa8c5b-ac0a-4b15-9788-ff6dfa22901f"
}
`

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"dgservicebus": providerserver.NewProtocol6WithError(New("test")()),
}

func createClient(t *testing.T) asb.AsbClientWrapper {
	tenantId := "35aa8c5b-ac0a-4b15-9788-ff6dfa22901f"
	clientId := os.Getenv("DG_SERVICEBUS_CLIENTID")
	clientSecret := os.Getenv("DG_SERVICEBUS_CLIENTSECRET")

	var credential azcore.TokenCredential
	var err error

	if clientId != "" && clientSecret != "" {
		credential, err = azidentity.NewClientSecretCredential(tenantId, clientId, clientSecret, nil)
	} else {
		credential, err = azidentity.NewDefaultAzureCredential(nil)
	}
	assert.Nil(t, err, "No error expected")

	admin_client, err := azservicebus.NewClient("DG-PROD-Chabis-Messaging-Testing.servicebus.windows.net", credential, nil)
	assert.Nil(t, err, "No error expected")

	return asb.AsbClientWrapper{
		Client: admin_client,
	}
}

func TestAcc_TestResource(t *testing.T) {
	var uuid string = acctest.RandString(10)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "%v-test-endpoint"
					topic_name    = "bundle-1"
					subscriptions = [
						"Dg.Test.V1.Subscription"
					]
					additional_queues = [
						"%v-additional-queue"
					]

					queue_options = {
					  enable_partitioning   = true,
					  max_size_in_megabytes = 5120,
					  max_message_size_in_kilobytes = 5120
					}
				}`, uuid, uuid),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0", "Dg.Test.V1.Subscription"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "additional_queues.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "additional_queues.0", fmt.Sprintf("%v-additional-queue", uuid)),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_exists", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", fmt.Sprintf("%v-test-endpoint", uuid)),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_exists", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_create_endpoint", "false"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_create_queue", "false"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_update_subscriptions", "false"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "topic_name", "bundle-1"),
				),
			},
		},
	})
}

func TestAcc_TestData(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
				data "dgservicebus_endpoint" "test" {
					endpoint_name = "test-queue"
					topic_name	= "bundle-1"
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0", "Dg.Test.V1.Subscription"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", "test-queue"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "5120"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
				),
			},
		},
	})
}

func TestAcc_ResourceTakeover(t *testing.T) {
	var queueTakeoverEnpointName string = acctest.RandString(10) + "-queue-no-subscription"
	var additionalQueueTakeoverEndpointName string = acctest.RandString(10) + "-additional-queue-enpoint"
	var additionalQueueName string = acctest.RandString(10) + "-additional-queue"

	// create needed queue to take over
	var ctx context.Context = context.Background()
	var client = createClient(t)
	var err = client.CreateEndpointQueue(ctx, queueTakeoverEnpointName, asb.EndpointQueueOptions{
		EnablePartitioning:        pointer.Bool(true),
		MaxSizeInMegabytes:        pointer.Int32(int32(5120)),
		MaxMessageSizeInKilobytes: pointer.Int64(int64(5120)),
	})
	assert.Nil(t, err, "Could not create queue "+queueTakeoverEnpointName)

	// Create additianl queue
	err = client.CreateEndpointQueue(ctx, additionalQueueName, asb.EndpointQueueOptions{
		EnablePartitioning:        pointer.Bool(true),
		MaxSizeInMegabytes:        pointer.Int32(int32(5120)),
		MaxMessageSizeInKilobytes: pointer.Int64(int64(5120)),
	})
	assert.Nil(t, err, "Could not create queue "+additionalQueueName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Take over existing queue
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "queue-takeover" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscriptions = []

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 5120
					}
				}
				`, queueTakeoverEnpointName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "subscriptions.#", "0"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "endpoint_name", queueTakeoverEnpointName),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "topic_name", "bundle-1"),
				),
			},
			// Take over existing additional queue
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "additional-queue-takeover" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscriptions = []
					additional_queues = [
						"%v"
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 5120
					}
				}
				`, additionalQueueTakeoverEndpointName, additionalQueueName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "subscriptions.#", "0"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "endpoint_name", additionalQueueTakeoverEndpointName),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "additional_queues.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "additional_queues.0", additionalQueueName),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "queue_options.max_message_size_in_kilobytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "topic_name", "bundle-1"),
				),
			},
			// Already existing subscription
			{
				Config: providerConfig + `
				resource "dgservicebus_endpoint" "subscription-overtake" {
					endpoint_name = "test-subscription-no-queue"
					topic_name	= "bundle-1"
					subscriptions = [
						"Dg.Test.V1.Subscription"
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 5120
					}
				}
				`,
				ExpectError: regexp.MustCompile(`.*(Cannot create endpoint|Subscription test-subscription-no-queue already existis on topic bundle-1 for| endpoint test-subscription-no-queue).*`),
			},
		},
	})
}
