package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

type Body struct {
	Key string `json:"key"`
}

type DeleteKey struct {
	PK string `json:"PK" dynamodbav:"PK"`
	SK string `json:"SK" dynamodbav:"SK"`
}

type Tombstone struct {
	PK string `json:"PK" dynamodbav:"PK"`
	SK string `json:"SK" dynamodbav:"SK"`
}

func handler(ctx context.Context, body Body) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("Received event with a body of: %v\n", body)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	ddb := dynamodb.NewFromConfig(cfg)
	tableName := os.Getenv("FILES_TABLE_NAME")
	if tableName == "" {
		return respond(http.StatusInternalServerError, "FILES_TABLE_NAME is not set")
	}

	deleteKey := DeleteKey{
		PK: fmt.Sprintf("OBJECT#%s", body.Key),
		SK: fmt.Sprintf("OBJECT#%s", body.Key),
	}
	deleteKeyAvs, err := dynamodbattributevalue.MarshalMap(deleteKey)
	if err != nil {
		return respond(http.StatusInternalServerError, err.Error())
	}

	tombstone := Tombstone{
		PK: fmt.Sprintf("OBJECT#%s", body.Key),
		SK: fmt.Sprintf("TOMBSTONE#%s", body.Key),
	}
	tombstoneAvs, err := dynamodbattributevalue.MarshalMap(tombstone)
	if err != nil {
		return respond(http.StatusInternalServerError, err.Error())
	}

	_, err = ddb.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []dynamodbtypes.TransactWriteItem{
			{
				Delete: &dynamodbtypes.Delete{
					TableName: aws.String(tableName),
					Key:       deleteKeyAvs,
				},
			},
			{
				Put: &dynamodbtypes.Put{
					TableName: aws.String(tableName),
					Item:      tombstoneAvs,
				},
			},
		},
	})
	if err != nil {
		var canceledErr *dynamodbtypes.TransactionCanceledException
		if errors.As(err, &canceledErr) {
			msg := strings.Join(getCancellationReasons(canceledErr), ", ")
			return respond(http.StatusConflict, msg)
		}

		var conflictErr *dynamodbtypes.TransactionConflictException
		if errors.As(err, &conflictErr) {
			return respond(http.StatusConflict, conflictErr.ErrorMessage())
		}

		return respond(http.StatusInternalServerError, err.Error())
	}

	return respond(http.StatusOK, http.StatusText(http.StatusOK))
}

func respond(statusCode int, message string) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       message,
	}, nil
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
