# Contributing

Thank you for contributing to the Lettermint Terraform provider.

## Requirements

- Go 1.25 or later
- Terraform 1.14 or later
- GoReleaser for release snapshot checks

## Set Up the Repository

Fork and clone the repository. Then download the Go modules:

```shell
go mod download
```

## Make a Change

Keep each change small and focused. Add tests for changed behavior. Update the documentation when the provider interface changes.

The provider must match both the checked OpenAPI specifications and `github.com/lettermint/lettermint-go/v2`. Do not add an undocumented API request, default, retry, validation rule, or response status.

Do not edit a checked OpenAPI snapshot to make a test pass. When the API contract changes, update the snapshot, its hash, and the coverage classification together.

## Run the Checks

Run these commands before you open a pull request:

```shell
gofmt -s -w -e .
terraform fmt -recursive examples
go generate ./...
go test -race ./...
go vet ./...
goreleaser release --snapshot --clean --skip=sign
git diff --check
```

Review generated changes before you commit them.

## Open a Pull Request

Explain the user-visible change and the checks that you ran. Link an issue when one exists. Do not include tokens, secrets, customer data, or other sensitive values.
