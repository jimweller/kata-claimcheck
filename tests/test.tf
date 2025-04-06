// Simple instance of the claimcheck module
// to use with terratest. Note the aggressive
// for recieve count and visibility timeout.

module "claimcheck" {
  source     = "../modules/claimcheck"
  queue_name = "claimcheck"

  # OPTIONAL PARAMETERS
  max_receive_count          = 1
  visibility_timeout_seconds = 1
  message_retention_seconds  = 300

  tags = {
    Company     = "Clinical Trials Inc."
    Department  = "Medical Machine Learning"
    CostAccount = "54321"
    CostCenter  = "123456"
    Environment = "dev"
    Runbook     = "https://example.com/runbook"
    Owner       = "John Doe"
  }
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

