# Lettermint Terraform Provider

Use the provider to manage Lettermint projects, domains, routes, and route webhooks.

## Requirements

- Terraform 1.14 or later
- Go 1.25 or later for development
- A Lettermint team token

## Use the provider

```hcl
terraform {
  required_version = ">= 1.14.0"

  required_providers {
    lettermint = {
      source = "lettermint/lettermint"
    }
  }
}

provider "lettermint" {}
```

Set `LETTERMINT_TEAM_TOKEN` before you run Terraform. You can also set `team_token` in the provider block. Terraform marks this value as sensitive.

## Domain DNS records

The domain create response can contain `dns_records`. Use these computed records to configure your DNS provider. Then invoke `lettermint_verify_domain_dns` or `lettermint_verify_domain_dns_record`.

The DNS record collection keys are not known before Lettermint creates the domain. A DNS provider resource that uses these keys can require a second apply.

A later domain read can omit the optional DNS record relationship. The provider keeps the existing records in state when this happens. The provider clears the records only when the API returns an empty array.

An imported domain cannot recover DNS records. `lettermint-go` v2.6.0 cannot add `include=dnsRecords` to a domain read.

## Current SDK subset

This provider uses `github.com/lettermint/lettermint-go/v2` v2.6.0. It does not use a second HTTP client.

The current release does not include delivery modes, expanded webhook scopes, relationship reads, sending, messages, statistics, suppressions, webhook deliveries, token or secret rotation, or webhook tests. See `contracts/coverage.json` for the complete operation and field classification.

The webhook list API does not document a cursor request parameter. For this reason, the `lettermint_webhooks` data source returns the documented response page and does not send an undocumented cursor query.

## Development

```shell
go generate ./...
go test -race ./...
go vet ./...
goreleaser release --snapshot --clean --skip=sign
```

The checked OpenAPI files are immutable snapshots. Update the snapshots, hashes, and coverage policy together. Do not edit an OpenAPI snapshot to make a provider test pass.

## License

MIT
