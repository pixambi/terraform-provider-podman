terraform {
  required_providers {
    podman = {
      source = "registry.terraform.io/pixambi/podman"
    }
  }
}

provider "podman" {
    connection = "unix:///run/podman/podman.sock"
    identity  = "default"
    host      = "localhost"
    username  = "user"
    uri       = "ssh://localhost:8080"
    socket_path = "/run/podman/podman.sock"
}

data "podman_example" "example" {}

