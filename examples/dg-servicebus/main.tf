terraform {
  required_providers {
    dgservicebus = {
      source = "hashicorp.com/edu/dg-servicebus"
    }
  }
}

provider "dgservicebus" {
  azure_servicebus_hostname = "terraform-provider-test.servicebus.windows.net"
}


resource "dgservicebus_endpoint" "dev" {
  endpoint_name = "dg-nservicebus-test-endpoint"
  topic_name    = "bundle-1"
  subscriptions = [
    "Dg.SalesOrder.V1.SalesOrderCreated",
    "Dg.SalesOrder.V1.A",
    "Dg.SalesOrder.V1.AAA",
  ]
  queue_options = {
    enable_partitioning   = false,
    max_size_in_megabytes = 5120,
  }
}
