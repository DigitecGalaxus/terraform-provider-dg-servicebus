---
page_title: "Generating Terraform Documentation"
---

# Generating Terraform Documentation

## Goal

The goal of this guide is to provide instructions on how to generate or update Terraform documentation for your project. This process is necessary when you have made changes to the attributes on the provider's schema or updated documentation in the /template directory.

## Step-by-Step

1. Navigate to the root directory of the repository and open a shell.
2. Ensure that tfpluindocs is installed by running `make tfdocs-install`.
3. Execute `make tfdocs-generate`.
