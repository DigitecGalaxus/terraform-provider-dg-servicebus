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

func TestAcc_TestEndpointCompleteEndpoint(t *testing.T) {
	sqlFilterCases := map[string]string{
		"sql":         "Dg.Test.V1.Subscription",
		"correlation": "Dg.Test.V1.Subscription, Dg.Test.V1, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null",
	}

	for filterType, filterValue := range sqlFilterCases {
		var uuid string = acctest.RandString(10)

		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: providerConfig + fmt.Sprintf(`
					resource "dgservicebus_endpoint" "test" {
						endpoint_name = "%v-test-endpoint"
						topic_name    = "bundle-1"
						subscription_filter_type = "%v"
						subscriptions = [
							"%v"
						]
						additional_queues = [
							"%v-additional-queue"
						]

						queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
						}
					}`, uuid, filterType, filterValue, uuid),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "1"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0", filterValue),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscription_filter_type", filterType),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "additional_queues.#", "1"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "additional_queues.0", fmt.Sprintf("%v-additional-queue", uuid)),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_exists", "true"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", fmt.Sprintf("%v-test-endpoint", uuid)),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_exists", "true"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "5120"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_create_endpoint", "false"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_create_queue", "false"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_update_subscriptions", "false"),
						resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "topic_name", "bundle-1"),
					),
				},
			},
		})
	}
}

func TestAcc_EndpointTakeover(t *testing.T) {
	var ctx context.Context = context.Background()
	var client = createClient(t)

	var queueTakeoverEnpointName string = acctest.RandString(10) + "-takeover-queue-no-subscription"
	var additionalQueueTakeoverEndpointName string = acctest.RandString(10) + "-takeover-additional-queue-enpoint"
	var additionalQueueName string = acctest.RandString(10) + "-takeover-additional-queue"
	var endpoint_with_no_queue, _ = create_test_endpoint(client, 1, "sql", false)

	// create needed queue to take over
	var err = client.CreateEndpointQueue(ctx, queueTakeoverEnpointName, asb.AsbEndpointQueueOptions{
		EnablePartitioning:        pointer.Bool(true),
		MaxSizeInMegabytes:        pointer.Int32(int32(5120)),
		MaxMessageSizeInKilobytes: pointer.Int64(int64(256)),
	})
	assert.Nil(t, err, "Could not create queue "+queueTakeoverEnpointName)

	// Create additional queue
	err = client.CreateEndpointQueue(ctx, additionalQueueName, asb.AsbEndpointQueueOptions{
		EnablePartitioning:        pointer.Bool(true),
		MaxSizeInMegabytes:        pointer.Int32(int32(5120)),
		MaxMessageSizeInKilobytes: pointer.Int64(int64(256)),
	})
	assert.Nil(t, err, "Could not create queue "+additionalQueueName)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Take over existing queue
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "queue-takeover" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscription_filter_type = "sql"
					subscriptions = []

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, queueTakeoverEnpointName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "subscriptions.#", "0"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "endpoint_name", queueTakeoverEnpointName),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "queue_options.max_message_size_in_kilobytes", "256"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.queue-takeover", "topic_name", "bundle-1"),
				),
			},
			// Take over existing additional queue
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "additional-queue-takeover" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscription_filter_type = "sql"
					subscriptions = []
					additional_queues = [
						"%v"
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
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
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "queue_options.max_message_size_in_kilobytes", "256"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.additional-queue-takeover", "topic_name", "bundle-1"),
				),
			},
			// Already existing subscription
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "subscription-overtake" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscription_filter_type = "sql"
					subscriptions = [
						"Dg.Test.Subscription.V1"
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, endpoint_with_no_queue),
				ExpectError: regexp.MustCompile(`.*(Cannot create endpoint|Subscription test-subscription-no-queue already existis on topic bundle-1 for| endpoint test-subscription-no-queue).*`),
			},
		},
	})
	ensure_enpoint_deleted(client, endpoint_with_no_queue)
}

func TestAcc_EndpointDataSourceTest(t *testing.T) {
	var client = createClient(t)

	endpoint_sql_no_subscription_name, _ := create_test_endpoint(client, 0, "sql", true)
	endpoint_correlation_no_subscription_name, _ := create_test_endpoint(client, 0, "correlation", true)
	endpoint_sql_with_subcription_name, sql_subscriptions := create_test_endpoint(client, 1, "sql", true)
	endpoint_correlation_with_subcription_name, correlation_subscriptions := create_test_endpoint(client, 1, "correlation", true)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Test with no subscription
			{
				Config: providerConfig + fmt.Sprintf(`
				data "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
				}
				`, endpoint_sql_no_subscription_name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.#", "0"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscription_filter_type", ""),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", endpoint_sql_no_subscription_name),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
				data "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
				}
				`, endpoint_correlation_no_subscription_name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.#", "0"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscription_filter_type", ""),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", endpoint_correlation_no_subscription_name),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
				),
			},
			// Test with subscription
			{
				Config: providerConfig + fmt.Sprintf(`
				data "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
				}
				`, endpoint_sql_with_subcription_name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0", sql_subscriptions[0]),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscription_filter_type", "sql"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", endpoint_sql_with_subcription_name),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
				data "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
				}
				`, endpoint_correlation_with_subcription_name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0", correlation_subscriptions[0]),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscription_filter_type", "correlation"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", endpoint_correlation_with_subcription_name),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
				),
			},
		},
	})

	ensure_enpoint_deleted(client, endpoint_sql_no_subscription_name)
	ensure_enpoint_deleted(client, endpoint_correlation_no_subscription_name)
	ensure_enpoint_deleted(client, endpoint_sql_with_subcription_name)
	ensure_enpoint_deleted(client, endpoint_correlation_with_subcription_name)
}

// Helper functions.
func create_test_endpoint(client asb.AsbClientWrapper, number_of_subscription uint, subscription_filter_type string, create_queue bool) (endpoint_name string, subscriptions []string) {
	uuid := acctest.RandString(10)
	var ctx context.Context = context.Background()

	endpoint_name = uuid + "-test-endpoint"
	subscriptions = make([]string, number_of_subscription)

	queueOptions := asb.AsbEndpointQueueOptions{
		EnablePartitioning:        pointer.Bool(true),
		MaxSizeInMegabytes:        pointer.Int32(int32(5120)),
		MaxMessageSizeInKilobytes: pointer.Int64(int64(256)),
	}

	model := asb.AsbEndpointModel{
		EndpointName:           endpoint_name,
		TopicName:              "bundle-1",
		Subscriptions:          subscriptions,
		SubscriptionFilterType: subscription_filter_type,
		QueueOptions:           queueOptions,
	}
	if create_queue {
		// Create queue
		err := client.CreateEndpointQueue(ctx, endpoint_name, queueOptions)
		if err != nil {
			panic(err)
		}
	}

	// Create subscription in bundle-1
	err := client.CreateEndpointWithDefaultRuleWithOptinalForwading(ctx, model, !create_queue)
	if err != nil {
		panic(err)
	}

	// Add subscription rule
	for i := range subscriptions {
		if subscription_filter_type == "sql" {
			subscriptions[i] = fmt.Sprintf("Dg.Test.Subscription.V%v", i)
		} else {
			subscriptions[i] = fmt.Sprintf("Dg.Test.Subscription.V%v, Dg.Test.V1, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null", i)
		}
		err = client.CreateAsbSubscriptionRule(ctx, model, subscriptions[i], subscription_filter_type)
		if err != nil {
			panic(err)
		}
	}

	return
}

func ensure_enpoint_deleted(client asb.AsbClientWrapper, endpoint_name string) {
	ctx := context.Background()
	model := asb.AsbEndpointModel{
		EndpointName: endpoint_name,
		TopicName:    "bundle-1",
	}
	_ = client.DeleteEndpoint(ctx, model)
	_ = client.DeleteEndpointQueue(ctx, model)
}
