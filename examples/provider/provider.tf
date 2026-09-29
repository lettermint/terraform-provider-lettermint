terraform {
  required_version = ">= 1.14.0"

  required_providers {
    lettermint = {
      source = "lettermint/lettermint"
    }
  }
}

provider "lettermint" {}
