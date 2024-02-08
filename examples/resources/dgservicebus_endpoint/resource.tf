resource "dgservicebus_endpoint" "example" {
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
    max_message_size_in_kilobytes = 256,
  }
}
