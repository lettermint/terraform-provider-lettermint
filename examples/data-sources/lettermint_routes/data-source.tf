data "lettermint_routes" "project" {
  project_id = "00000000-0000-4000-8000-000000000000"
  route_type = "transactional"
  sort       = ["name"]
}
