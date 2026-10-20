variable "ca_certificate_pem" {
  description = "Dedicated public workload CA certificate; never its private key."
  type        = string
}
variable "notification_emails" {
  type    = list(string)
  default = []
}
data "aws_caller_identity" "current" {}
locals {
  region   = "eu-west-1"
  regions  = ["eu-central-1", "eu-north-1", "eu-south-1", "eu-south-2", "eu-west-1", "eu-west-3"]
  models   = ["amazon.nova-2-lite-v1:0", "anthropic.claude-sonnet-4-5-20250929-v1:0"]
  profiles = [for model in local.models : "arn:aws:bedrock:${local.region}:${data.aws_caller_identity.current.account_id}:inference-profile/eu.${model}"]
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow", Action = ["bedrock:InvokeModel"], Resource = local.profiles
      },
      {
        Effect    = "Allow", Action = ["bedrock:InvokeModel"]
        Resource  = flatten([for region in local.regions : [for model in local.models : "arn:aws:bedrock:${region}::foundation-model/${model}"]])
        Condition = { StringEquals = { "bedrock:InferenceProfileArn" = local.profiles } }
      }
    ]
  })
}
resource "aws_rolesanywhere_trust_anchor" "inference" {
  name    = "homelab-inference"
  enabled = true
  source {
    source_type = "CERTIFICATE_BUNDLE"
    source_data { x509_certificate_data = var.ca_certificate_pem }
  }
}
resource "aws_iam_role" "inference" {
  name = "homelab-inference-bedrock"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "rolesanywhere.amazonaws.com" }
      Action    = ["sts:AssumeRole", "sts:TagSession", "sts:SetSourceIdentity"]
      Condition = {
        ArnEquals = { "aws:SourceArn" = aws_rolesanywhere_trust_anchor.inference.arn }
        StringEquals = {
          "aws:SourceAccount"               = data.aws_caller_identity.current.account_id
          "aws:PrincipalTag/x509Subject/CN" = "inference-bedrock"
        }
      }
    }]
  })
  max_session_duration = 3600
}
resource "aws_iam_role_policy" "inference" {
  name   = "approved-eu-inference-only"
  role   = aws_iam_role.inference.id
  policy = local.policy
}
resource "aws_rolesanywhere_profile" "inference" {
  name             = "homelab-inference"
  enabled          = true
  duration_seconds = 3600
  role_arns        = [aws_iam_role.inference.arn]
  session_policy   = local.policy
}
resource "aws_budgets_budget" "bedrock" {
  name         = "homelab-bedrock-monthly"
  budget_type  = "COST"
  limit_amount = "100"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"
  cost_filter {
    name   = "Service"
    values = ["Amazon Bedrock"]
  }
  cost_types {
    include_credit = false
    include_refund = false
  }
  dynamic "notification" {
    for_each = length(var.notification_emails) > 0 ? toset([50, 80, 95]) : toset([])
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = var.notification_emails
    }
  }
}
output "role_arn" { value = aws_iam_role.inference.arn }
output "profile_arn" { value = aws_rolesanywhere_profile.inference.arn }
output "trust_anchor_arn" { value = aws_rolesanywhere_trust_anchor.inference.arn }
