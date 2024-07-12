---
page_title: "Use local provider in terraform root moduls"
---

# How-To use local provider in terraform deployment

## Goal

Use the local version of the terraform provider plugin to test the changes in an real life terraform root module using this provider.

1. Navigate to the root directory of the terraform provider and build the provider using the command `go build`. Make sure to run this command after every update to the provider.

2. Depending on your operating system, create the appropriate configuration file for the CLI. On Linux/Apple, create `~/.terraformrc`. On Windows, create `%APPDATA\terraform.rc`. Inside the configuration file, add the following code:

```hcl
provider_installation {
  dev_overrides {
   "DigitecGalaxus/dg-servicebus" = "/mnt/c/Developement/terraform-provider-dg-servicebus"
  }
}
```

3. Navigate to the root directory of the desired root module and run `terraform init -reconfigure`. You should see a warning message in the output indicating that the provider has been overwritten.

![Provider overwrite warning](provider_overwrite.jpg)
