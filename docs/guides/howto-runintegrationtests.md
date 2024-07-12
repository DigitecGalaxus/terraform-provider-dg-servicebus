---
page_title: "Run integration tests locally"
---

# Run integration tests locally

## Goal

The goal of this guide is to provide instructions on how to run all integration tests locally.

## Step-by-Step

Follow these steps to run the integration tests locally:

1. Open a terminal in the root directory of the project.
2. Depending on your operating system, run the appropriate command:

   - For Linux and Mac:

     ```shell
     make testacc
     ```

   - For Windows:

     ```powershell
     .\run-tests.ps1
     ```

Remember to ensure that all necessary dependencies are installed before running the tests.
