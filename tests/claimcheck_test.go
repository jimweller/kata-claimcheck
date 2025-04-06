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

	cloudevents "github.com/cloudevents/sdk-go/v2"
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

	// looad the terraform ouput into a ClaimCheckOutput struct
	tfOutputJson := terraform.OutputJson(t, terraformOptions, "claimcheck")
	var tfOutput ClaimCheckOutput
	json.Unmarshal([]byte(tfOutputJson), &tfOutput)

	region := cfg.Region
	bucket := tfOutput.ClaimcheckS3.Name
	queueURL := tfOutput.ClaimcheckSQS.URL
	topicARN := tfOutput.ClaimcheckSNS.ARN
	dlqURL := tfOutput.ClaimcheckSQSDLQ.URL

	s3Client := tt_aws.NewS3Client(t, region)
	sqsClient := tt_aws.NewSqsClient(t, region)
	snsClient := tt_aws.NewSnsClient(t, region)

	// generate 15MB file with random data. Most of the AWS streaming
	// services have limits of less than 10MB (apigw) so this is a good
	// size to test with.
	tmpFile, _ := os.CreateTemp("", "claimcheck-*.bin")
	defer os.Remove(tmpFile.Name())

	data := make([]byte, 15*1024*1024)
	rand.Read(data)
	tmpFile.Write(data)
	tmpFile.Close()

	// md5sum for verifying later
	hash := md5.Sum(data)
	localMD5 := hex.EncodeToString(hash[:])

	key := uuid.NewString()

	f, _ := os.Open(tmpFile.Name())

	outS3Put, errS3Put := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	f.Close()
	assert.Nil(t, errS3Put)
	fmt.Println("S3 PutObject:", *outS3Put.ETag)

	// create CloudEvent to use as a message
	event := cloudevents.NewEvent()
	event.SetID(key)
	event.SetSource("hhh.com/publisher")
	event.SetType("HIPAA.Record")
	event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"sender": "Honolulu Hula Hospitals",
		"key":    key,
		"md5sum": localMD5,
	})

	eventBytes, _ := json.Marshal(event)

	// publish CloudEvent to SNS
	outSns, errSns := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(topicARN),
		Message:  aws.String(string(eventBytes)),
	})
	assert.Nil(t, errSns)
	fmt.Println("SNS Publish:", *outSns.MessageId)

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
	var receivedEvent cloudevents.Event
	err = json.Unmarshal([]byte(wrapper.Message), &receivedEvent)
	assert.Nil(t, err)
	assert.Equal(t, event.ID(), receivedEvent.ID())
	assert.Equal(t, event.Type(), receivedEvent.Type())
	assert.Equal(t, event.Source(), receivedEvent.Source())

	// use the facts in the message to download the file from S3
	var payload struct {
		Key    string `json:"key"`
		MD5Sum string `json:"md5sum"`
	}
	receivedEvent.DataAs(&payload)

	outS3Get, errS3Get := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(payload.Key),
	})
	assert.Nil(t, errS3Get)
	fmt.Println("S3 GetObject:", *outS3Get.ETag)

	downloaded, _ := io.ReadAll(outS3Get.Body)
	outS3Get.Body.Close()

	// compare the md5sum of the downloaded file to the original file
	downloadHash := md5.Sum(downloaded)
	downloadMD5 := hex.EncodeToString(downloadHash[:])

	assert.Equal(t, payload.MD5Sum, downloadMD5)

	//
	// DLQ test
	// This test will send a message to the SNS topic that will fail processing
	// and be sent to the DLQ. The message will be a simple string
	// that will be sent to the SQS queue. The SQS queue has a max_receive_count
	// of 1 and a visibility_timeout_seconds of 1. So, we rertrieve, don't delete,
	// and wait for the message to be sent back tot he queue. The second retreive
	// exceeds the max_receive_count, causes a redrive, and the message is sent to
	// the DLQ.
	//

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
		WaitTimeSeconds:     10,
	})
	assert.Nil(t, errDlqSqs)
	assert.Len(t, outDlqSqs.Messages, 1)

	// DO NOT delete — let it be returned to the queue
	time.Sleep(5 * time.Second) // allow retry and DLQ redrive

	// Try to retrieve the message again, this will exceed the max_receive_count
	// and cause a redrive which will send the message to the DLQ
	sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     10,
	})
	assert.Nil(t, errDlqSqs)

	// Retrieve the message from the DLQ
	dlqOut, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(dlqURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     10,
	})
	assert.Nil(t, err)

	// unwrap the aws envelope to get our original message
	var dlqEnvelope struct {
		Message string `json:"Message"`
	}

	// see tha the message in the DLQ is the same as what we sent
	err = json.Unmarshal([]byte(*dlqOut.Messages[0].Body), &dlqEnvelope)
	assert.Nil(t, err)
	assert.Equal(t, dlqMsg, dlqEnvelope.Message)

}
