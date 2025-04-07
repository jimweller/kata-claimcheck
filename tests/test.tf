// Simple instance of the claimcheck module
// to use with terratest. Note the aggressive
// recieve count and visibility timeout for 
// testing DLQ.

module "claimcheck" {
  source     = "../modules/claimcheck"
  queue_name = "claimcheck"

  # OPTIONAL PARAMETERS
  max_receive_count          = 1
  visibility_timeout_seconds = 1
  message_retention_seconds  = 300
  sdlc_env = local.sdlc_env

  tags = {
    Company     = "Clinical Trials Inc."
    Department  = "Medical Machine Learning"
    CostAccount = "54321"
    CostCenter  = "123456"
    Environment = local.sdlc_env
    Runbook     = "https://example.com/runbook"
    Owner       = "John Doe"
  }
}

locals {
  sdlc_env = "dev"
}


output "claimcheck" {
  value = module.claimcheck
}



terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  required_version = "~> 1.0"
}

