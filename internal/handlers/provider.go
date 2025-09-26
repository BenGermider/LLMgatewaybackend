package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"llmgatewaybackend/internal/config"
	"llmgatewaybackend/internal/models"
)

// getProvider extracts the provider parameter and returns it's url.
func getProvider(r *http.Request) (string, string) {
	reqProvider := r.URL.Query().Get("provider")
	if reqProvider == "" {
		return "", ""
	}

	providerUrl := config.HealthProviders[reqProvider]
	if providerUrl == "" {
		return "", ""
	}
	return reqProvider, providerUrl
}

// providerRequest sends the http request to the LLM.
func providerRequest(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// HealthCheck handler. Sends ping to LLM and returns result to user. Handles exceptions.
func HealthCheck(w http.ResponseWriter, r *http.Request) {

	// Checks whether the input is valid.
	provider, providerUrl := getProvider(r)
	if provider == "" {
		http.Error(w, "Unrecognized provider", http.StatusBadRequest)
	}

	// Ping LLM.
	resp, respError := providerRequest(providerUrl)
	if respError != nil {
		http.Error(w, "Provider unavailable", http.StatusServiceUnavailable)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	// Builds the response to the user.
	available := respError == nil && resp.StatusCode < 500
	logEntry := models.HealthLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Provider:  provider,
		Available: available,
		Status:    resp.StatusCode,
	}

	logJSON, err := json.MarshalIndent(logEntry, "", "  ")
	if err != nil {
		log.Println("Failed to marshal health log:", err)
	} else {
		log.Println(string(logJSON))
	}

	// Response.
	w.Header().Set(config.ContentType, config.ApplicationJson)
	encoderErr := json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":  provider,
		"available": respError == nil && resp.StatusCode < 500,
	})
	if encoderErr != nil {
		return
	}
}
