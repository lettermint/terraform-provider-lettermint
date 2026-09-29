resource "lettermint_webhook" "delivered" {
  route_id = lettermint_route.transactional.id
  name     = "Delivery events"
  url      = "https://example.com/webhooks/lettermint"
  events   = ["message.delivered", "message.hard_bounced"]
  enabled  = true
}
