package main

import (
	"encoding/json"
	"github.com/Financial-Times/go-logger"
	"net/http"
)

type requestHandler struct {
	healthchecker healthchecker
}

func (handler *requestHandler) getHealthDetails(writer http.ResponseWriter, request *http.Request) {

	writer.Header().Add("Content-Type", "application/json")

	msg, err := json.Marshal(handler.healthchecker.getHealthStatus())

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
	} else {
		writer.WriteHeader(http.StatusOK)
		writer.Write(msg)
	}
}
