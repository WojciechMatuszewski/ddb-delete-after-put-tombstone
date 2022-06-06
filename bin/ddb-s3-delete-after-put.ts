#!/usr/bin/env node
import "source-map-support/register";
import * as cdk from "aws-cdk-lib";
import { DdbS3DeleteAfterPutStack } from "../lib/ddb-s3-delete-after-put-stack";
import { Aspects, CfnResource, IAspect } from "aws-cdk-lib";
import { IConstruct } from "constructs";

const app = new cdk.App();
new DdbS3DeleteAfterPutStack(app, "DdbS3DeleteAfterPutStack", {
  synthesizer: new cdk.DefaultStackSynthesizer({
    qualifier: "putdel"
  })
});

export class DeletionPolicySetter implements IAspect {
  constructor() {}
  visit(node: IConstruct): void {
    /**
     * Nothing stops you from adding more conditions here.
     */
    if (node instanceof CfnResource) {
      node.applyRemovalPolicy(cdk.RemovalPolicy.DESTROY);
    }
  }
}

Aspects.of(app).add(new DeletionPolicySetter());
