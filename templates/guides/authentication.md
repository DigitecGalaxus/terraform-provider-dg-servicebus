---
page_title: "Authentication"
---

# Authentication

To authenticate, either provide credentials for an Azure Service Principle or authenticate using default credentials. The Service Principal credentials take priority over the default credentials and will always be used, if provided.

To run the provider locally, install the Azure CLI, which acts as a token source for the default credential. Be sure to run az login first, to log in with your account.
