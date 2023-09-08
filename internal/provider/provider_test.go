package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const providerConfig = `
provider "dgservicebus" {
    azure_servicebus_hostname = "DG-PROD-Chabis-Messaging-Testing.servicebus.windows.net"
    tenant_id                 = "35aa8c5b-ac0a-4b15-9788-ff6dfa22901f"
}


provider "azurerm" {
    subscription_id = "1f528d4c-510c-40ed-b8e2-3865dd80f12c" # Module Subscription
    tenant_id       = "35aa8c5b-ac0a-4b15-9788-ff6dfa22901f"
    
    features {}
}
`

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"dgservicebus": providerserver.NewProtocol6WithError(New("test")()),
}

func TestAccProviderTests(t *testing.T) {
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
				resource "dgservicebus_endpoint" "test" {
					endpoint_name = "test-endpoint"
					topic_name    = "bundle-1"
					subscriptions = [
						"Dg.Test.V1.Subscription"
					]
				  
					queue_options = {
					  enable_partitioning   = true,
					  max_size_in_megabytes = 5120
					}
				  }`,
			},
		},
	})
}
