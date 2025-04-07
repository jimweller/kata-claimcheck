// Terrform module to create a messaging system  using claimcheck pattern using
// SQS, SNS and S3. Everything is encrypted using KMS customer managed key.

module "claimcheck_kms" {
  source  = "terraform-aws-modules/kms/aws"
  version = "~> 3.1.0"

  description = "${var.queue_name}-kms"

  // Sns needs to use the key to push to sqs. Sqs needs to use the key to push
  // to the DLQ. Note: SQS does not have an aws:SourceArn to use in a conditon
  key_statements = [
    {
      sid = "AllowSNSServiceToUseKey"
      actions = [
        "kms:GenerateDataKey",
        "kms:Decrypt"
      ]
      principals = [
        {
          type        = "Service"
          identifiers = ["sns.amazonaws.com"]
        }
      ]
      resources = ["*"]
      conditions = [
        {
          test     = "StringEquals"
          variable = "aws:SourceArn"
          values   = [module.claimcheck_sns.topic_arn]
        }
      ]
    },
    {
      sid = "AllowSQSServiceToUseKey"
      actions = [
        "kms:GenerateDataKey",
        "kms:Decrypt"
      ]
      principals = [
        {
          type        = "Service"
          identifiers = ["sqs.amazonaws.com"]
        }
      ]
      resources  = ["*"]
      conditions = [] # no SourceArn condition for redrive
    }
  ]

  tags = merge(var.tags)
}


module "claimcheck_sqs" {
  source  = "terraform-aws-modules/sqs/aws"
  version = "~> 4.3.0"

  name = "${var.queue_name}-sqs"

  message_retention_seconds  = var.message_retention_seconds
  visibility_timeout_seconds = var.visibility_timeout_seconds

  kms_master_key_id     = module.claimcheck_kms.key_id
  dlq_kms_master_key_id = module.claimcheck_kms.key_id

  create_dlq = true
  dlq_name   = "${var.queue_name}-sqs-dlq"

  redrive_policy = {
    maxReceiveCount = var.max_receive_count
  }

  create_queue_policy = true

  queue_policy_statements = {
    sns = {

      sid     = "SNSPublish"
      actions = ["sqs:SendMessage"]
      principals = [{
        type        = "AWS"
        identifiers = ["*"]
      }]

      conditions = [{
        test     = "ArnEquals"
        variable = "aws:SourceArn"
        values   = [module.claimcheck_sns.topic_arn]
      }]
    }
  }

  tags     = merge(var.tags)
  dlq_tags = merge(var.tags)
}


module "claimcheck_sns" {
  source  = "terraform-aws-modules/sns/aws"
  version = "~> 6.1.0"

  name = "${var.queue_name}-sns"

  topic_policy_statements = {
    sqs = {
      sid = "SQSSubscribe"
      actions = [
        "sns:Subscribe",
        "sns:Receive",
      ]

      principals = [{
        type        = "AWS"
        identifiers = ["*"]
      }]

      conditions = [{
        test     = "StringLike"
        variable = "sns:Endpoint"
        values   = [module.claimcheck_sqs.queue_arn]
      }]
    }

    s3 = {
      sid     = "AllowS3"
      actions = ["sns:Publish"]
      principals = [{
        type        = "Service"
        identifiers = ["s3.amazonaws.com"]
      }]
      conditions = [{
        test     = "ArnLike"
        variable = "aws:SourceArn"
        values   = [module.claimcheck_s3.s3_bucket_arn]
      }]
    }

  }

  subscriptions = {
    sqs = {
      protocol = "sqs"
      endpoint = module.claimcheck_sqs.queue_arn
    }
  }

  tags = merge(var.tags)
}

resource "random_id" "s3_suffix" {
  byte_length = 4
}

module "claimcheck_s3" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 4.6.0"

  bucket = "${var.queue_name}-s3-${random_id.s3_suffix.hex}"

  force_destroy = true

  server_side_encryption_configuration = {
    rule = {
      apply_server_side_encryption_by_default = {
        kms_master_key_id = module.claimcheck_kms.key_id
        sse_algorithm     = "aws:kms"
      }
    }
  }

  tags = merge(var.tags)
}


resource "aws_s3_bucket_notification" "s3_to_sns" {
  bucket = module.claimcheck_s3.s3_bucket_id

  topic {
    topic_arn = module.claimcheck_sns.topic_arn
    events    = ["s3:ObjectCreated:Put"]
  }

  depends_on = [module.claimcheck_sns, module.claimcheck_sqs]

}
