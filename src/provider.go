package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

type HealthLog struct {
	Timestamp string `json:"timestamp"`
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Status    int    `json:"status"`
}

func getProvider(r *http.Request) (string, string) {
	reqProvider := r.URL.Query().Get("provider")
	if reqProvider == "" {
		return "", ""
	}

	providerUrl := healthProviders[reqProvider]
	if providerUrl == "" {
		return "", ""
	}
	return reqProvider, providerUrl
}

func providerRequest(url string) (*http.Response, error) {

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func healthCheck(w http.ResponseWriter, r *http.Request) {

	provider, providerUrl := getProvider(r)
	if provider == "" {
		http.Error(w, "Unrecognized provider", http.StatusBadRequest)
	}

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

	available := respError == nil && resp.StatusCode < 500
	logEntry := HealthLog{
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

	w.Header().Set("Content-Type", "application/json")
	encoderErr := json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":  provider,
		"available": respError == nil && resp.StatusCode < 500,
	})
	if encoderErr != nil {
		return
	}
}
