package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const providerConfig = `
provider "dgservicebus" {
    azure_servicebus_hostname = "DG-PROD-Chabis-Messaging-Testing.servicebus.windows.net"
    tenant_id                 = "35aa8c5b-ac0a-4b15-9788-ff6dfa22901f"
    client_id                 = "377ac5e1-4767-42c5-b778-d16039b1a201"
}


provider "azurerm" {
    subscription_id = "1f528d4c-510c-40ed-b8e2-3865dd80f12c" # Module Subscription
    tenant_id       = "35aa8c5b-ac0a-4b15-9788-ff6dfa22901f"
    client_id                 = "377ac5e1-4767-42c5-b778-d16039b1a201"
    
    features {}
}
`

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"dgservicebus": providerserver.NewProtocol6WithError(New("test")()),
}

func TestAcc_TestResource(t *testing.T) {
	var uuid string = strings.Replace(uuid.New().String(), "-", "", -1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"azurerm": {
				Source:            "hashicorp/azurerm",
				VersionConstraint: "",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "%v-test-endpoint"
					topic_name    = "bundle-1"
					subscriptions = [
						"Dg.Test.V1.Subscription"
					]
				  
					queue_options = {
					  enable_partitioning   = true,
					  max_size_in_megabytes = 5120
					}
				}`, uuid),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify number of coffees returned
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "subscriptions.0", "Dg.Test.V1.Subscription"),
					// Verify the first coffee to ensure all attributes are set
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_exists", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "endpoint_name", fmt.Sprintf("%v-test-endpoint", uuid)),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_exists", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "5120"),
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
		ExternalProviders: map[string]resource.ExternalProvider{
			"azurerm": {
				Source:            "hashicorp/azurerm",
				VersionConstraint: "",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
				data "dgservicebus_endpoint" "test" {
					endpoint_name = "test-queue"
					topic_name	= "bundle-1"
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify number of coffees returned
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.#", "1"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "subscriptions.0", "Dg.Test.V1.Subscription"),
					// Verify the first coffee to ensure all attributes are set
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "endpoint_name", "test-queue"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.enable_partitioning", "true"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "queue_options.max_size_in_megabytes", "81920"),
					resource.TestCheckResourceAttr("data.dgservicebus_endpoint.test", "topic_name", "bundle-1"),
				),
			},
		},
	})
}
