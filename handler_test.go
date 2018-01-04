package main

import (
	"encoding/json"
	"github.com/Financial-Times/go-logger"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetHealthDetails_200(t *testing.T) {

	status := healthStatus{
		OpenTransactions: []transaction{},
		CheckingPeriod:   "Between -15m and -5m",
		LastTimeCheck:    "2017-12-21T11:42:15.262459+02:00",
		Successful:       true,
	}

	req, err := http.NewRequest("GET", "/__details", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h := requestHandler{&mockService{healthStatus: status}}
	handler := http.HandlerFunc(h.getHealthDetails)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	msg, err := json.Marshal(status)
	if rr.Body.String() != string(msg) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), string(msg))
	}
}

func TestGetHealthDetails_500(t *testing.T) {

	logger.InitDefaultLogger("healthchecker")

	req, err := http.NewRequest("GET", "/__details", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h := requestHandler{&mockService{healthStatus: make(chan int)}}
	handler := http.HandlerFunc(h.getHealthDetails)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}

	if rr.Body.String() != "" {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), "")
	}
}

type mockService struct {
	healthStatus interface{}
}

func (ms *mockService) getHealthStatus() interface{} {
	return ms.healthStatus
}
