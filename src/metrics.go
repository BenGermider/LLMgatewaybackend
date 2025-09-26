package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type Metrics struct {
	TotalRequests       int64            `json:"total_requests"`
	RequestsPerProvider map[string]int64 `json:"requests_per_provider"`
	AverageResponseTime float64          `json:"average_response_time_ms"`
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	bytes, err := os.ReadFile(USAGE_FILE)
	if err != nil {
		http.Error(w, "Failed to read logs file", http.StatusInternalServerError)
		return
	}

	var logsMap map[string]Usage
	if err := json.Unmarshal(bytes, &logsMap); err != nil {
		http.Error(w, "Failed to parse logs", http.StatusInternalServerError)
		return
	}

	logBytes, err := json.MarshalIndent(logsMap, "", "  ")
	if err != nil {
		log.Println("Failed to marshal logsMap:", err)
	} else {
		log.Println(string(logBytes))
	}

	totalDuration := int64(0)
	metrics := Metrics{
		RequestsPerProvider: make(map[string]int64),
	}

	for _, usage := range logsMap {
		metrics.TotalRequests += int64(usage.RequestCount)
		metrics.RequestsPerProvider[usage.Provider] += int64(usage.RequestCount)
		totalDuration += usage.TotalRequestTimeMs // approximate total duration
	}

	if metrics.TotalRequests > 0 {
		metrics.AverageResponseTime = float64(totalDuration / metrics.TotalRequests)
	} else {
		metrics.AverageResponseTime = 0
	}

	w.Header().Set("Content-Type", "application/json")
	encodeErr := json.NewEncoder(w).Encode(metrics)
	if encodeErr != nil {
		return
	}
}
