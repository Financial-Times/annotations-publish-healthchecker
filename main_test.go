package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

type flags struct {
	error       bool
	healthyFlow bool
	sync.RWMutex
}

const checkingPeriodInMillis = 100

var (
	testFlags flags = flags{healthyFlow: true}

	testTxs = []transaction{
		{
			TransactionID: "tid1",
			UUID:          "uuid1",
			LastModified:  "2017-10-16T14:00:00.000Z",
		},
		{
			TransactionID: "tid2",
			UUID:          "uuid2",
			LastModified:  "2017-10-16T16:00:00.000Z",
		},
	}
)

func TestMain(m *testing.M) {

	healthcheckerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		testFlags.RLock()
		if testFlags.error {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			if testFlags.healthyFlow {
				txs := []transaction{}
				msg, _ := json.Marshal(txs)
				w.Write(msg)
			} else {
				msg, _ := json.Marshal(testTxs)
				w.Write(msg)
			}
		}
		testFlags.RUnlock()
	}))

	defer healthcheckerServer.Close()

	args := []string{
		`--port=8083`,
		fmt.Sprintf(`--event-reader=%s`, healthcheckerServer.URL),
	}

	ticker := time.NewTicker(checkingPeriodInMillis * time.Millisecond)
	app := initApp(ticker)

	go func() {
		app.Run(args)
	}()

	client := &http.Client{}
	retryCount := 0
	for {
		retryCount++
		if retryCount > 5 {
			fmt.Printf("Unable to start server")
			os.Exit(-1)
		}
		req, _ := http.NewRequest("GET", "http://localhost:8083/__gtg", nil)
		res, err := client.Do(req)
		if err == nil && res.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	os.Exit(m.Run())
}

func Test_GetGtg(t *testing.T) {
	tests := []struct {
		url            string
		expectedStatus int
		expectedHealth bool
		errorIsExp     bool
	}{
		{url: "http://localhost:8083/__gtg", expectedStatus: http.StatusOK, expectedHealth: true, errorIsExp: false},
		{url: "http://localhost:8083/__gtg", expectedStatus: http.StatusServiceUnavailable, expectedHealth: false, errorIsExp: true},
		// __gtg no more depends on the flow health (transaction closure), but only on Splunk reachability
		{url: "http://localhost:8083/__gtg", expectedStatus: http.StatusOK, expectedHealth: false, errorIsExp: false},
	}

	for _, test := range tests {

		testFlags.Lock()
		testFlags.error = test.errorIsExp
		testFlags.healthyFlow = test.expectedHealth
		testFlags.Unlock()

		time.Sleep(checkingPeriodInMillis * time.Millisecond)

		client := &http.Client{}
		req, _ := http.NewRequest("GET", test.url, nil)
		res, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedStatus, res.StatusCode)
	}
}

func Test_GetHealth(t *testing.T) {
	tests := []struct {
		url            string
		expectedStatus int
		expectedHealth bool
		errorIsExp     bool
	}{
		{url: "http://localhost:8083/__health", expectedStatus: http.StatusOK, expectedHealth: true, errorIsExp: false},
		{url: "http://localhost:8083/__health", expectedStatus: http.StatusOK, expectedHealth: false, errorIsExp: true},
		{url: "http://localhost:8083/__health", expectedStatus: http.StatusOK, expectedHealth: false, errorIsExp: false},
	}

	for _, test := range tests {

		testFlags.Lock()
		testFlags.error = test.errorIsExp
		testFlags.healthyFlow = test.expectedHealth
		testFlags.Unlock()

		time.Sleep(checkingPeriodInMillis * time.Millisecond)

		client := &http.Client{}

		req, _ := http.NewRequest("GET", test.url, nil)
		res, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedStatus, res.StatusCode)

		rBody := make([]byte, res.ContentLength)
		res.Body.Read(rBody)
		res.Body.Close()

		health := fthealth.HealthResult{}
		json.Unmarshal(rBody, &health)
		assert.Equal(t, test.expectedHealth, health.Ok)
	}
}

func Test_GetDetails(t *testing.T) {
	tests := []struct {
		url             string
		expectedStatus  int
		expHealthStatus healthStatus
		expectedHealth  bool
		errorIsExp      bool
	}{
		{url: "http://localhost:8083/__details", expectedStatus: http.StatusOK,
			expHealthStatus: healthStatus{[]transaction{}, "", fmt.Sprintf("Between %s and %s", earliestTime, latestTime), true},
			expectedHealth:  true, errorIsExp: false},
		{url: "http://localhost:8083/__details", expectedStatus: http.StatusOK,
			expHealthStatus: healthStatus{[]transaction{}, "", fmt.Sprintf("Between %s and %s", earliestTime, latestTime), false},
			expectedHealth:  false, errorIsExp: true},
		{url: "http://localhost:8083/__details", expectedStatus: http.StatusOK,
			expHealthStatus: healthStatus{testTxs, "", fmt.Sprintf("Between %s and %s", earliestTime, latestTime), true},
			expectedHealth:  false, errorIsExp: false},
	}

	for _, test := range tests {

		testFlags.Lock()
		testFlags.error = test.errorIsExp
		testFlags.healthyFlow = test.expectedHealth
		testFlags.Unlock()

		time.Sleep(checkingPeriodInMillis * time.Millisecond)

		client := &http.Client{}

		req, _ := http.NewRequest("GET", test.url, nil)
		res, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedStatus, res.StatusCode)

		b, err := ioutil.ReadAll(res.Body)
		assert.NoError(t, err)

		var actHealthStatus healthStatus
		err = json.Unmarshal(b, &actHealthStatus)
		assert.NoError(t, err)

		assertEqual(t, test.expHealthStatus, actHealthStatus)
	}
}
