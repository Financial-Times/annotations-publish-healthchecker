package main

import (
	"encoding/json"
	"fmt"
	logger "github.com/Financial-Times/go-logger/test"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type input struct {
	eventReaderAddress string
	contentType        string
	earliestTime       string
	latestTime         string
	slaWindow          time.Duration
}

type output struct {
	status    healthStatus
	err       string
	outputMsg string
}

func TestDetermineHealth_Unhealthy(t *testing.T) {

	hook := logger.NewTestHook("healthchecker-test")

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(""))
	}))
	defer healthcheckerServer.Close()

	var tests = []struct {
		scenario string
		in       input
		out      output
	}{
		{"Incorrect address - wrong protocol",
			input{"address", "annotations", "earliest-time", "latest-time", 2 * 60 * time.Second},
			output{healthStatus{
				[]transaction{},
				time.Now().Format(timestampFormat),
				"Between earliest-time and latest-time",
				false,
			},
				"unsupported protocol scheme", "Failed to retrieve transactions from",
			},
		},
		{"Incorrect address - no response",
			input{"http://localhost:8080", "annotations", "earliest-time", "latest-time", 2 * 60 * time.Second},
			output{healthStatus{
				[]transaction{},
				time.Now().Format(timestampFormat),
				"Between earliest-time and latest-time",
				false,
			},
				"connection refused", "Failed to retrieve transactions from",
			},
		},
		{"Server errors: 503",
			input{healthcheckerServer.URL, "annotations", "earliest-time", "latest-time", 2 * 60 * time.Second},
			output{healthStatus{
				[]transaction{},
				time.Now().Format(timestampFormat),
				"Between earliest-time and latest-time",
				false,
			},
				"", "Failed to retrieve transactions from",
			},
		},
	}

	for _, test := range tests {

		res := determineHealth(test.in.eventReaderAddress, test.in.slaWindow, test.in.contentType, test.in.earliestTime, test.in.latestTime)
		if test.out.outputMsg == "" {
			assert.Equal(t, 0, len(hook.Entries))
		} else {
			e := hook.LastEntry()
			assert.Contains(t, e.Message, test.out.outputMsg)
			assert.Equal(t, e.Level.String(), "error")
		}

		if test.out.err != "" {
			e := hook.LastEntry()
			assert.Contains(t, e.Data["error"].(error).Error(), test.out.err)
		}

		assertEqual(t, test.out.status, res)
	}
}

func TestDetermineHealth_IncorrectResponse(t *testing.T) {

	hook := logger.NewTestHook("healthchecker-test")
	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("some unparsable message"))
	}))
	defer healthcheckerServer.Close()

	res := determineHealth(healthcheckerServer.URL, 2*60*time.Second, "anyType", "earliestTime", "latestTime")
	assert.Equal(t, 1, len(hook.Entries))
	assert.Contains(t, hook.LastEntry().Message, "Error unmarshalling transaction log messages for url")
	assert.Contains(t, hook.LastEntry().Data["error"].(error).Error(), "invalid character")

	assertEqual(t, healthStatus{[]transaction{}, "", "Between earliestTime and latestTime", false}, res)
}

func TestDetermineHealth_200(t *testing.T) {

	hook := logger.NewTestHook("healthchecker-test")
	txs := []transaction{
		{
			TransactionID: "tid1",
			UUID:          "uuid1",
			LastModified:  "2018-01-15T14:57:42.567Z",
		},
		{
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-12-15T14:57:42.567Z",
		},
	}

	msg, err := json.Marshal(txs)
	assert.Nil(t, err)

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(msg)
	}))
	defer healthcheckerServer.Close()

	res := determineHealth(healthcheckerServer.URL, 2*60*time.Second, "anyType", "earliestTime", "latestTime")
	assert.Equal(t, 0, len(hook.Entries))
	assertEqual(t, healthStatus{txs, "", "Between earliestTime and latestTime", true}, res)
}

func TestMonitorPublishHealth(t *testing.T) {

	hook := logger.NewTestHook("healthchecker-test")
	txs := []transaction{
		{
			TransactionID: "tid1",
			UUID:          "uuid1",
			LastModified:  "2017-01-15T14:57:42.567Z",
		},
		{
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-02-13T12:00:00.000Z",
		},
	}

	msg, err := json.Marshal(txs)
	assert.Nil(t, err)

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(msg)
	}))
	defer healthcheckerServer.Close()

	service := healthcheckerService{
		eventReaderAddress: healthcheckerServer.URL,
		healthStatus:       healthStatus{},
	}

	ticker := time.NewTicker(1 * time.Second)
	quit := service.monitorPublishHealth(ticker)

	time.Sleep(5 * time.Second)
	quit <- true

	assertEqual(t, service.getHealthStatus().(healthStatus), healthStatus{txs, "", fmt.Sprintf("Between %s and %s", earliestTime, latestTime), true})
	assert.Equal(t, 0, len(hook.Entries))
}

func TestIgnoreSLAWindow(t *testing.T) {

	timeCheck := "2017-02-13T12:00:00.000Z"
	slaWindow := 2 * 60 * time.Second
	fullTXS := []transaction{
		{ // OK
			TransactionID: "tid1",
			UUID:          "uuid1",
			LastModified:  "2017-02-13T11:55:00.000Z",
		},
		{ // OUTSIDE SLA
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-02-13T11:59:00.000Z",
		},
		{ // OUTSIDE SLA
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-02-13T11:58:59.000Z",
		},
		{ // OK
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-02-13T11:50:59.000Z",
		},
		{ // OK
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "not_parsable",
		},
	}

	expTXS := transactions{
		{
			TransactionID: "tid1",
			UUID:          "uuid1",
			LastModified:  "2017-02-13T11:55:00.000Z",
		},
		{
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-02-13T11:50:59.000Z",
		},
		{
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "not_parsable",
		},
	}

	actualTXS := ignoreSLAWindow(fullTXS, timeCheck, slaWindow)
	assert.Equal(t, expTXS, actualTXS)
}

func TestInsideSLA(t *testing.T) {

	var tests = []struct {
		timeToCompare string
		checkingTime  string
		slaWindow     time.Duration
		isInsideSLA   bool
	}{
		{
			"2017-02-13T11:59:00.000Z",
			"2017-02-13T12:00:00.000Z",
			2 * 60 * time.Second,
			true,
		},
		{
			"2017-02-13T11:58:59.000Z",
			"2017-02-13T12:00:00.000Z",
			2 * 60 * time.Second,
			true,
		},
		{
			"2017-02-13T11:58:00.000Z",
			"2017-02-13T12:00:00.000Z",
			2 * 60 * time.Second,
			false,
		},
		{
			"2017-02-13T11:50:00.000Z",
			"2017-02-13T12:00:00.000Z",
			2 * 60 * time.Second,
			false,
		},
	}

	for _, test := range tests {
		actualSLA := insideSLA(test.timeToCompare, test.checkingTime, test.slaWindow)
		assert.Equal(t, test.isInsideSLA, actualSLA)
	}
}

func TestInsideSLA_NotParsable(t *testing.T) {

	hook := logger.NewTestHook("healthchecker-test")
	actualSLA := insideSLA("not_parsable", "2017-02-13T12:00:00.000Z", 2*60*time.Second)
	assert.Equal(t, false, actualSLA)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Contains(t, hook.LastEntry().Message, "Duration couldn't be determined for timestamp not_parsable")

}

func assertEqual(t *testing.T, s1 healthStatus, s2 healthStatus) {
	assert.Equal(t, s1.OpenTransactions, s2.OpenTransactions)
	assert.Equal(t, s1.LastTimeCheck, s2.LastTimeCheck)
	assert.Equal(t, s1.Successful, s2.Successful)
}
