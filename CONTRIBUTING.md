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

The provider must match both the source OpenAPI specifications and `github.com/lettermint/lettermint-go/v3`. Do not add an undocumented API request, default, retry, validation rule, or response status.

Do not copy source OpenAPI specifications into this repository. Keep them in the API repository. This repository stores only their names, hashes, and the coverage classification. When the API contract changes, review the change before you update its hash and coverage classification.

To update coverage, give the command the external specification directory:

```shell
go run ./internal/contract/cmd/coverage -spec-dir /path/to/api-repository/docs/api-reference
go run ./internal/contract/cmd/coverage -check -spec-dir /path/to/api-repository/docs/api-reference
LETTERMINT_SPEC_DIR=/path/to/api-repository/docs/api-reference go test ./internal/contract
```

The command checks source hashes before it generates coverage. It does not copy source files into the repository. Normal tests check the coverage policy and use small test inputs. CI does not require a checkout of the API repository. Run the external contract checks before an API contract update.

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

`go generate` generates provider documentation only. Review generated changes before you commit them.

## Open a Pull Request

Explain the user-visible change and the checks that you ran. Link an issue when one exists. Do not include tokens, secrets, customer data, or other sensitive values.
