# 🥋Kata: Claim Check Pattern Terraform Module

## Overview

This is a terraform module that creates the AWS infrastructure necessary for an
application to use the claim check messaging pattern (Gregor Hohpe and Bobby Woolf. 2003. Enterprise Integration Patterns​).

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
- Payload is of arbitrary size. Assume larger than most off-the-shelf messaging systems.
- IDP is already defined by the module's consumer
- Requestor does not get a response
- Processing can take several minutes

## Requirements

- Encryption in flight and at rest because (PII/HIPAA)
- Arbitrary payload sizes. Assume larger than 10MB

## Assumptions

- The app team will upload the payload and send the message. It is a possibility the expectation is to upload a complete package including metadata and payload that needs to be split. In which, case there would be some event emitted from S3 and processing to split the meatadata from the payload. For example the singe bundle could be a zip file with json and binary files. Or it could be a json with the payload base64 encoded.
- The module should include the KMS key for encryption
- The excercise does not require me to include an application. The module is tested with terratest.
- Multiple SDLC environments will be handled outside of the module with something like terragrunt or SDLC progression in a runner (github actions, spacelift, etc.). This can be simulated with the terratest by changing AWS accounts in the terminal.
- Terraform state management is handled by the consumer or the runner
- OpenTofu is the preferred IaC tool
- CloudEvents will be used for message structure
- Dead letter queue should be included by default

## Tests

1. End to End
   1. Upload file to S3
   2. Send event to SNS
   3. Retrieve event from SQS
   4. Verify sent and received events match
   5. Download file from S3
   6. Verify uploaded and downloaded files match
2. Dead Letter Queue
   1. Send event to SNS
   2. Retrieve message but do not delete
   3. Retrieve again to force redrive to dead letter queue
   4. Retrive event from dead letter queue
   5. Verify sent and recieved events match

## Future Considerations

These are some future considerations.

- Network perimeter infrastructure like waf, apigw, vpc links
- Testing to use ephemeral ec2 instances instead of the local machine or runner
- IAM constructs to access resources (roles, policies, instance profile, IRSA, PodIdentity etc.)
- AWS Glue schema registry for CloudEvent structure definition and validation
- Partitioned encryption model, like per user/customer, separate S3 & SQS keys
- More regulatory compliant encryptiong model, like customer provided keys or HSM
- Compute service to generate presigned s3 urls or gate s3 with an IDP instead of direct access to S3 with AWS IAM
- CloudWatch logging to obervability tool (datadog, honeycomb, etc)
- CloudWatch metrics to observability tool
- OpenTelemetry to observability tool, including trace/span IDs that follow CloudEvents
- Refactor large terratest function into more manageable functions

## Notes

- I used vanilla terraform modules from the terraform registry purposely for ease of demonstration

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
