# Lettermint Terraform Provider

[![Terraform Registry](https://img.shields.io/badge/Terraform%20Registry-lettermint%2Flettermint-7B42BC?logo=terraform&logoColor=white&style=flat-square)](https://registry.terraform.io/providers/lettermint/lettermint/latest)
[![GitHub Tests](https://img.shields.io/github/actions/workflow/status/lettermint/terraform-provider-lettermint/ci.yml?branch=main&label=tests&style=flat-square)](https://github.com/lettermint/terraform-provider-lettermint/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/lettermint/terraform-provider-lettermint?include_prereleases&style=flat-square)](https://github.com/lettermint/terraform-provider-lettermint/releases)
[![License](https://img.shields.io/github/license/lettermint/terraform-provider-lettermint?style=flat-square)](LICENSE)
[![Join our Discord server](https://img.shields.io/discord/1305510095588819035?logo=discord&logoColor=eee&label=Discord&labelColor=464ce5&color=0D0E28&cacheSeconds=43200)](https://lettermint.co/r/discord)

The official Terraform provider for [Lettermint](https://lettermint.co).

## Requirements

- Terraform 1.14 or later
- A Lettermint team token
- Go 1.25 or later for development

## Installation

Add the provider to your Terraform configuration:

```hcl
terraform {
  required_version = ">= 1.14.0"

  required_providers {
    lettermint = {
      source = "lettermint/lettermint"
    }
  }
}
```

Run `terraform init` to install the provider from the Terraform Registry.

## Usage

Set the team token in the environment:

```shell
export LETTERMINT_TEAM_TOKEN="your-team-token"
```

The provider also accepts `team_token` in its configuration. Terraform marks this value as sensitive.

```hcl
provider "lettermint" {}

resource "lettermint_project" "application" {
  name                 = "Application"
  smtp_enabled         = true
  initial_routes       = "both"
  short_token          = false
  redact_email_content = true
}

resource "lettermint_domain" "example" {
  domain = "example.com"
}
```

The project `api_token` and webhook `secret` are sensitive values that the API returns only during creation. Store them securely. An import cannot recover them.

## Resources

- [`lettermint_project`](docs/resources/project.md) manages a project.
- [`lettermint_domain`](docs/resources/domain.md) manages a sending domain.
- [`lettermint_route`](docs/resources/route.md) manages a route and its inbound settings.
- [`lettermint_webhook`](docs/resources/webhook.md) manages a route-scoped webhook.

## Data Sources

- [`lettermint_team`](docs/data-sources/team.md) reads the current team.
- [`lettermint_project`](docs/data-sources/project.md) and [`lettermint_projects`](docs/data-sources/projects.md) read projects.
- [`lettermint_domain`](docs/data-sources/domain.md) and [`lettermint_domains`](docs/data-sources/domains.md) read domains.
- [`lettermint_route`](docs/data-sources/route.md) and [`lettermint_routes`](docs/data-sources/routes.md) read routes.
- [`lettermint_webhook`](docs/data-sources/webhook.md) and [`lettermint_webhooks`](docs/data-sources/webhooks.md) read route-scoped webhooks.

## Actions

- [`lettermint_verify_domain_dns`](docs/actions/verify_domain_dns.md) verifies all DNS records for a domain.
- [`lettermint_verify_domain_dns_record`](docs/actions/verify_domain_dns_record.md) verifies one DNS record.
- [`lettermint_verify_route_inbound_domain`](docs/actions/verify_route_inbound_domain.md) verifies a route inbound domain.

Terraform actions require Terraform 1.14 or later.

## Domain DNS Records

The domain create response can contain computed `dns_records`. Use these records to configure your DNS provider. Then invoke `lettermint_verify_domain_dns` or `lettermint_verify_domain_dns_record`.

Terraform does not know the DNS record collection keys before Lettermint creates the domain. A DNS provider resource that uses these keys can require a second apply.

A later domain read can omit the optional DNS record relationship. The provider keeps the records that are already in state. It clears the records only when the API returns an empty array.

An imported domain cannot recover DNS records. `lettermint-go` v2.6.0 cannot add `include=dnsRecords` to a domain read. See the [domain DNS guide](docs/guides/domain-dns.md) for the complete flow.

## Current SDK Scope

This provider uses `github.com/lettermint/lettermint-go/v2` v2.6.0. It does not use a second HTTP client.

The current release does not include delivery modes, expanded webhook scopes, relationship reads, sending, messages, statistics, suppressions, webhook deliveries, token or secret rotation, or webhook tests. See [`contracts/coverage.json`](contracts/coverage.json) for the complete operation and field classification.

The webhook list API does not document a cursor request parameter. Therefore, the `lettermint_webhooks` data source returns the documented response page and does not send an undocumented cursor query.

## Development

<details>
<summary>Tests, generation, and contract checks</summary>

Install Go 1.25 or later, and run:

```shell
go generate ./...
go test -race ./...
go vet ./...
goreleaser release --snapshot --clean --skip=sign
```

The checked OpenAPI files are immutable snapshots. Update the snapshots, hashes, and coverage policy together. Do not edit an OpenAPI snapshot to make a provider test pass.

</details>

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for release changes.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution instructions.

## Security

See [SECURITY.md](SECURITY.md) to report a security issue privately.

## Support

For help, join the [Lettermint Discord server](https://lettermint.co/r/discord).

## Credits

- [Bjarn Bronsveld](https://github.com/bjarn)

## License

The MIT License (MIT). See [LICENSE](LICENSE) for details.
