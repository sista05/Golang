# AWS Lambda for Kinesis Data Streams Transformation

[![Language](http://img.shields.io/badge/language-Go-brightgreen.svg?style=flat
)](https://golang.org/)
[![License](http://img.shields.io/badge/license-MIT-lightgrey.svg?style=flat
)](http://mit-license.org)
[![Twitter](https://img.shields.io/badge/twitter-@sista05-blue.svg?style=flat)](http://twitter.com/sista05)
[![Build Status](https://travis-ci.org/sista05/Log_aggregation_by_lambda.svg?branch=master)](https://travis-ci.org/sista05/Log_aggregation_by_lambda)

## Overview
Send nginx access log and laravel application log to s3/slack/elasticsearch.

## Description

- Supported nginx(ltsv) access log and laravel(json) log format.
- Set mapping for elasticsearch (dynamic mapping not working).
- Sort nginx log by hostname.
- Supported athena JSON SerDe libraries.
- Send notification alert when AWS Lambda function has an error.

## Requirement

- [CloudFormation](https://github.com/sista05/CFn)

## Environment Variables

#### kinesis-send-log

| Variable |Description|
| :--- | :--- |
| REGION | region name (e.g. ap-northeast-1)
| S3_BUCKET| log strage bucket name |
| SLACK_WEBHOOK_URL| slack webhook URL |
| SLACK_NAME| slack profile name |
| ES_URL| elasticsearch endpoint |
| ES_NGINX_INDEX| Elasticsearch index (nginx log)|
| ES_NGINX_INDEXTYPE| Elasticsearch type (nginx log)|
| ES_APP_INDEX| Elasticsearch index (laravel log)|
| ES_APP_INDEXTYPE| Elasticsearch type (laravel log)|

#### kinesis-send-end-log

| Variable |Description|
| :--- | :--- |
| REGION | region name (e.g. ap-northeast-1)
| S3_BUCKET| log strage bucket name |
| SLACK_WEBHOOK_URL| log strage bucket name |
| SLACK_CHANNEL| slack channel destination |
| SLACK_NAME| slack profile name |

#### alert-lambda-failure

| Variable |Description|
| :--- | :--- |
| SLACK_WEBHOOK_URL| log strage bucket name |
| SLACK_CHANNEL| slack channel destination |
| SLACK_NAME| slack profile name |


## build

```
$ make build

````

## Deploying Lambda functions to AWS

First we'll need to zip up the code for our Lambda function and then upload it to ~~S3~~ local directory before we can deploy it via CloudFormation. We also need to make sure that our project and its functions are within a git repository. Run git init to set this up.

Get [CFn](https://github.com/sista05/Cloudformation) and build/move zipfile to path (Cloudformation/build)

## License
The package is available as open source under the terms of the MIT License.

## TODO

- use Travis CI

## Reference
https://deeeet.com/writing/2014/07/31/readme/
