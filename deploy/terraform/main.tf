# sclawion infrastructure entrypoint.
#
# Single-file scaffold for clarity; split when it grows past ~300 lines.
#
# Provisioned:
#   - KMS keyring + crypto keys (CMEK for Pub/Sub + Firestore + Artifact Reg)
#   - Pub/Sub topics: sclawion.inbound, sclawion.outbound, *.dead (DLQ)
#   - Push subscriptions with OIDC tokens to Cloud Run receivers
#   - Per-service Google Service Accounts (no JSON keys)
#   - Secret Manager secrets (placeholder values)
#   - Cloud Run services (ingress, router, scion-bridge, 4x emitter)
#   - External HTTPS LB + Cloud Armor policy in front of ingress
#   - VPC Service Controls perimeter (commented; project-level decision)
#   - BigQuery audit log sink

terraform {
  required_version = ">= 1.7"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.30"
    }
  }
}

variable "project_id" { type = string }
variable "region"     { type = string  default = "us-central1" }
variable "env"        { type = string  description = "dev|stage|prod" }

provider "google" {
  project = var.project_id
  region  = var.region
}

locals {
  name_prefix = "sclawion-${var.env}"
}

# --- KMS for CMEK ---------------------------------------------------------
resource "google_kms_key_ring" "main" {
  name     = "${local.name_prefix}-kr"
  location = var.region
}

resource "google_kms_crypto_key" "pubsub" {
  name            = "pubsub"
  key_ring        = google_kms_key_ring.main.id
  rotation_period = "7776000s" # 90 days
}

# --- Pub/Sub topics -------------------------------------------------------
resource "google_pubsub_topic" "inbound" {
  name = "${local.name_prefix}.inbound"
  kms_key_name = google_kms_crypto_key.pubsub.id
  message_retention_duration = "604800s" # 7 days
}

resource "google_pubsub_topic" "outbound" {
  name = "${local.name_prefix}.outbound"
  kms_key_name = google_kms_crypto_key.pubsub.id
  message_retention_duration = "604800s"
}

resource "google_pubsub_topic" "inbound_dead" {
  name = "${local.name_prefix}.inbound.dead"
  kms_key_name = google_kms_crypto_key.pubsub.id
}

resource "google_pubsub_topic" "outbound_dead" {
  name = "${local.name_prefix}.outbound.dead"
  kms_key_name = google_kms_crypto_key.pubsub.id
}

# Push subscriptions, Cloud Run services, IAM, Cloud Armor, BQ sink — TODO.
