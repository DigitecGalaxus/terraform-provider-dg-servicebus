---
page_title: "Debugging Provider in VSCode"
---

# Debugging Provider in VSCode

## Goal

The goal of this guide is to explain how to debug a provider with an actual terraform plan/apply in VSCode.

## Step-by-Step instructions

1. Add a launch config to the .vscode/launch.json with the `--debug` argument
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Provider",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}",
      "env": {},
      "args": [
          "--debug",
      ]
    },
  ]
}
```

2. Launch the profile with the debugger.
The debug session will output information regarding the `TF_REATTACH_PROVIDERS` environment variable.
Set this variable according to the output, per example: `export TF_REATTACH_PROVIDERS='{"registry.terraform.io/DigitecGalaxus/dg-servicebus":{"Protocol":"grpc","ProtocolVersion":6,"Pid":480731,"Test":true,"Addr":{"Network":"unix","String":"/tmp/plugin128242622"}}}'`

3. Create a `~/.terraformrc` file in your home directory. Check the [use local provider](./howto-uselocalprovider.md) guide.

4. Set brake points in VSCode.

5. Launch a terraform plan/apply from the same terminal where you set the `TF_REATTACH_PROVIDERS` environment variable from a project where you use this provider


## Source

https://medium.com/@aserkan/debugging-terraform-plugins-with-visual-studio-code-vscode-f3f8734c8cb0
