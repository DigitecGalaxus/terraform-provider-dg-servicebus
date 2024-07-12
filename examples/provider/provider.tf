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
