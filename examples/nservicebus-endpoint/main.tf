// Resource Groups
data "azurerm_resource_group" "resource_group" {
  name = "Messaging-${title(var.environment)}"
}

// Service Bus Instances
data "azurerm_servicebus_namespace" "servicebus_instance" {
  name                = "Dg-${title(var.environment)}-Messaging-NServiceBus"
  resource_group_name = data.azurerm_resource_group.resource_group.name
}

// Bundle Topic
data "azurerm_servicebus_topic" "nservicebus_bundle_topic" {
  name                = var.topic_name
  namespace_name      = "Dg-${title(var.environment)}-Messaging-NServiceBus"
  resource_group_name = "Messaging-${title(var.environment)}"
}

locals {
  isPremium            = data.azurerm_servicebus_namespace.servicebus_instance.sku == "Premium"
  hasSubscriptionRules = length(var.subscriptions) > 0
}

// Endpoint Queue
resource "azurerm_servicebus_queue" "nservicebus_endpoint_queue" {
  name                      = var.endpoint_name
  namespace_id              = data.azurerm_servicebus_namespace.servicebus_instance.id
  enable_partitioning       = local.isPremium ? false : true
  max_delivery_count        = 2147483647
  max_size_in_megabytes     = local.isPremium ? 81920 : 5120
  enable_batched_operations = true
  lock_duration             = "PT5M"
}

// Endpoint Subscription
resource "azurerm_servicebus_subscription" "nservicebus_endpoint_subscription" {
  count = local.hasSubscriptionRules ? 1 : 0

  name                                      = var.endpoint_name
  topic_id                                  = data.azurerm_servicebus_topic.nservicebus_bundle_topic.id
  max_delivery_count                        = 2147483647
  enable_batched_operations                 = true
  lock_duration                             = "PT5M"
  dead_lettering_on_message_expiration      = false
  dead_lettering_on_filter_evaluation_error = false
  requires_session                          = false
  forward_to                                = azurerm_servicebus_queue.nservicebus_endpoint_queue.name
  status                                    = "Disabled"

  lifecycle {
    ignore_changes = [
      status
    ]
  }
}

// Subscription Rules
resource "azurerm_servicebus_subscription_rule" "nservicebus_endpoint_subscription_rule" {
  for_each = toset(var.subscriptions)

  name            = trim(substr(each.key, -50, -1), ".")
  subscription_id = azurerm_servicebus_subscription.nservicebus_endpoint_subscription[0].id
  filter_type     = "SqlFilter"
  sql_filter      = "[NServiceBus.EnclosedMessageTypes] LIKE '%${each.key}%'"
}

resource "dg_nservicebus_endpoint" "nservicebus_endpoint" {
  environment = "dev"

  module_name   = "Dg.NServiceBusTestEndpoint"
  product_group = "Dg.NServiceBusTestEndpoint"
  endpoint_name = "dg-nservicebus-test-endpoint"
  subscriptions = [
    "Dg.SalesOrder.V1.SalesOrderCreated",
  ]
}