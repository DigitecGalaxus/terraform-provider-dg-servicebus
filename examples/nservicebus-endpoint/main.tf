terraform {
  required_providers {
    nservicebus = {
      source = "hashicorp.com/edu/nservicebus"
    }
  }

  required_version = ">= 1.1.0"
}


provider "nservicebus" {
  azure_servicebus_hostname = "terraform-provider-test.servicebus.windows.net"
}

resource "nservicebus_endpoint" "dev" {
  endpoint_name = "dg-nservicebus-test-endpoint"
  resource_group = "Messaging-Dev"
  topic_name = "bundle-1"
  subscriptions = [
    "Dg.SalesOrder.V1.SalesOrderCreated",
    "Dg.SalesOrder.V1.A",
    "Dg.SalesOrder.V1.AAA",
  ]
  queue_options = {
    enable_partitioning = false,
    max_size_in_megabytes = 5120,
  }
}