# Testing

We run the test on the [DG-PROD-Chabis-Messaging-Testing](https://portal.azure.com/#@migros.onmicrosoft.com/resource/subscriptions/1f528d4c-510c-40ed-b8e2-3865dd80f12c/resourceGroups/Messaging-Prod/providers/Microsoft.ServiceBus/namespaces/DG-PROD-Chabis-Messaging-Testing/overview) Servicebus instance.

To ensure the tests succeed, please make sure the following components exist:
- Topic: `bundle-1` with a subscription named `test-queue`
- Queue: `test-queue`

## Run Tests Locally
To run the tests locally, ensure that you are logged in with `az login`, and then execute the following command from the repository root, depending on your shell:

### Bash
```shell
# chmod +x ./run-tests.sh
./run-tests.sh
```

### PowerShell
```powershell
./run-tests.ps1
```

### Makefile
```shell
make testacc
```

## Debug test
To debug a test, follow these steps:
1. Select the name of the test function.
2. In the 'Debug and run' tab, choose the 'Debug tests' configuration.
3. Press the 'Run' icon.

![debugging picture](debugging.jpg)
