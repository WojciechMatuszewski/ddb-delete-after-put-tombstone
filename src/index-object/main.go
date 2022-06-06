package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	dynamodbattributevalue "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
	lambda.Start(handler)
}

type Detail struct {
	Version string `json:"version"`
	Bucket  struct {
		Name string `json:"name"`
	} `json:"bucket"`
	Object struct {
		Key       string `json:"key"`
		Size      int    `json:"size"`
		Etag      string `json:"etag"`
		VersionID string `json:"version-id"`
		Sequencer string `json:"sequencer"`
	} `json:"object"`
	RequestID       string `json:"request-id"`
	Requester       string `json:"requester"`
	SourceIPAddress string `json:"source-ip-address"`
	Reason          string `json:"reason"`
}

type Item struct {
	PK string `json:"PK" dynamodbav:"PK"`
	SK string `json:"SK" dynamodbav:"SK"`
}

func handler(ctx context.Context, event events.CloudWatchEvent) error {
	eventDetailBuf, err := event.Detail.MarshalJSON()
	if err != nil {
		return err
	}
	var eventDetail Detail
	err = json.Unmarshal(eventDetailBuf, &eventDetail)
	if err != nil {
		return err
	}

	fmt.Printf("Having the event for %v", eventDetail)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	ddb := dynamodb.NewFromConfig(cfg)
	tableName := os.Getenv("FILES_TABLE_NAME")
	if tableName == "" {
		return errors.New("FILES_TABLE_NAME is not set")
	}

	item := Item{
		PK: fmt.Sprintf("OBJECT#%s", eventDetail.Object.Key),
		SK: fmt.Sprintf("OBJECT#%s", eventDetail.Object.Key),
	}
	itemav, err := dynamodbattributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	tombstone := Item{
		PK: fmt.Sprintf("OBJECT#%s", eventDetail.Object.Key),
		SK: fmt.Sprintf("TOMBSTONE#%s", eventDetail.Object.Key),
	}
	tombstoneav, err := dynamodbattributevalue.MarshalMap(tombstone)
	if err != nil {
		return err
	}

	fmt.Printf("Putting %v", itemav)

	_, err = ddb.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []dynamodbtypes.TransactWriteItem{
			{
				ConditionCheck: &dynamodbtypes.ConditionCheck{
					TableName:           aws.String(tableName),
					ConditionExpression: aws.String("attribute_not_exists(#SK)"),
					Key:                 tombstoneav,
					ExpressionAttributeNames: map[string]string{
						"#SK": "SK",
					},
				},
			},
			{
				Put: &dynamodbtypes.Put{
					Item:      itemav,
					TableName: aws.String(tableName),
				},
			},
		},
	})
	if err != nil {
		var transactionCanceledErr *dynamodbtypes.TransactionCanceledException
		if !errors.As(err, &transactionCanceledErr) {
			fmt.Println("not transaction canceled error")
			return err
		}

		if isTombstonePresent(transactionCanceledErr) {
			fmt.Println("Tombstone detected, skipping")
			return nil
		}

		errMsg := strings.Join(getCancellationReasons(transactionCanceledErr), ", ")
		fmt.Println("errors", errMsg)
		return err

	}

	return nil
}

func isTombstonePresent(err *dynamodbtypes.TransactionCanceledException) bool {
	reasons := err.CancellationReasons
	return reasons[0].Message == nil && reasons[1].Message != nil
}

func getCancellationReasons(err *dynamodbtypes.TransactionCanceledException) []string {
	var reasons []string
	for _, reason := range err.CancellationReasons {
		if reason.Message != nil {
			reasons = append(reasons, *reason.Message)
		}

	}
	return reasons
}
