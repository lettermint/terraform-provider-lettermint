resource "lettermint_project" "example" {
  name = "Example"
}

resource "lettermint_domain" "example" {
  domain = "example.com"

  # Only these projects can send from the domain. Leave this out to manage
  # project assignments in the dashboard, or set [] to allow every project.
  project_ids = [lettermint_project.example.id]
}

output "lettermint_dns_records" {
  value = lettermint_domain.example.dns_records
}
