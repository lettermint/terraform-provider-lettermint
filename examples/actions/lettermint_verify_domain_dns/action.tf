action "lettermint_verify_domain_dns" "example" {
  config {
    domain_id = lettermint_domain.example.id
  }
}
