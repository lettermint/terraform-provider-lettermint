resource "lettermint_project" "application" {
  name                 = "Application"
  smtp_enabled         = true
  initial_routes       = "both"
  short_token          = false
  redact_email_content = true
}
