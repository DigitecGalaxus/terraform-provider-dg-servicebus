.DEFAULT_GOAL := build
BIN_FILE=terraform-provider-dg-servicebus.exe

# Build the executable binary
build:
	@go build -o "${BIN_FILE}"

# Run the executable binary
run:
	./"${BIN_FILE}"

# Clean the project by removing build artifacts
clean:
	go clean
	rm --force "cp.out"
	rm --force nohup.out

# Run linter to analyze the code for potential issues
lint:
	golangci-lint run

# Run acceptance tests with Terraform
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m -count=1 -parallel=5

# Generate Terraform provider documentation
tfdocs-generate:
	tfplugindocs generate --provider-name "dgservicebus"
