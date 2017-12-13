package main

import (
	"encoding/json"
	"fmt"
	"github.com/Financial-Times/go-logger"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	earliestTimePathVar = "earliestTime"
	latestTimePathVar   = "latestTime"
	earliestTime        = "-15m"
	latestTime          = "-5m"
	checkFrequency      = 1
	contentType         = "annotations"
	timestampFormat     = time.RFC3339Nano
)

var cache txCache

type requestHandler struct {
	eventReaderAddress string
}

func (handler *requestHandler) getOpenTransactions(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	msg, err := json.Marshal(cache)

	if err != nil {
		logger.Error(err)
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.Write([]byte(msg))
	}
}

func (handler *requestHandler) checkMonitoringStatus() {

	resetTransactionCache(handler.eventReaderAddress)
	ticker := time.NewTicker(checkFrequency * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				resetTransactionCache(handler.eventReaderAddress)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func resetTransactionCache(eventReaderAddress string) {

	cache = txCache{LastTimeCheck: time.Now().Format(timestampFormat), Successful: false, CheckingPeriod: fmt.Sprintf("Between %s and %s", earliestTime, latestTime)}

	req, err := http.NewRequest("GET", eventReaderAddress+"/"+contentType+"/transactions", nil)

	q := req.URL.Query()
	q.Add(earliestTimePathVar, fmt.Sprintf("%s", earliestTime))
	q.Add(latestTimePathVar, fmt.Sprintf("%s", latestTime))
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.WithError(err).Errorf("Failed to retrieve transactions from %s", req.URL.String())
		return
	}
	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		logger.WithError(err).Errorf("Failed to retrieve transactions from %s with status code %s", req.URL.String(), resp.StatusCode)
		return
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Errorf("Error parsing transaction body for url %s", req.URL.String())
		return
	}

	var txs transactions
	if err := json.Unmarshal(b, &txs); err != nil {
		logger.WithError(err).Errorf("Error unmarshalling transaction log messages for url %s", req.URL.String())
		return
	}

	cache.Successful = true
	cache.OpenTransactions = txs
	return
}

func cleanUp(resp *http.Response) {
	_, err := io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		logger.Warnf("[%v]", err)
	}

	err = resp.Body.Close()
	if err != nil {
		logger.Warnf("[%v]", err)
	}
}
