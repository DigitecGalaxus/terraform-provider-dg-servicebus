package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestAcc_TestCreateEndpoint(t *testing.T) {
	// Init test test resources
	var uuid string = acctest.RandString(10)
	sqlFilterCases := map[string]string{
		"sql":         "Dg.Test.V1.Subscription",
		"correlation": "Dg.Test.V1.Subscription",
	}
	testSteps := []resource.TestStep{}

	for filterType, filterValue := range sqlFilterCases {
		endpoint_name := fmt.Sprintf("%v-%v-test-create-endpoint", uuid, filterType)

		testSteps = append(testSteps, resource.TestStep{
			Config: providerConfig + fmt.Sprintf(`
					resource "dgservicebus_endpoint" "test" {
						endpoint_name = "%v"
						topic_name    = "bundle-1"
						subscriptions = [
							{filter = "%v", filter_type = "%v"}
						]

						queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
						}
					}`, endpoint_name, filterValue, filterType),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "1"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter", filterValue),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter_type", filterType),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_exists", "true"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", endpoint_name),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_exists", "true"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "5120"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_create_endpoint", "false"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_create_queue", "false"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "should_update_subscriptions", "false"),
				resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "topic_name", "bundle-1"),
			),
		})

	}

	// Run tests
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    testSteps,
	})

	// Clean up test resources
	var client = createClient(t)
	for filterType := range sqlFilterCases {
		endpoint_name := fmt.Sprintf("%v-%v-test-create-endpoint", uuid, filterType)

		ensure_enpoint_deleted(client, endpoint_name)
	}
}

func TestAcc_EndpointTakeover(t *testing.T) {
	// Init test test resources
	var client = createClient(t)
	var ctx context.Context = context.Background()

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

	// Run tests
	resource.ParallelTest(t, resource.TestCase{
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
					subscriptions = [
						{filter: "Dg.Test.Subscription.V1", filter_type: "sql"}
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

	// Clean up test resources
	ensure_enpoint_deleted(client, endpoint_with_no_queue)
	ensure_enpoint_deleted(client, queueTakeoverEnpointName)
	ensure_enpoint_deleted(client, additionalQueueTakeoverEndpointName)
}

func TestAcc_EndpointDataSourceTest(t *testing.T) {

	// Init test test resources
	var client = createClient(t)

	endpoint_sql_no_subscription_name, _ := create_test_endpoint(client, 0, "sql", true)
	endpoint_correlation_no_subscription_name, _ := create_test_endpoint(client, 0, "correlation", true)
	endpoint_sql_with_subcription_name, sql_subscriptions := create_test_endpoint(client, 1, "sql", true)
	endpoint_correlation_with_subcription_name, correlation_subscriptions := create_test_endpoint(client, 1, "correlation", true)

	// Run tests
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
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0.filter", sql_subscriptions[0].Filter),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0.filter_type", sql_subscriptions[0].FilterType),
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
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0.filter", correlation_subscriptions[0].Filter),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0.filter_type", correlation_subscriptions[0].FilterType),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", endpoint_correlation_with_subcription_name),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_message_size_in_kilobytes", "256"),
				),
			},
		},
	})

	// Clean up
	ensure_enpoint_deleted(client, endpoint_sql_no_subscription_name)
	ensure_enpoint_deleted(client, endpoint_correlation_no_subscription_name)
	ensure_enpoint_deleted(client, endpoint_sql_with_subcription_name)
	ensure_enpoint_deleted(client, endpoint_correlation_with_subcription_name)
}

func TestAcc_EndpointSubscriptionOrderChanges(t *testing.T) {
	client := createClient(t)

	endpoint_name := acctest.RandString(10) + "-test-ordes-endpoint"
	subscriptionFilterValue1 := "Dg.Test.Subscription.V1"
	subscriptionFilterValue2 := "Dg.Test.Subscription.V2"
	subscriptionFilterValue3 := "Dg.Test.Subscription.V3"
	subscriptionFilterValue4 := "Dg.Test.Subscription.V4"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			//Create resource with 3 subscriptions
			{
				Config: providerConfig + fmt.Sprintf(`
					resource "dgservicebus_endpoint" "test" {
						endpoint_name = "%v"
						topic_name    = "bundle-1"
						subscriptions = [
							{filter = "%v", filter_type = "sql"},
							{filter = "%v", filter_type = "sql"},
							{filter = "%v", filter_type = "sql"}
						]

						queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
						}
					}`, endpoint_name, subscriptionFilterValue1, subscriptionFilterValue2, subscriptionFilterValue3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "3"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter", subscriptionFilterValue1),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.1.filter", subscriptionFilterValue2),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.2.filter", subscriptionFilterValue3),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", endpoint_name),
				),
			},
			// Change order of subscriptions
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name    = "bundle-1"
					subscriptions = [
						{filter = "%v", filter_type = "sql"},
						{filter = "%v", filter_type = "sql"},
						{filter = "%v", filter_type = "sql"}
					]

					queue_options = {
					enable_partitioning   = true,
					max_size_in_megabytes = 5120,
					max_message_size_in_kilobytes = 256
					}
				}`, endpoint_name, subscriptionFilterValue3, subscriptionFilterValue1, subscriptionFilterValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "3"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter", subscriptionFilterValue1),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.1.filter", subscriptionFilterValue2),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.2.filter", subscriptionFilterValue3),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", endpoint_name),
					func(s *terraform.State) error {
						subscriptions, _ := client.GetAsbSubscriptionsRules(context.Background(), asb.AsbEndpointModel{
							EndpointName: endpoint_name,
							TopicName:    "bundle-1",
						})
						if len(subscriptions) != 3 {
							return fmt.Errorf("Expected 3 subscriptions")
						}
						contains := func(subscriptions []asb.AsbSubscriptionRule, filter string) bool {
							for _, subscription := range subscriptions {
								if strings.Contains(subscription.Filter, filter) {
									return true
								}
							}
							return false
						}

						if !contains(subscriptions, subscriptionFilterValue1) || !contains(subscriptions, subscriptionFilterValue2) || !contains(subscriptions, subscriptionFilterValue3) {
							return fmt.Errorf("Expected subscriptions to contain %v, %v, %v", subscriptionFilterValue1, subscriptionFilterValue2, subscriptionFilterValue3)
						}

						return nil
					},
				),
			},
			// Add new subscription, remove one
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name    = "bundle-1"
					subscriptions = [
						{filter = "%v", filter_type = "sql"},
						{filter = "%v", filter_type = "sql"},
						{filter = "%v", filter_type = "sql"}
					]

					queue_options = {
					enable_partitioning   = true,
					max_size_in_megabytes = 5120,
					max_message_size_in_kilobytes = 256
					}
				}`, endpoint_name, subscriptionFilterValue3, subscriptionFilterValue2, subscriptionFilterValue4),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "3"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter", subscriptionFilterValue2),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.1.filter", subscriptionFilterValue3),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.2.filter", subscriptionFilterValue4),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", endpoint_name),
					func(s *terraform.State) error {
						subscriptions, _ := client.GetAsbSubscriptionsRules(context.Background(), asb.AsbEndpointModel{
							EndpointName: endpoint_name,
							TopicName:    "bundle-1",
						})
						if len(subscriptions) != 3 {
							return fmt.Errorf("Expected 3 subscriptions")
						}
						contains := func(subscriptions []asb.AsbSubscriptionRule, filter string) bool {
							for _, subscription := range subscriptions {
								if strings.Contains(subscription.Filter, filter) {
									return true
								}
							}
							return false
						}

						if !contains(subscriptions, subscriptionFilterValue2) || !contains(subscriptions, subscriptionFilterValue3) || !contains(subscriptions, subscriptionFilterValue4) {
							return fmt.Errorf("Expected subscriptions to contain %v, %v, %v", subscriptionFilterValue2, subscriptionFilterValue3, subscriptionFilterValue4)
						}

						return nil
					},
				),
			},
		},
	})

	ensure_enpoint_deleted(client, endpoint_name)
}

func TestAcc_EndpointSqlCorrelationUpdate(t *testing.T) {
	// Init test test resources
	var client = createClient(t)
	uuid := acctest.RandString(10)

	endpoint_name := fmt.Sprintf("%v-test-sql2correlation-endpoint", uuid)
	subscriptionFilterValue := "Dg.Test.Subscription.V1"
	subscriptionFilterValueChanged := "Dg.Test.Subscription.V2"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Test with no subscription
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "sql2correlation" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscriptions = [
						{filter: "%v", filter_type: "sql"}
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, endpoint_name, subscriptionFilterValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.0.filter", subscriptionFilterValue),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.0.filter_type", "sql"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "endpoint_name", endpoint_name),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.max_message_size_in_kilobytes", "256"),
					func(s *terraform.State) error {
						subscriptions, _ := client.GetAsbSubscriptionsRules(context.Background(), asb.AsbEndpointModel{
							EndpointName: endpoint_name,
							TopicName:    "bundle-1",
						})
						if len(subscriptions) != 1 {
							return fmt.Errorf("Expected 1 subscription")
						}

						if subscriptions[0].FilterType != "sql" {
							return fmt.Errorf("Expected sql filter type")
						}

						return nil
					},
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "sql2correlation" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscriptions = [
						{filter: "%v.Test", filter_type: "correlation"}
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, endpoint_name, subscriptionFilterValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.0.filter", fmt.Sprintf("%v.Test", subscriptionFilterValue)),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.0.filter_type", "correlation"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "endpoint_name", endpoint_name),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.max_message_size_in_kilobytes", "256"),
					func(s *terraform.State) error {
						subscriptions, _ := client.GetAsbSubscriptionsRules(context.Background(), asb.AsbEndpointModel{
							EndpointName: endpoint_name,
							TopicName:    "bundle-1",
						})
						if len(subscriptions) != 1 {
							return fmt.Errorf("Expected 1 subscription")
						}

						if subscriptions[0].FilterType != "correlation" {
							return fmt.Errorf("Expected correlation filter type")
						}

						return nil
					},
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "sql2correlation" {
					endpoint_name = "%v"
					topic_name	= "bundle-1"
					subscriptions = [
						{filter: "%v", filter_type: "sql"}
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, endpoint_name, subscriptionFilterValueChanged),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.0.filter", subscriptionFilterValueChanged),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "subscriptions.0.filter_type", "sql"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "endpoint_name", endpoint_name),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "topic_name", "bundle-1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.max_size_in_megabytes", "5120"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.sql2correlation", "queue_options.max_message_size_in_kilobytes", "256"),
					func(s *terraform.State) error {
						subscriptions, _ := client.GetAsbSubscriptionsRules(context.Background(), asb.AsbEndpointModel{
							EndpointName: endpoint_name,
							TopicName:    "bundle-1",
						})
						if len(subscriptions) != 1 {
							return fmt.Errorf("Expected 1 subscription")
						}

						if subscriptions[0].FilterType != "sql" {
							return fmt.Errorf("Expected sql filter type")
						}

						return nil
					},
				),
			},
		},
	})

	ensure_enpoint_deleted(client, endpoint_name)
}

func TestAcc_EndpointStateUpgrader(t *testing.T) {
	endpoint_name := acctest.RandString(10) + "-test-endpoint"

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"dgservicebus": {
						VersionConstraint: "0.0.16", // last version of old schema version
						Source:            "DigitecGalaxus/dg-servicebus",
					},
				},
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name    = "bundle-1"
					subscriptions = [
					  "Dg.Test.SubscriptionStateUpgrader.V1"
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, endpoint_name),
				Destroy: false,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0", "Dg.Test.SubscriptionStateUpgrader.V1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", endpoint_name),
				),
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Destroy:                  false,
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "%v"
					topic_name    = "bundle-1"
					subscriptions = [
					  { filter: "Dg.Test.SubscriptionStateUpgrader.V1", filter_type: "sql" }
					]

					queue_options = {
						enable_partitioning   = true,
						max_size_in_megabytes = 5120,
						max_message_size_in_kilobytes = 256
					}
				}
				`, endpoint_name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter", "Dg.Test.SubscriptionStateUpgrader.V1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0.filter_type", "sql"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", endpoint_name),
				),
			},
		},
	})

	ensure_enpoint_deleted(createClient(t), endpoint_name)
}

// Helper functions.
func create_test_endpoint(client asb.AsbClientWrapper, number_of_subscription uint, subscription_filter_type string, create_queue bool) (endpoint_name string, subscriptions []asb.AsbSubscriptionModel) {
	uuid := acctest.RandString(10)
	var ctx context.Context = context.Background()

	endpoint_name = uuid + "-test-endpoint"
	subscriptions = make([]asb.AsbSubscriptionModel, number_of_subscription)

	queueOptions := asb.AsbEndpointQueueOptions{
		EnablePartitioning:        pointer.Bool(true),
		MaxSizeInMegabytes:        pointer.Int32(int32(5120)),
		MaxMessageSizeInKilobytes: pointer.Int64(int64(256)),
	}

	model := asb.AsbEndpointModel{
		EndpointName:  endpoint_name,
		TopicName:     "bundle-1",
		Subscriptions: subscriptions,
		QueueOptions:  queueOptions,
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
		subscriptions[i] = asb.AsbSubscriptionModel{
			Filter:     fmt.Sprintf("Dg.Test.Subscription.V%v", i),
			FilterType: subscription_filter_type,
		}
		err = client.CreateAsbSubscriptionRule(ctx, model, subscriptions[i])
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
