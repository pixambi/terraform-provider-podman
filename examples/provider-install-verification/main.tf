terraform {
  required_providers {
    podman = {
      source = "registry.terraform.io/pixambi/podman"
    }
  }
}

provider "podman" {
  connection = "podman-machine-default"
}

data "podman_example" "example" {}

