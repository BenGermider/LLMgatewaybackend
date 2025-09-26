package handlers

import (
	"encoding/json"
	"llmgatewaybackend/internal/models"
	"log"
	"net/http"
	"os"

	"llmgatewaybackend/internal/config"
)

// MetricsHandler handle the metrics request, calculates them and returns to the user.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {

	// Tries opening the data for calculations, trigger an error if fails.
	bytes, err := os.ReadFile(config.UsageFile)
	if err != nil {
		http.Error(w, "Failed to read logs file", http.StatusInternalServerError)
		return
	}

	var logsMap map[string]models.Usage
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

	// Metrics calculations - counts the time and the requests to calculate average request time/
	totalDuration := int64(0)
	metrics := models.Metrics{
		RequestsPerProvider: make(map[string]int64),
	}

	for _, usage := range logsMap {
		metrics.TotalRequests += int64(usage.RequestCount)
		metrics.RequestsPerProvider[usage.Provider] += int64(usage.RequestCount)
		totalDuration += usage.TotalRequestTimeMs
	}

	if metrics.TotalRequests > 0 {
		metrics.AverageResponseTime = float64(totalDuration / metrics.TotalRequests)
	} else {
		metrics.AverageResponseTime = 0
	}

	w.Header().Set(config.ContentType, config.ApplicationJson)
	encodeErr := json.NewEncoder(w).Encode(metrics)
	if encodeErr != nil {
		return
	}
}
