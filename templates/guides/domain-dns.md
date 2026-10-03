---
page_title: "Domain DNS setup"
description: |-
  Create a Lettermint domain, configure its DNS records, and verify the records.
---

# Domain DNS setup

Create a `lettermint_domain` resource first. The create response can contain the computed `dns_records` list.

Use each returned record with your selected DNS provider. Use the record ID as a stable collection key when possible. Terraform does not know these keys before Lettermint creates the domain. A second apply can be necessary.

After DNS changes propagate, invoke one of these actions:

- `lettermint_verify_domain_dns` verifies all records for the domain.
- `lettermint_verify_domain_dns_record` verifies one record.

Terraform actions require Terraform 1.14 or later.

The domain read operation can omit the optional DNS record relationship. The provider keeps the records that are already in state. It clears them only when the API returns an empty array.

This provider does not add `include=dnsRecords` to a domain read. Therefore, an imported domain cannot recover DNS records. This is a provider limit, not a limit of the current Go SDK. Keep this limit in mind before you import a domain.
