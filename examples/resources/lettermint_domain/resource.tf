resource "lettermint_domain" "example" {
  domain = "example.com"
}

output "lettermint_dns_records" {
  value = lettermint_domain.example.dns_records
}
