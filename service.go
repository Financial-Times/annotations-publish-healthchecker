package main

import (
	"encoding/json"
	"fmt"
	"github.com/Financial-Times/go-logger"
	"io"
	"io/ioutil"
	"net/http"
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
	slaWindow          time.Duration
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

func determineHealth(eventReaderAddress string, slaWindow time.Duration, contentType string, earliestTime string, latestTime string) healthStatus {

	timeCheck := time.Now().Format(timestampFormat)
	checkPeriod := fmt.Sprintf("Between %s and %s", earliestTime, latestTime)

	req, err := http.NewRequest("GET", eventReaderAddress+"/"+contentType+"/transactions", nil)

	q := req.URL.Query()
	q.Add(earliestTimePathVar, fmt.Sprintf("%s", earliestTime))
	q.Add(latestTimePathVar, fmt.Sprintf("%s", latestTime))
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.WithError(err).Errorf("Failed to retrieve transactions from %s", req.URL.String())
		return healthStatus{[]transaction{}, timeCheck, checkPeriod, false}
	}
	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		logger.WithError(err).Errorf("Failed to retrieve transactions from %s with status code %s", req.URL.String(), resp.StatusCode)
		return healthStatus{[]transaction{}, timeCheck, checkPeriod, false}
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Errorf("Error parsing transaction body for url %s", req.URL.String())
		return healthStatus{[]transaction{}, timeCheck, checkPeriod, false}
	}

	var txs transactions
	if err := json.Unmarshal(b, &txs); err != nil {
		logger.WithError(err).Errorf("Error unmarshalling transaction log messages for url %s", req.URL.String())
		return healthStatus{[]transaction{}, timeCheck, checkPeriod, false}
	}

	// ignore publishes within the SLA window
	// they could still successfully make through, even if they are unclosed yet
	txs = ignoreSLAWindow(txs, timeCheck, slaWindow)

	return healthStatus{txs, timeCheck, checkPeriod, true}
}

func ignoreSLAWindow(txs transactions, timeCheck string, slaWindow time.Duration) transactions {

	res := transactions{}
	for _, tx := range txs {
		if !insideSLA(tx.LastModified, timeCheck, slaWindow) {
			res = append(res, tx)
		}
	}
	return res
}

func insideSLA(timeToCompare string, checkingTime string, slaWindow time.Duration) bool {

	// if not parsable => not inside SLA
	t1, err := time.Parse(timestampFormat, timeToCompare)
	if err != nil {
		logger.WithError(err).Errorf("Duration couldn't be determined for timestamp %s.", timeToCompare)
		return false
	}

	t2, _ := time.Parse(timestampFormat, checkingTime)
	if err != nil {
		logger.WithError(err).Errorf("Duration couldn't be determined for timestamp %s.", checkingTime)
		return false
	}

	if t2.Sub(t1) < slaWindow {
		return true
	} else {
		return false
	}
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
