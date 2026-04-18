# Config plane (cmd/admin-api) — IAP-fronted Cloud Run service that owns the
# Firestore + Secret Manager state the data plane reads. Infra (KMS, topics,
# IAM, Cloud Armor *rules*, BigQuery sinks, Binary Auth) stays Terraform-owned.
#
# Provisioned here:
#   - Admin GSA + IAM bindings (Firestore RW, Secret Manager versionAdder)
#   - IAP brand + OAuth client (one per project; brand creation is manual the
#     first time per GCP — comment below)
#   - Firestore composite indexes for config_audit reads
#   - BigQuery dataset + table for long-term audit storage
#   - Bootstrap admin user document (refuses to start if admin_users empty)
#
# Not provisioned here (lives in main.tf or is set out-of-band):
#   - The HTTPS LB itself + IAP toggle on the backend service
#   - Cloud Run service object (deploy/cloudrun/admin-api.yaml)

variable "admin_owner_email" {
  type        = string
  description = "Email of the bootstrap owner. Seeded into admin_users on first apply."
}

variable "iap_support_email" {
  type        = string
  description = "Support email shown on the IAP consent screen. Must be a Google Group the project owners belong to, or the project owner address."
}

# --- Admin GSA ------------------------------------------------------------
resource "google_service_account" "admin_api" {
  account_id   = "${local.name_prefix}-admin-api"
  display_name = "sclawion admin API (config plane)"
}

resource "google_project_iam_member" "admin_api_firestore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.admin_api.email}"
}

# Read existing versions (for connector docs that reference SecretRef) +
# add new versions on rotate. Resource creation stays Terraform-owned.
resource "google_project_iam_member" "admin_api_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.admin_api.email}"
}

resource "google_project_iam_member" "admin_api_secret_versionadder" {
  project = var.project_id
  role    = "roles/secretmanager.secretVersionAdder"
  member  = "serviceAccount:${google_service_account.admin_api.email}"
}

# Audit logs land in BigQuery via the existing log sink; admin-api also writes
# the Firestore mirror via roles/datastore.user above.
resource "google_project_iam_member" "admin_api_bq_data_editor" {
  project = var.project_id
  role    = "roles/bigquery.dataEditor"
  member  = "serviceAccount:${google_service_account.admin_api.email}"
}

# --- IAP brand + OAuth client --------------------------------------------
# Brand creation requires manual one-time setup if the project doesn't already
# have an internal-type brand. After that this resource is no-op-importable.
resource "google_iap_brand" "admin" {
  support_email     = var.iap_support_email
  application_title = "sclawion admin"
}

resource "google_iap_client" "admin" {
  display_name = "sclawion-admin-${var.env}"
  brand        = google_iap_brand.admin.name
}

# --- BigQuery audit table -------------------------------------------------
resource "google_bigquery_dataset" "audit" {
  dataset_id    = "sclawion_audit_${var.env}"
  location      = var.region
  description   = "Long-term store for sclawion config-plane audit entries (400-day retention per SECURITY.md)."
  default_table_expiration_ms = 1000 * 60 * 60 * 24 * 400
}

resource "google_bigquery_table" "audit_entries" {
  dataset_id = google_bigquery_dataset.audit.dataset_id
  table_id   = "entries"
  deletion_protection = true

  time_partitioning {
    type  = "DAY"
    field = "at"
  }

  schema = jsonencode([
    { name = "id",            type = "STRING",    mode = "REQUIRED" },
    { name = "actor",         type = "STRING",    mode = "REQUIRED" },
    { name = "actor_role",    type = "STRING",    mode = "REQUIRED" },
    { name = "action",        type = "STRING",    mode = "REQUIRED" },
    { name = "resource_type", type = "STRING",    mode = "REQUIRED" },
    { name = "resource_id",   type = "STRING",    mode = "NULLABLE" },
    { name = "tenant_id",     type = "STRING",    mode = "NULLABLE" },
    { name = "before",        type = "JSON",      mode = "NULLABLE" },
    { name = "after",         type = "JSON",      mode = "NULLABLE" },
    { name = "at",            type = "TIMESTAMP", mode = "REQUIRED" },
    { name = "trace_id",      type = "STRING",    mode = "NULLABLE" },
    { name = "span_id",       type = "STRING",    mode = "NULLABLE" },
    { name = "request_id",    type = "STRING",    mode = "NULLABLE" },
    { name = "result",        type = "STRING",    mode = "REQUIRED" },
    { name = "error",         type = "STRING",    mode = "NULLABLE" },
  ])
}

# --- Firestore composite indexes for audit queries -----------------------
# Queries: filter by tenant_id + order by at desc; filter by actor + order by at desc.
resource "google_firestore_index" "audit_by_tenant" {
  collection = "config_audit"
  fields {
    field_path = "tenant_id"
    order      = "ASCENDING"
  }
  fields {
    field_path = "at"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "audit_by_actor" {
  collection = "config_audit"
  fields {
    field_path = "actor"
    order      = "ASCENDING"
  }
  fields {
    field_path = "at"
    order      = "DESCENDING"
  }
}

# --- Bootstrap owner -------------------------------------------------------
# admin-api refuses to start if admin_users is empty. This document closes the
# boot-loop on a fresh deploy. Subsequent owner edits go through the UI.
resource "google_firestore_document" "bootstrap_owner" {
  collection  = "admin_users"
  document_id = var.admin_owner_email
  fields = jsonencode({
    email    = { stringValue = var.admin_owner_email },
    role     = { stringValue = "owner" },
    added_by = { stringValue = "terraform-bootstrap" },
    added_at = { timestampValue = timestamp() }
  })

  lifecycle {
    # Don't replace the doc if the owner edits their record via the UI.
    ignore_changes = [fields]
  }
}

output "admin_api_service_account" {
  value = google_service_account.admin_api.email
}

output "iap_client_id" {
  value     = google_iap_client.admin.client_id
  sensitive = true
}
