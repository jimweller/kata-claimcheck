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

You must be logged in AWS as an administrator (or appropriate user) on your
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
   4. Retrive event from dead letter queue
   5. Verify sent and recieved messages match

## Future Considerations

These are some future considerations.

Security

- IAM constructs for the app team to access resources (roles, policies, instance profile, IRSA, PodIdentity etc.)
- Partitioned encryption model, like per user/customer, separate S3 & SQS keys
- More regulatory compliant encryptiong model, like customer provided keys or HSM

Infrastructure

- Compute service to generate presigned s3 urls or gate s3 with an IDP instead of direct access to S3 with AWS IAM
- Network perimeter infrastructure like waf, apigw, vpc links

Observability

- CloudWatch logging to obervability tool (datadog, honeycomb, etc)
- CloudWatch metrics to observability tool

Testing

- Test harness could use ephemeral ec2 instances instead of the local machine or runner
- Refactor large terratest function into more manageable functions

## Notes

- I used vanilla terraform modules from the terraform registry purposely for ease of demonstration
- I used CloudEvents in v1 when I controlled the event schema

## References

- [Terraform Modules](https://registry.terraform.io/namespaces/terraform-aws-modules)
  - [SQS](https://registry.terraform.io/modules/terraform-aws-modules/sqs/aws/latest)
  - [SNS](https://registry.terraform.io/modules/terraform-aws-modules/sns/aws/latest)
  - [KMS](https://registry.terraform.io/modules/terraform-aws-modules/kms/aws/latest)
  - [S3](https://registry.terraform.io/modules/terraform-aws-modules/s3-bucket/aws/latest)
- [Terratest](https://terratest.gruntwork.io/)
  - [AWS module](https://pkg.go.dev/github.com/gruntwork-io/terratest@v0.48.2/modules/aws)
- [CloudEvents SDK for Go](https://github.com/cloudevents/sdk-go)
- [AWS SDK for Go](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2)
