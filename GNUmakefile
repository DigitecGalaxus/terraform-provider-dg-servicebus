.DEFAULT_GOAL := build
BIN_FILE=terraform-provider-dg-servicebus.exe
.PHONY: testacc
build:
	@go build -o "${BIN_FILE}"
run:
	./"${BIN_FILE}"
clean:
	go clean
	rm --force "cp.out"
	rm --force nohup.out
lint:
	golangci-lint run
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m -count=1 -parallel=5
tfdocs-generate:
	tfplugindocs generate --provider-name "dgservicebus"
