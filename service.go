package main

import (
	"encoding/json"
	"fmt"
	"github.com/Financial-Times/go-logger"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	earliestTimePathVar = "earliestTime"
	latestTimePathVar   = "latestTime"
	earliestTime        = "-15m"
	latestTime          = "-5m"
	contentType         = "annotations"
	timestampFormat     = time.RFC3339Nano
)

type healthchecker interface {
	getHealthStatus() interface{}
}

type healthmonitor interface {
	monitorPublishHealth()
}

type healthcheckerService struct {
	eventReaderAddress string
	healthStatus       healthStatus
	slaWindow          int
	sync.RWMutex
}

func (s *healthcheckerService) monitorPublishHealth(ticker *time.Ticker) chan bool {

	s.Lock()
	s.healthStatus = determineHealth(s.eventReaderAddress, s.slaWindow, contentType, earliestTime, latestTime)
	s.Unlock()

	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.Lock()
				s.healthStatus = determineHealth(s.eventReaderAddress, s.slaWindow, contentType, earliestTime, latestTime)
				s.Unlock()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return quit
}

func (s *healthcheckerService) getHealthStatus() interface{} {

	s.RLock()
	status := s.healthStatus
	s.RUnlock()

	return status
}

func determineHealth(eventReaderAddress string, slaWindow int, contentType string, earliestTime string, latestTime string) healthStatus {

	now := time.Now()
	checkingTime := now.Format(timestampFormat)
	checkingPeriod := fmt.Sprintf("Between %s and %s", earliestTime, latestTime)

	req, err := http.NewRequest("GET", eventReaderAddress+"/"+contentType+"/transactions", nil)

	q := req.URL.Query()
	q.Add(earliestTimePathVar, fmt.Sprintf("%s", earliestTime))
	q.Add(latestTimePathVar, fmt.Sprintf("%s", latestTime))
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.WithError(err).Errorf("Failed to retrieve transactions from %s", req.URL.String())
		return healthStatus{[]transaction{}, checkingTime, checkingPeriod, false}
	}
	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		logger.WithError(err).Errorf("Failed to retrieve transactions from %s with status code %s", req.URL.String(), resp.StatusCode)
		return healthStatus{[]transaction{}, checkingTime, checkingPeriod, false}
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Errorf("Error parsing transaction body for url %s", req.URL.String())
		return healthStatus{[]transaction{}, checkingTime, checkingPeriod, false}
	}

	var txs transactions
	if err := json.Unmarshal(b, &txs); err != nil {
		logger.WithError(err).Errorf("Error unmarshalling transaction log messages for url %s", req.URL.String())
		return healthStatus{[]transaction{}, checkingTime, checkingPeriod, false}
	}

	// ignore recent transactions that might be already closed - even if they are unclosed when the query happens
	txs = ignoreRecentTransactions(txs, now, latestTime, slaWindow)

	return healthStatus{txs, checkingTime, checkingPeriod, true}
}

func ignoreRecentTransactions(txs transactions, referenceTime time.Time, delay string, slaWindow int) transactions {

	// compute the delay that the requests are executed with
	delay = strings.Split(latestTime, "m")[0]
	d, err := strconv.Atoi(delay)
	if err != nil {
		logger.WithError(err).Errorf("LatestTime (%s) was not parsable, couldn't be determined the delay that the requests are executed with. No transactions are filtered out from the resultset.", latestTime)
		return txs
	}

	// compute the time when the checking period starts - example: the checks are done with a 5 minutes delay
	referenceTime = referenceTime.Add(time.Duration(d) * time.Minute)

	// ignore the SLA Window - publishes inside that could still successfully make through, even if they are unclosed yet
	referenceTime = referenceTime.Add(-time.Duration(slaWindow) * time.Minute)

	res := transactions{}
	for _, tx := range txs {

		// if time not parsable: don't filter out
		txTime, err := time.Parse(timestampFormat, tx.LastModified)
		if err != nil {
			logger.WithTransactionID(tx.TransactionID).WithError(err).Errorf("Duration couldn't be determined for timestamp %s.", txTime)
			res = append(res, tx)
		} else if !txTime.After(referenceTime) {
			// if transaction time is before the reference time: consider it as a failed publish
			res = append(res, tx)
		}
	}
	return res
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
