package main

import (
	"fmt"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

const healthPath = "/__health"

type healthService struct {
	config        *healthConfig
	checks        []health.Check
	healthchecker *healthcheckerService
}

type healthConfig struct {
	appSystemCode string
	appName       string
	port          string
}

func newHealthService(config *healthConfig, healthchecker *healthcheckerService) *healthService {
	service := &healthService{config: config}
	service.checks = []health.Check{
		service.reachabilityCheck(),
		service.failedTransactionsCheck(),
	}
	service.healthchecker = healthchecker

	return service
}

func (service *healthService) reachabilityCheck() health.Check {
	return health.Check{
		BusinessImpact:   "Shows whether this healthcheckerService can monitor the success of the annotations publishing",
		Name:             "Splunk Event Reader is reachable",
		PanicGuide:       "https://dewey.ft.com/annotations-publish-healthchecker.html",
		Severity:         1,
		TechnicalSummary: "This check verifies whether the latest call to the splunk-event-reader was successful, hence the results are relevant",
		Checker:          service.eventReaderIsReachable,
	}
}

func (service *healthService) eventReaderIsReachable() (string, error) {

	status := service.healthchecker.getHealthStatus().(healthStatus)
	msg := fmt.Sprintf("Latest check at: %s", status.LastTimeCheck)
	if status.Successful {
		return fmt.Sprintf("Splunk Event Reader was reachable. %s", msg), nil
	} else {
		return "", fmt.Errorf("Splunk Event Reader was not reachable. %s", msg)
	}
}

func (service *healthService) failedTransactionsCheck() health.Check {
	return health.Check{
		BusinessImpact:   "At least 2 publish failures were detected for the latest check. This will reflect in the SLA measurement.",
		Name:             "Annotations Publish Failures",
		PanicGuide:       "https://dewey.ft.com/annotations-publish-healthchecker.html",
		Severity:         1,
		TechnicalSummary: "Annotations publishes failed. There is a degradation in the annotations publish or monitoring services. Check the /__details endpoint.",
		Checker:          service.failedTransactionsChecker,
	}
}

func (service *healthService) failedTransactionsChecker() (string, error) {

	status := service.healthchecker.getHealthStatus().(healthStatus)
	msg := fmt.Sprintf("NO of failures: %d. Latest check at: %s", len(status.OpenTransactions), status.LastTimeCheck)
	if len(status.OpenTransactions) >= 2 {
		return "", fmt.Errorf("Degradation detected. %s", msg)
	} else {
		return fmt.Sprintf("No degradation detected. %s", msg), nil
	}
}

func (service *healthService) gtgCheck() gtg.Status {

	if _, err := service.reachabilityCheck().Checker(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}

	return gtg.Status{GoodToGo: true}
}
