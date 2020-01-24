# TODO: move the network configuration into terraform too? It was created by hand with:
# gcloud compute networks subnets update golang --region=us-central1 --enable-private-ip-google-access
#

terraform {
  backend "gcs" {
    bucket = "tf-state-prod-golang-org"
    prefix = "terraform/state"
  }
}

provider "google-beta" {
  project = "golang-org"
  region  = "us-central1"
  zone    = "us-central1-f"
}

provider "google" {
  project = "golang-org"
  region  = "us-central1"
  zone    = "us-central1-f"
}

data "local_file" "cloud_init" {
  filename = "${path.module}/cloud-init.yaml"
}

data "local_file" "konlet" {
  filename = "${path.module}/konlet.yaml.expanded"
}

data "google_compute_image" "cos" {
  family  = "cos-stable"
  project = "cos-cloud"
}

resource "google_compute_instance_template" "inst_tmpl" {
  name         = "play-sandbox-tmpl"
  machine_type = "n1-standard-8"
  metadata = {
    "ssh-keys"                  = "bradfitz:ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDaRpEbckQ+harGnrKUjk3JziwYqvz2bRNn0ngpzROaeCwm1XetDby/fgmQruZE/OBpbeOaCOd/yyP89Oer9CJx41AFEfHbudePZti/y+fmZ05N+QoBSAG0JtYWVydIjAjCenKBbNrYmwcQ840uNdIv9Ztqu3lbO/syMgcajappzdqMlwVZuHTJUe1JQD355PiinFHPTa7l0MrZPfiSsBdiTGmO39iVa312yshu6dZAvDgRL+bgIzTL6udPL/cVq+zlkvoZbzC4ajuZs4w2in+kqXHQSxbKHlXOhPrej1fwhspm+0Y7hEZOaN5Juc5GseNCHImtJh1rei1Qa4U/nTjt bradfitz@bradfitz-dev"
    "gce-container-declaration" = data.local_file.konlet.content
    "user-data"                 = data.local_file.cloud_init.content
  }
  network_interface {
    network = "golang"
  }
  service_account {
    scopes = ["logging-write", "storage-ro"]
  }
  disk {
    source_image = data.google_compute_image.cos.self_link
    auto_delete  = true
    boot         = true
  }
  scheduling {
    automatic_restart   = true
    on_host_maintenance = "MIGRATE"
  }
  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_region_autoscaler" "default" {
  provider = "google-beta"

  name   = "play-sandbox-autoscaler"
  region = "us-central1"
  target = "${google_compute_region_instance_group_manager.rigm.self_link}"

  autoscaling_policy {
    max_replicas    = 10
    min_replicas    = 3
    cooldown_period = 60

    cpu_utilization {
      target = 0.5
    }
  }
}

resource "google_compute_region_instance_group_manager" "rigm" {
  provider = "google-beta"
  name     = "play-sandbox-rigm"

  base_instance_name = "playsandbox"
  region             = "us-central1"

  version {
    name              = "primary"
    instance_template = "${google_compute_instance_template.inst_tmpl.self_link}"
  }

  named_port {
    name = "http"
    port = 80
  }
}

data "google_compute_region_instance_group" "rig" {
  provider  = "google-beta"
  self_link = "${google_compute_region_instance_group_manager.rigm.instance_group}"
}
