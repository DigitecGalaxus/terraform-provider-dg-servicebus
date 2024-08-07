---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "dgservicebus Provider"
subcategory: ""
description: |-
  This provider allows you to manage Endpoints for NServiceBus on Azure Service Bus.
---

# dgservicebus Provider

This provider allows you to manage Endpoints for NServiceBus on Azure Service Bus.

## Example Usage

```terraform
terraform {
  required_providers {
    dgservicebus = {
      source  = "DigitecGalaxus/dg-servicebus"
      version = ">= 1.0.0, < 2.0.0"
    }
  }
}

provider "dgservicebus" {
  azure_servicebus_hostname = "nservicebus.servicebus.windows.net"
  tenant_id                 = var.tenant_id
  client_id                 = var.client_id
  client_secret             = var.client_secret
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `azure_servicebus_hostname` (String) The hostname of the Azure Service Bus instance

### Optional

- `client_id` (String) The Client ID of the service principal. This can also be sourced from the `DG_SERVICEBUS_CLIENTID` Environment Variable.
- `client_secret` (String, Sensitive) The Client Secret of the service principal. This can also be sourced from the `DG_SERVICEBUS_CLIENTSECRET` Environment Variable.
- `tenant_id` (String) The Tenant ID of the service principal. This can also be sourced from the `DG_SERVICEBUS_TENANTID` Environment Variable.
