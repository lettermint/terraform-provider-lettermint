default: fmt vet test generate

build:
	go build -v ./...

fmt:
	gofmt -s -w -e .
	terraform fmt -recursive examples

generate:
	go generate ./...

test:
	go test -race ./...

testacc:
	TF_ACC=1 go test -v -timeout 120m ./internal/provider/...

vet:
	go vet ./...

snapshot:
	goreleaser release --snapshot --clean --skip=sign

.PHONY: build fmt generate snapshot test testacc vet
