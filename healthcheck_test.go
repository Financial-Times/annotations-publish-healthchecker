package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEventReaderIsReachable(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))

	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck: time.Now().Format(timestampFormat),
		Successful:    true,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	message, err := healthService.eventReaderIsReachable()

	assert.Equal(t, fmt.Sprintf("Splunk Event Reader was reachable. Latest check at: %s", healthStatus.LastTimeCheck), message)
	assert.Nil(t, err)
}

func TestEventReaderIsNotReachable(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))

	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck: time.Now().Format(timestampFormat),
		Successful:    false,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	message, err := healthService.eventReaderIsReachable()

	assert.Equal(t, fmt.Sprintf("Splunk Event Reader was not reachable. Latest check at: %s", healthStatus.LastTimeCheck), err.Error())
	assert.Empty(t, message)
}

func TestFailedTransactionsChecker_0(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))

	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck:    time.Now().Format(timestampFormat),
		OpenTransactions: []transaction{},
		Successful:       true,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	message, err := healthService.failedTransactionsChecker()

	assert.Equal(t, fmt.Sprintf("No degradation detected. NO of failures: 0. Latest check at: %s", healthStatus.LastTimeCheck), message)
	assert.Nil(t, err)
}

func TestFailedTransactionsChecker_1(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))

	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck: time.Now().Format(timestampFormat),
		OpenTransactions: []transaction{
			{
				TransactionID: "tid1",
				UUID:          "uuid1",
				LastModified:  "some_date",
			},
		},
		Successful: true,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	message, err := healthService.failedTransactionsChecker()

	assert.Equal(t, fmt.Sprintf("No degradation detected. NO of failures: 1. Latest check at: %s", healthStatus.LastTimeCheck), message)
	assert.Nil(t, err)
}

func TestFailedTransactionsChecker_2(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))

	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck: time.Now().Format(timestampFormat),
		OpenTransactions: []transaction{
			{
				TransactionID: "tid1",
				UUID:          "uuid1",
				LastModified:  "some_date1",
			},
			{
				TransactionID: "tid2",
				UUID:          "uuid2",
				LastModified:  "some_date2",
			},
		},
		Successful: true,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	message, err := healthService.failedTransactionsChecker()

	assert.Equal(t, fmt.Sprintf("Degradation detected. NO of failures: 2. Latest check at: %s", healthStatus.LastTimeCheck), err.Error())
	assert.Empty(t, message)
}

func TestEventReader_GTG_Succeeds(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))
	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck: time.Now().Format(timestampFormat),
		OpenTransactions: []transaction{
			{
				TransactionID: "tid1",
				UUID:          "uuid1",
				LastModified:  "some_date",
			},
		},
		Successful: true,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	status := healthService.gtgCheck()

	assert.Empty(t, status.Message)
	assert.Equal(t, true, status.GoodToGo)
}

func TestEventReader_GTG_Fails(t *testing.T) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))
	defer healthcheckerServer.Close()

	healthStatus := healthStatus{
		LastTimeCheck: time.Now().Format(timestampFormat),
		OpenTransactions: []transaction{
			{
				TransactionID: "tid1",
				UUID:          "uuid1",
				LastModified:  "some_date",
			},
		},
		Successful: false,
	}

	healthService := newHealthService(&healthConfig{}, &healthcheckerService{eventReaderAddress: healthcheckerServer.URL, healthStatus: healthStatus})
	status := healthService.gtgCheck()

	assert.Equal(t, fmt.Sprintf("Splunk Event Reader was not reachable. Latest check at: %s", healthStatus.LastTimeCheck), status.Message)
	assert.Equal(t, false, status.GoodToGo)
}
