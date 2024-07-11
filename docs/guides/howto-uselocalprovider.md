---
page_title: "Use local provider in terraform root moduls"
---

# How-To use local provider in terraform deployment

## Goal

Use the local version of the terraform provider plugin to test the changes in an real life terraform root module using this provider.

## Step-By-Step

1. Depending on the OS create the following file (see [CLI Configuration](https://developer.hashicorp.com/terraform/cli/config/config-file#locations))

   - On Linux/Apple `~/.terraformrc`
   - On Windows `%APPDATA\terraform.rc`

   ```hcl
   provider_installation {
     dev_overrides {
       "DigitecGalaxus/dg-servicebus" = "/mnt/c/Developement/terraform-provider-dg-servicebus"
     }
   }
   ```

2. Go the root directory of the root module you want to deploy and run `terraform init -reconfigure`. Your should see the follwing message in the output:

   ![Provider overwrite warning](provider_overwrite.jpg)
