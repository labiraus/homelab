terraform {
  required_version = ">= 1.14.0"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
}
provider "aws" { region = "eu-west-1" }
variable "inference_ca_certificate_pem" {
  type        = string
  description = "Public dedicated workload CA certificate, supplied through TF_VAR; private key remains offline."
}
variable "inference_budget_emails" {
  type    = list(string)
  default = []
}
module "bedrock" {
  source              = "../../modules/inference-bedrock"
  ca_certificate_pem  = var.inference_ca_certificate_pem
  notification_emails = var.inference_budget_emails
}
output "role_arn" { value = module.bedrock.role_arn }
output "profile_arn" { value = module.bedrock.profile_arn }
output "trust_anchor_arn" { value = module.bedrock.trust_anchor_arn }
