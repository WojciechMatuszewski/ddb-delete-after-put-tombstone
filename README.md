# Indexing S3 files in DDB + Delete after Put consistency

Inspired by [this article](https://dev.to/aws-builders/serverlessly-uploading-files-1dog#fn2).

## The goal

The goal is to learn how to handle eventual consistency when dealing with asynchronous processes on AWS.
In this case, that would be indexing S3 files in DynamoDB and managing operations related to these files.

The article mentions _tombstones_, a record in a database containing meta-information about a given entity and its state.
Dealing with _tombstones_ is something that developers working with Cassandra have to deal with daily. This is not the case in DynamoDB, as DynamoDB does not have a concept of a _tombstone_.

## Learnings

- There are two ways to receive a notification from the S3 (I do not include the CloudTrail events in this list).

  - The first way is to use the good old S3 notifications feature.

    - Only a handful of targets.
    - The notification configuration is a resource tied with the bucket – it might lead to issues when writing IaC.
    - Do not cost you anything.

  - The second is to use the S3 <-> EventBridge integration.

    - Supports a wide variety of targets.
    - The configuration is decoupled from the bucket.
    - Much more extensive filtering options.
    - Usual EventBridge costs apply.

- The **S3 can only send the events to the default EventBridge event bus**.

- S3 introduced a convenient feature of **S3 Object Ownership** settings. I no longer have to think about ACL settings when creating the presigned URL.

  - This is a **bucket-wide setting**.

  - As a reminder, the Object ACL is a legacy way of controlling access to a given object. You might think of Object ACLs as a predecessor to IAM.
    Nowadays, AWS discourages the usage of Object ACLs in favor of IAM and bucket policies.

  - You can **disable the Object ACLs altogether by setting the Object Ownership to _"Bucket owner enforced"_**.
    There are other values one can use for Object Ownership like _"Bucket owner preferred"_ and _"Object writer"_ (default). When using those settings, the Object ACLs still have an effect.

  - To learn more about the Object Ownership setting, [consult this documentation page](https://docs.aws.amazon.com/AmazonS3/latest/userguide/about-object-ownership.html).

- The S3 EventBridge integrations, at least for me, feels better to work with than the S3 native event notifications.
  I guess it has to do with the better filtering capabilities. I feel like I have more control over the resources related to the flow.

  - One cannot forget that the **EventBridge forwards the S3 events to the default bus, and you cannot change that**. Not ideal.

- The newer AWS SDKs for JavaScript (v3) and Go (v2) enable you to get the errors from the DynamoDB TransactWrite operation somewhat reasonably.

  - This is not the case for the v2 of the JavaScript and v1 of the Go SDK. There, you either match the underlying error message (which is error-prone and does not get you the actual cause of the issue) or implement a [relatively complex workaround](https://github.com/aws/aws-sdk-js/issues/2464#issuecomment-503524701).

  - One thing that **caught me off guard** was that **if the transaction did not fail for a given item, the `CancellationReason` `Message` would be null, but the `Code` will NOT (for that item)**. This makes for interesting error handling logic.

- The `.grantWrite` method on the DynamoDB CDK construct does not include the `ConditionCheckItem` statement. It's interesting, but it makes sense as the `ConditionCheckItem` does not write anything to the table.

- General observation: when dealing with eventual consistency and asynchronous flows, it often makes more sense to write an additional item into the database that indicates a _"tombstone"_ rather than delete the original item. Of course, you most likely want to use TTL not to accumulate unnecessary storage costs.

### Go language

When writing error handling code for DynamoDB `TransactWrite` operation, I've stumbled upon the following code snippet in the documentation

```go
  if err != nil {
    var transactionCancelledErr *dynamodbtypes.TransactionCanceledException
    if errors.As(err, &transactionCancelledErr) {
      // code
    }
  }
```

Since `errors.As` **mutates the second argument** (pointer semantics), we must pass a pointer to that function. But why do we declare the "initial" value as the pointer?

We declare the "initial" value as the pointer because **the `error` interface is implemented by a pointer receiver**. If we **did not initialize that value as pointer**, that value **would not satisfy the error interface implementation**.

Now the question is: **Why the error interface implementation is based on pointer receivers?**.

The answer is that the implementation is based on pointer receivers to ensure that every error instance is unique.

```go
errors.New("foo") == errors.New("foo") // false
```

If we were working on concrete structs, the compiler would compare the **values inside the struct, and since the values are the same, the comparison would return true**.

```go
errors.NewWithoutPointers("foo") == errors.NewWithoutPointers("foo") // true
```

This is pretty bad as you could, in theory, create errors that have the same message (or properties) as some library and use that to compare errors from that library.
