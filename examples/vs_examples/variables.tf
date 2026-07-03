variable "lab_tkn" {
  description = "The ScyllaDB Cloud API token for authentication."
  type        = string
  sensitive   = true
}

variable "api_endpoint" {
  description = "The ScyllaDB Cloud API endpoint."
  type        = string
}
