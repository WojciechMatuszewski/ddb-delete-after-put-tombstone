import {
  aws_dynamodb,
  aws_s3,
  Stack,
  StackProps,
  aws_events,
  RemovalPolicy,
  CfnOutput,
  aws_events_targets,
  aws_iam
} from "aws-cdk-lib";
import * as aws_lambda_go from "@aws-cdk/aws-lambda-go-alpha";
import { Construct } from "constructs";
import { join } from "path";

export class DdbS3DeleteAfterPutStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const filesBucket = new aws_s3.Bucket(this, "FilesBucket", {
      autoDeleteObjects: true,
      removalPolicy: RemovalPolicy.DESTROY,
      eventBridgeEnabled: true,
      objectOwnership: aws_s3.ObjectOwnership.BUCKET_OWNER_ENFORCED
    });
    new CfnOutput(this, "FilesBucketName", {
      value: filesBucket.bucketName
    });

    const filesTable = new aws_dynamodb.Table(this, "FilesTable", {
      billingMode: aws_dynamodb.BillingMode.PAY_PER_REQUEST,
      partitionKey: {
        name: "PK",
        type: aws_dynamodb.AttributeType.STRING
      },
      sortKey: {
        name: "SK",
        type: aws_dynamodb.AttributeType.STRING
      },
      stream: aws_dynamodb.StreamViewType.NEW_AND_OLD_IMAGES
    });
    new CfnOutput(this, "FilesBucketTableName", {
      value: filesTable.tableArn
    });

    const fileCreatedRule = new aws_events.Rule(this, "FileCreatedRule", {
      eventPattern: {
        source: ["aws.s3"],
        detailType: ["Object Created"],
        detail: {
          bucket: {
            name: [filesBucket.bucketName]
          }
        }
      }
    });

    const filesCreatedRuleProcessorFunction = new aws_lambda_go.GoFunction(
      this,
      "FilesCreatedRuleProcessorFunction",
      {
        entry: join(__dirname, "../src/index-object"),
        environment: {
          FILES_TABLE_NAME: filesTable.tableName
        }
      }
    );
    new CfnOutput(this, "FilesCreatedRuleProcessorFunctionName", {
      value: filesCreatedRuleProcessorFunction.functionName
    });

    filesTable.grantWriteData(filesCreatedRuleProcessorFunction);
    filesCreatedRuleProcessorFunction.addToRolePolicy(
      new aws_iam.PolicyStatement({
        actions: ["dynamodb:ConditionCheckItem"],
        resources: [filesTable.tableArn]
      })
    );
    fileCreatedRule.addTarget(
      new aws_events_targets.LambdaFunction(filesCreatedRuleProcessorFunction, {
        retryAttempts: 0
      })
    );

    const deleterFunction = new aws_lambda_go.GoFunction(
      this,
      "DeleterFunction",
      {
        entry: join(__dirname, "../src/delete-object-index"),
        environment: {
          FILES_TABLE_NAME: filesTable.tableName
        },
        retryAttempts: 0
      }
    );
    new CfnOutput(this, "DeleterFunctionName", {
      value: deleterFunction.functionName
    });

    filesTable.grantWriteData(deleterFunction);
  }
}
