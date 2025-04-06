output "claimcheck_sqs" {
  description = "Properties of the main SQS queue created."
  value = {
    "name" = module.claimcheck_sqs.queue_name
    "arn"  = module.claimcheck_sqs.queue_arn
    "url"  = module.claimcheck_sqs.queue_url
  }
}

output "claimcheck_sqs_dlq" {
  description = "Properties of the DLQ SQS queue created."
  value = {
    "name" = module.claimcheck_sqs.dead_letter_queue_name
    "arn"  = module.claimcheck_sqs.dead_letter_queue_arn
    "url"  = module.claimcheck_sqs.dead_letter_queue_url
  }
}

output "claimcheck_kms" {
  description = "Properties of the KMS key created."
  value = {
    "id"  = module.claimcheck_kms.key_id
    "arn" = module.claimcheck_kms.key_arn
  }
}

output "claimcheck_s3" {
  description = "Properties of the s3 bucket created."
  value = {
    "name" = module.claimcheck_s3.s3_bucket_id
    "arn"  = module.claimcheck_s3.s3_bucket_arn
  }
}


output "claimcheck_sns" {
  description = "Properties of the SNS topic created."
  value = {
    "topic" = module.claimcheck_sns.topic_name
    "arn"   = module.claimcheck_sns.topic_arn
  }
}
