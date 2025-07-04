terraform {
  required_providers {
    podman = {
      source = "registry.terraform.io/pixambi/podman"
    }
  }
}

provider "podman" {}

data "podman_example" "example" {}

