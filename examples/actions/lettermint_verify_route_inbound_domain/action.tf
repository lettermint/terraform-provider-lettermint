action "lettermint_verify_route_inbound_domain" "example" {
  config {
    route_id = lettermint_route.inbound.id
  }
}
