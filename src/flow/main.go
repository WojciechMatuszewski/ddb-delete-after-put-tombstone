package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type DeleterPayload struct {
	Key string `json:"key"`
}

func main() {
	const bucketName = "xx"
	const deleterFunctionName = "xx"

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	s3Client := s3.NewFromConfig(cfg)

	r := bytes.NewReader([]byte("hello world"))

	now := time.Now().Format(time.RFC3339)
	key := fmt.Sprintf("%s.txt", now)
	fmt.Printf("Uploading %s to %s\n", key, bucketName)

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   r,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("File uploaded")

	fmt.Println("Invoking the deleter function")

	lambdaClient := lambda.NewFromConfig(cfg)

	payload := DeleterPayload{Key: key}
	payloadBuf, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	out, err := lambdaClient.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(deleterFunctionName),
		Payload:      payloadBuf,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Deleter returned with %v", string(out.Payload))
}
