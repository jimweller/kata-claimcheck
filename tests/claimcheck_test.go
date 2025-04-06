/*
Terratest for claimcheck

Test package for a terraform stack that implements a claimcheck pattern using
AWS services. This test uploads a file to S3, publishes a CloudEvent to SNS,
and verifies the message is received in SQS. It also tests the DLQ
functionality by sending a message that will fail processing.
*/
package test

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tt_aws "github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	uuid "github.com/google/uuid"
)

// ClaimCheckOutput mathces the output of the terraform stack for marshalling from JSON
type ClaimCheckOutput struct {
	ClaimcheckKMS struct {
		ARN string `json:"arn"`
		ID  string `json:"id"`
	} `json:"claimcheck_kms"`
	ClaimcheckS3 struct {
		ARN  string `json:"arn"`
		Name string `json:"name"`
	} `json:"claimcheck_s3"`
	ClaimcheckSQS struct {
		ARN  string `json:"arn"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"claimcheck_sqs"`
	ClaimcheckSQSDLQ struct {
		ARN  string `json:"arn"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"claimcheck_sqs_dlq"`
	ClaimcheckSNS struct {
		Topic string `json:"topic"`
		ARN   string `json:"arn"`
	} `json:"claimcheck_sns"`
}

func TestTerraformClaimcheck(t *testing.T) {

	ctx := context.TODO()
	cfg, _ := config.LoadDefaultConfig(ctx)

	// assume opentofu over terraform
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir:    ".",
		TerraformBinary: "tofu",
	})
	terraform.InitAndApply(t, terraformOptions)

	// cleanup at the end
	defer terraform.Destroy(t, terraformOptions)

	// load the terraform ouput into a ClaimCheckOutput struct
	tfOutputJson := terraform.OutputJson(t, terraformOptions, "claimcheck")
	var tfOutput ClaimCheckOutput
	json.Unmarshal([]byte(tfOutputJson), &tfOutput)

	region := cfg.Region
	uploadBucket := tfOutput.ClaimcheckS3.Name
	queueURL := tfOutput.ClaimcheckSQS.URL
	topicARN := tfOutput.ClaimcheckSNS.ARN
	dlqURL := tfOutput.ClaimcheckSQSDLQ.URL

	s3Client := tt_aws.NewS3Client(t, region)
	sqsClient := tt_aws.NewSqsClient(t, region)
	snsClient := tt_aws.NewSnsClient(t, region)

	defer sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(tfOutput.ClaimcheckSQS.URL),
	})
	defer sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(tfOutput.ClaimcheckSQSDLQ.URL),
	})

	// The SNS -> SQS subscription was not spinning up within a reasonable sleep
	// time, even with depends_on in TF. We'll poll for an active subscription.
	fmt.Println("Waiting for SNS -> SQS subscription to become active...")

	found := false
	for i := 0; i < 60; i++ {
		resp, err := snsClient.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
			TopicArn: aws.String(tfOutput.ClaimcheckSNS.ARN),
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, sub := range resp.Subscriptions {
			if *sub.Endpoint == tfOutput.ClaimcheckSQS.ARN && *sub.SubscriptionArn != "PendingConfirmation" {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("SNS -> SQS subscription is active after %ds\n", i)
			break
		}
		time.Sleep(3 * time.Second)
	}
	if !found {
		t.Fatal("SNS -> SQS subscription did not become active in time")
	}

	fmt.Println("Sleep 10s for the S3->SNS TestEvent to propagate to SQS")
	time.Sleep(10 * time.Second) // allow retry and DLQ redrive

	// Purge queues before beginning testing
	fmt.Printf("\n(purging queues with 10s sleep)\n\n")
	_, _ = sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(tfOutput.ClaimcheckSQS.URL),
	})
	_, _ = sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(tfOutput.ClaimcheckSQSDLQ.URL),
	})
	time.Sleep(10 * time.Second)

	//
	// E2E Test
	// Upload a file to s3, get subsequent message from SQS, download the file
	// described in the event message and compare upload to download
	//

	fmt.Println("TESTING E2E")
	fmt.Println("--------------------")

	// generate 15-20MB file with random data. Most of the AWS streaming
	// services have limits of less than 10MB (apigw) so this is a good
	// size to test with.
	tmpFile, _ := os.CreateTemp("", "claimcheck-*.bin")
	defer os.Remove(tmpFile.Name())

	size := 15 + mathrand.Intn(6) // 15–20
	data := make([]byte, size*1024*1024)

	rand.Read(data)
	tmpFile.Write(data)
	tmpFile.Close()

	// md5sum for verifying later
	hash := md5.Sum(data)
	uploadMD5 := hex.EncodeToString(hash[:])

	uploadKey := uuid.NewString()

	f, _ := os.Open(tmpFile.Name())

	outS3Put, errS3Put := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(uploadBucket),
		Key:    aws.String(uploadKey),
		Body:   f,
	})
	f.Close()
	assert.Nil(t, errS3Put)
	fmt.Println("S3 PutObject:", *outS3Put.ETag)

	fmt.Println("Sleep 10s for messages to propogate to SQS")
	time.Sleep(10 * time.Second) // allow retry and DLQ redrive

	// retrieve CloudEvent from SQS
	outSqsRcv, errSqsRcv := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
	})
	assert.Nil(t, errSqsRcv)
	fmt.Println("SQS ReceiveMessage:", *outSqsRcv.Messages[0].MessageId)

	// remove the retrieved message from SQS queue
	sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &tfOutput.ClaimcheckSQS.URL,
		ReceiptHandle: outSqsRcv.Messages[0].ReceiptHandle,
	})

	// Unwrap the aws envelope to get our original message
	var wrapper struct {
		Message string `json:"Message"`
	}
	err := json.Unmarshal([]byte(*outSqsRcv.Messages[0].Body), &wrapper)
	assert.Nil(t, err)

	// Compare what we sent to what we received

	var payload struct {
		Records []struct {
			S3 struct {
				Bucket struct {
					Name string `json:"name"`
				} `json:"bucket"`
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		} `json:"Records"`
	}

	err = json.Unmarshal([]byte(wrapper.Message), &payload)
	assert.Nil(t, err)

	downloadBucket := payload.Records[0].S3.Bucket.Name
	downloadKey := payload.Records[0].S3.Object.Key

	assert.Equal(t, downloadBucket, uploadBucket)
	assert.Equal(t, downloadKey, uploadKey)

	// use the facts in the message to download the file from S3

	outS3Get, errS3Get := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(downloadBucket),
		Key:    aws.String(downloadKey),
	})
	assert.Nil(t, errS3Get)
	fmt.Println("S3 GetObject:", *outS3Get.ETag)

	downloaded, _ := io.ReadAll(outS3Get.Body)
	outS3Get.Body.Close()

	// compare the md5sum of the downloaded file to the original file
	downloadHash := md5.Sum(downloaded)
	downloadMD5 := hex.EncodeToString(downloadHash[:])

	assert.Equal(t, uploadMD5, downloadMD5)

	fmt.Printf("\nbuckets: %s %s\n", uploadBucket, downloadBucket)
	fmt.Printf("keys: %s %s\n", uploadKey, downloadKey)
	fmt.Printf("MD5sums: %s %s\n\n", uploadMD5, downloadMD5)

	//
	// DLQ test
	// This test will send a message to the SNS topic that will fail processing
	// and be sent to the DLQ. The message will be a simple string
	// that will be sent to the SQS queue. The SQS queue has a max_receive_count
	// of 1 and a visibility_timeout_seconds of 1. So, we retrieve, don't delete,
	// and wait for the message to be sent back to the queue. The second retreive
	// exceeds the max_receive_count, causes a redrive, and the message is sent to
	// the DLQ.
	//

	// Purge queues before beginning testing
	fmt.Printf("\n(purging queues with 10s sleep)\n\n")
	_, _ = sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(tfOutput.ClaimcheckSQS.URL),
	})
	_, _ = sqsClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(tfOutput.ClaimcheckSQSDLQ.URL),
	})
	time.Sleep(10 * time.Second)

	fmt.Println("TESTING DLQ")
	fmt.Println("--------------------")

	dlqMsg := fmt.Sprintf("dlq-test-%s", uuid.NewString())

	// publish a message to the SNS topic
	outDlqSns, errDlqSns := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(topicARN),
		Message:  aws.String(dlqMsg),
	})
	assert.Nil(t, errDlqSns)
	fmt.Println("SNS Publish:", *outDlqSns.MessageId)

	// retrieve the message from SQS
	outDlqSqs, errDlqSqs := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
	})
	assert.Nil(t, errDlqSqs)
	fmt.Println("SQS Retrieve:", *outDlqSqs.Messages[0].MessageId)

	// DO NOT delete — let it be returned to the queue
	fmt.Println("Sleep 10s for visibility timeout to expire")
	time.Sleep(10 * time.Second)

	// Try to retrieve the message again, this will exceed the max_receive_count
	// and cause a redrive which will send the message to the DLQ
	outDlqSqs, errDlqSqs = sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
	})
	assert.Nil(t, errDlqSqs)

	// Retrieve the message from the DLQ
	dlqOut, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(dlqURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
	})
	assert.Nil(t, err)
	fmt.Println("DLQ Retrieve:", *dlqOut.Messages[0].MessageId)

	// unwrap the aws envelope to get our original message
	var dlqEnvelope struct {
		Message string `json:"Message"`
	}

	// see that the message in the DLQ is the same as what we sent
	err = json.Unmarshal([]byte(*dlqOut.Messages[0].Body), &dlqEnvelope)
	assert.Nil(t, err)

	fmt.Printf("DLQ messages: %s %s\n", dlqMsg, dlqEnvelope.Message)

	assert.Equal(t, dlqMsg, dlqEnvelope.Message)

}
