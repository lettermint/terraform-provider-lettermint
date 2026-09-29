resource "lettermint_route" "inbound" {
  project_id             = lettermint_project.application.id
  name                   = "Support"
  route_type             = "inbound"
  inbound_domain         = "support.example.com"
  inbound_spam_threshold = 5
  attachment_delivery    = "url"
}
