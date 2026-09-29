action "lettermint_verify_domain_dns_record" "example" {
  config {
    domain_id = lettermint_domain.example.id
    record_id = lettermint_domain.example.dns_records[0].id
  }
}
