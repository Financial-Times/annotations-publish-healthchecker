# annotations-publish-healthchecker

[![Circle CI](https://circleci.com/gh/Financial-Times/annotations-publish-healthchecker/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/annotations-publish-healthchecker/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/annotations-publish-healthchecker)](https://goreportcard.com/report/github.com/Financial-Times/annotations-publish-healthchecker) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/annotations-publish-healthchecker/badge.svg)](https://coveralls.io/github/Financial-Times/annotations-publish-healthchecker)

## Introduction

This is a service, that reports whether the annotations publishing flow works as expected.
It checks and caches a "healthiness" status every minute. 
The responses from the __health and __details endpoint will be provided based on this cache.


"Annotations publish flow healthiness check"
Such a check is looking for unclosed annotation publish transactions (transactions with no PublishEnd events).
Since the monitoring service closes the transactions every 5 minutes, this healthchecker verifies the transactions happening before the latest 5 minutes, and it checks for a period of 10 minutes.

## Installation

Download the source code, dependencies and test dependencies:

        go get -u github.com/kardianos/govendor
        go get -u github.com/Financial-Times/annotations-publish-healthchecker
        cd $GOPATH/src/github.com/Financial-Times/annotations-publish-healthchecker
        govendor sync
        go build .

## Running locally

1. Run the tests and install the binary:

        govendor sync
        govendor test -v -race
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/annotations-publish-healthchecker [--help]

Options:

        --app-system-code="annotations-publish-healthchecker"            System Code of the application ($APP_SYSTEM_CODE)
        --app-name="Annotations Publish Healthchecker"                   Application name ($APP_NAME)
        --port="8080"                                                    Port to listen on ($APP_PORT)
        --event-reader="http://localhost:8080/__splunk-event-reader"     URL for the Splunk Event Reader

## Build and deployment

* Built by Docker Hub on merge to master: [coco/annotations-publish-healthchecker](https://hub.docker.com/r/coco/annotations-publish-healthchecker/)
* CI provided by CircleCI: [annotations-publish-healthchecker](https://circleci.com/gh/Financial-Times/annotations-publish-healthchecker)

## Service endpoints

e.g.
### GET

Using curl:

    curl http://localhost:8080/__health | json_pp`
    
    curl http://localhost:8080/__details | json_pp`

Or using [httpie](https://github.com/jkbrzt/httpie):

    http GET http://localhost:8080/__details

The expected response will contain information about the health of the annotations publish flow.

An example response for the `__details` endpoint looks like this:
    
    {
    failed_transactions: [ ],
    event_reader_checking_period: "Between -15m and -5m",
    event_reader_checking_time: "2017-12-19T16:43:06.351754912+02:00",
    event_reader_was_reachable: true
    }
    

The response indicates:
 - `failed_transactions`: list of the transactions that have recently failed (`transaction_id`, `uuid`, `publish_start` time - if known)
 - `event_reader_checking_period`: the period that the check was executed for (defaults to an interval of 10 minutes, with a 5 minute delay)
 - `event_reader_checking_time`: the exact time when the sanity check happened
 - `event_reader_was_reachable`: whether the last sanity check was successful (the event reader could be reached) - otherwise we cannot know that the publishing flow is working properly


## Utility endpoints
_Endpoints that are there for support or testing, e.g read endpoints on the writers_

## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

The health endpoint executes two checks:
- `Splunk Event Reader is reachable` - This check verifies whether the latest call to the splunk-event-reader was successful, hence the healthcheck results are relevant
- `Annotations Publish Failures` - Splunk-event-reader is reachable, and at least 2 publish failures were detected for the latest call.

`/__build-info`

### Logging

* The application uses the FT logging library [go-logger](https://github.com/Financial-Times/go-logger), which is based on [logrus](https://github.com/sirupsen/logrus).
* NOTE: `/__build-info` and `/__gtg` endpoints are not logged as they are called every second from varnish/vulcand and this information is not needed in logs/splunk.
