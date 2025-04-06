# 🥋Kata: Claim Check Pattern Terraform Module v2

## Overview

This is a terraform module that creates the AWS infrastructure necessary for an
application to use the claim check messaging pattern (Gregor Hohpe and Bobby
Woolf. 2003. Enterprise Integration Patterns​).

> Note: This is v2 (Blissful Builder) of the solution (See [v1 Affable
> Achitect](https://github.com/jimweller/kata-claimcheck/tree/v1)). The
> difference between v1 and v2 is that in v1 the producer provided the event and
> the payload separately. In v2, the producer provides only the payload and the
> storage (S3) initiates the messaging flow.

![architecture diagram](diagram.drawio.svg)

## Usage

### Example

```terraform
module "claimcheck" {
  source     = "../modules/claimcheck"
  queue_name = "claimcheck"

  # OPTIONAL PARAMETERS
  max_receive_count          = 5
  visibility_timeout_seconds = 600    // 10 minutes
  message_retention_seconds  = 604800 // 7 days

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
```

### Prerequisites

You must be logged in AWS as an administrator (or appropriately permissioned user) on your
terminal before running commands.

Required software and the versions used for testing.

- OpenTofu (1.9.0)
- Go (1.24.2)
- AWS CLI (2.25.11) [optional]

### Testing

Running Tests

```bash
cd tests
go test
```

Successful test

```bash
Waiting for SNS -> SQS subscription to become active...
SNS -> SQS subscription is active after 0s
Sleep 10s for the S3->SNS TestEvent to propagate to SQS

(purging queues with 10s sleep)

TESTING E2E
--------------------
S3 PutObject: "99bdc8d22a271436de6ba5e3003bdf80"
Sleep 10s for messages to propogate to SQS
SQS ReceiveMessage: a3fe6b47-3036-4b9e-bedc-3b0907ef1cb2
S3 GetObject: "99bdc8d22a271436de6ba5e3003bdf80"

buckets: claimcheck-s3-31ec9362 claimcheck-s3-31ec9362
keys: d84dbb3d-e212-4654-b9ca-1fac3af5ddc6 d84dbb3d-e212-4654-b9ca-1fac3af5ddc6
MD5sums: 105e1100cfe112808970758d9c1629ba 105e1100cfe112808970758d9c1629ba


(purging queues with 10s sleep)

TESTING DLQ
--------------------
SNS Publish: 552bc6d2-a128-5cea-8fc8-a6be5c84039f
SQS Retrieve: 8b9728e3-c432-4657-9619-b74c5c3814f4
Sleep 10s for visibility timeout to expire
DLQ Retrieve: 8b9728e3-c432-4657-9619-b74c5c3814f4
DLQ messages: dlq-test-df00c89d-c9b2-46ba-9ba4-00bcc7077556 dlq-test-df00c89d-c9b2-46ba-9ba4-00bcc7077556

[...]

PASS
ok      kata-claimcheck/tests   346.519s
```

## Constraints

- Payloads are sensitive data (PII/HIPAA)
- Identity provider is already implemented by the app team
- Requestor does not get a response
- Processing payloads can take several minutes

## Requirements

- Encryption in flight and at rest because data is sensitive (PII/HIPAA)
- Payloads are of arbitrary size

## Assumptions

- The excercise does not require me to include an application. The module is tested with terratest.
- The app team handles separating the metadata from the payload
- The module should include the KMS key for encryption
- Multiple SDLC environments will be handled outside of the module (terragrunt, spacelift, etc.)
- Terraform state management is handled outside the module by the app team, by terragrunt or by a runner
- OpenTofu is the preferred IaC tool
- Dead letter queue should be included by default
- SNS is used for future proofing like for fanout. SNS -> SQS is free.

## Tests

1. End to End
   1. Upload file to S3
   2. Retrieve event from SQS
   3. Download file from S3
   4. Verify upload and download match
2. Dead Letter Queue
   1. Send event to SNS
   2. Retrieve message but do not delete
   3. Retrieve again to force redrive to dead letter queue
   4. Retrieve event from dead letter queue
   5. Verify sent and recieved messages match

## Future Considerations

These are some future considerations.

Security

- IAM constructs for the app team to access resources (roles, policies, instance profile, IRSA, PodIdentity etc.)
- Partitioned encryption model, like per user/customer, separate S3 & SQS keys
- More regulatory compliant encryption model, like customer provided keys or HSM

Infrastructure

- Compute service to generate presigned s3 urls or gate s3 with an IDP instead of direct access to S3 with AWS IAM
- Network perimeter infrastructure like waf, apigw, vpc links

Observability

- CloudWatch logging to obervability tool (datadog, honeycomb, etc)
- CloudWatch metrics to observability tool

Testing

- More robust verification that resources exist and are configured vs. just sleeping for 20s and firing e2e tests
- Test harness could use ephemeral compute or a runner instead of the local machine
- Refactor large terratest function into more manageable functions
- Look for other edge cases like race conditions for infra to propogate or async message delays

## Notes

- I used vanilla terraform modules from the terraform registry purposely for ease of demonstration
- I used CloudEvents in v1 when I controlled the event schema
- Terratest is awesome! My code might be newb-ugly though 😜
- Lots of weird depends_on, polling, and sleeping for edge cases around timing

## References

- [Claim Check Pattern](https://www.enterpriseintegrationpatterns.com/patterns/messaging/StoreInLibrary.html)
- [Terraform Modules](https://registry.terraform.io/namespaces/terraform-aws-modules)
  - [SQS](https://registry.terraform.io/modules/terraform-aws-modules/sqs/aws/latest)
  - [SNS](https://registry.terraform.io/modules/terraform-aws-modules/sns/aws/latest)
  - [KMS](https://registry.terraform.io/modules/terraform-aws-modules/kms/aws/latest)
  - [S3](https://registry.terraform.io/modules/terraform-aws-modules/s3-bucket/aws/latest)
- [Terratest](https://terratest.gruntwork.io/)
  - [Terraform module](https://pkg.go.dev/github.com/gruntwork-io/terratest@v0.48.2/modules/terraform)
  - [AWS module](https://pkg.go.dev/github.com/gruntwork-io/terratest@v0.48.2/modules/aws)
- [CloudEvents SDK for Go](https://github.com/cloudevents/sdk-go)
- [AWS SDK for Go](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2)
- [Kata v1 Affable Achitect](https://github.com/jimweller/kata-claimcheck/tree/v1)
