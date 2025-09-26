package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

import (
	"llmgatewaybackend/internal/config"
	"llmgatewaybackend/internal/models"
	"llmgatewaybackend/internal/services"
)

var usageMap = make(map[string]*models.Usage)
var usageMutex = &sync.Mutex{}

// validateRequest reads and parses the request body
func validateRequest(r *http.Request) (*models.RequestBody, int, error) {
	if r.Method != http.MethodPost {
		return nil, http.StatusBadRequest, fmt.Errorf("only POST method allowed")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to read request body: %w", err)
	}
	defer func() {
		if closeError := r.Body.Close(); closeError != nil {
			log.Println("Error closing response body:", closeError)
		}
	}()

	// Parse JSON
	var req models.RequestBody
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err)
	}

	return &req, http.StatusOK, nil
}

// getBearerToken extracts the virtual key from the header.
func getBearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get(config.Authorization)
	if len(auth) > len(config.Bearer) && strings.EqualFold(auth[:len(config.Bearer)], config.Bearer) {
		return strings.TrimSpace(auth[len(config.Bearer):]), true
	}
	return "", false
}

// extractProviderAndKey extracts virtual, api key and provider.
func extractProviderAndKey(r *http.Request, keysFile string) <-chan models.KeyDataVirtualKey {
	resultChan := make(chan models.KeyDataVirtualKey)

	go func() {
		defer close(resultChan)

		// Exception of invalid header.
		authHeader := r.Header.Get(config.Authorization)
		if len(authHeader) <= len(config.Bearer) || !strings.EqualFold(authHeader[:len(config.Bearer)], config.Bearer) {
			return
		}

		// Key extraction.
		virtualKey, _ := getBearerToken(r)
		ch := config.GetKeyDataAsync(keysFile, virtualKey)
		keyData, found := <-ch
		if !found {
			return
		}

		resultChan <- models.KeyDataVirtualKey{
			KeyData:    keyData,
			VirtualKey: virtualKey,
		}

	}()
	return resultChan
}

// extractKeyData wraps the existing extractProviderAndKey call
func extractKeyData(r *http.Request) (models.KeyDataVirtualKey, error) {
	ch := extractProviderAndKey(r, config.KeysJson)
	keyData, ok := <-ch
	if !ok {
		return models.KeyDataVirtualKey{}, fmt.Errorf("Unauthorized: invalid or missing API key")
	}
	return keyData, nil
}

// sendToProvider sends the request body to the LLM provider concurrently
func sendToProvider(r *http.Request, keyData models.KeyData, body []byte) (<-chan *http.Response, <-chan error) {
	respChan := make(chan *http.Response, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		req, err := http.NewRequest("POST", config.ChatProviders[keyData.Provider], bytes.NewBuffer(body))
		if err != nil {
			errChan <- err
			return
		}

		// Keep headers the same, just change the virtual key with api key.
		for name, values := range r.Header {
			if strings.EqualFold(name, config.Authorization) {
				continue
			}
			for _, v := range values {
				req.Header.Add(name, v)
			}
		}

		// Send the correct http request to the LLM provider.
		req.Header.Set(config.Authorization, config.Bearer+keyData.ApiKey)
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			errChan <- err
			return
		}

		respChan <- resp
	}()

	return respChan, errChan
}

// forwardResponse writes the provider response to the user.
func forwardResponse(w http.ResponseWriter, resp *http.Response) ([]byte, error) {
	defer func() {
		if closeError := resp.Body.Close(); closeError != nil {
			log.Println("Error closing response body:", closeError)
		}
	}()
	body, readError := io.ReadAll(resp.Body)
	if readError != nil {
		return nil, readError
	}

	for name, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}

	// Deliver response to user.
	w.WriteHeader(resp.StatusCode)
	if _, writeError := w.Write(body); writeError != nil {
		return nil, fmt.Errorf("failed to write response body to client: %w", writeError)
	}
	return body, nil
}

// ChatCompletion handler for sending user requests to the LLM.
func ChatCompletion(w http.ResponseWriter, r *http.Request) {

	// Validation of the request.
	start := time.Now()
	reqBody, code, err := validateRequest(r)
	if code != http.StatusOK {
		w.WriteHeader(code)
		if err != nil {
			w.Header().Set(config.ContentType, config.ApplicationJson)
			encodeErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			if encodeErr != nil {
				log.Println("Failed to encode JSON error:", encodeErr)
			}
		}
		return
	}

	// Important key extraction to change the virtual key to api key.
	keyDataVirtualKey, err := extractKeyData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Making sure user can send the request and did not exceed limit.
	if canSend, sendErr := services.CanSendMessage(keyDataVirtualKey.VirtualKey, keyDataVirtualKey.KeyData.Provider); canSend == false {
		errorMsg := "Rate limit exceeded"
		if sendErr != nil {
			errorMsg = sendErr.Error()
		}
		http.Error(w, errorMsg, http.StatusTooManyRequests)
		return
	}

	// Save interaction with LLM.
	usageMutex.Lock()
	usage, exists := usageMap[keyDataVirtualKey.VirtualKey]
	if !exists || usage == nil {
		usageMap[keyDataVirtualKey.VirtualKey] = &models.Usage{
			Provider:           keyDataVirtualKey.KeyData.Provider,
			VirtualKey:         keyDataVirtualKey.VirtualKey,
			TotalRequestTimeMs: 0,
			RequestCount:       0,
			TokensUsed:         0,
			LastReset:          time.Now(),
		}
		usage = usageMap[keyDataVirtualKey.VirtualKey]
	}
	usageMutex.Unlock()

	usageLog, err := json.MarshalIndent(usage, "", "  ")
	if err != nil {
		log.Println("Failed to marshal log:", err)
	} else {
		fmt.Println(string(usageLog))
	}

	body, _ := json.Marshal(reqBody)

	// Send request to LLM provider.
	respChan, errChan := sendToProvider(r, keyDataVirtualKey.KeyData, body)

	totalTime := time.Since(start).Milliseconds()

	select {

	case resp := <-respChan:

		// Handling errors in response from provider.
		if resp == nil {
			http.Error(w, "Failed to forward provider response", http.StatusBadGateway)
		}

		respBody, err := forwardResponse(w, resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Saving the interaction in a Json file database (for later metrics).
		if trackErr := services.TrackUsageFile(keyDataVirtualKey.VirtualKey, keyDataVirtualKey.KeyData.Provider, totalTime); trackErr != nil {
			log.Println("Failed to track usage file:", trackErr)
		}

		// Create a log output to the server.
		outputLog := models.Log{
			Timestamp:  start.UTC().Format(config.TimeFormat),
			VirtualKey: keyDataVirtualKey.VirtualKey,
			Provider:   keyDataVirtualKey.KeyData.Provider,
			Method:     r.Method,
			Status:     resp.StatusCode,
			DurationMs: totalTime,
			Request:    body,
			Response:   respBody,
		}

		logJSON, err := json.MarshalIndent(outputLog, "", "  ")
		if err != nil {
			log.Println("Failed to marshal log:", err)
		} else {
			fmt.Println(string(logJSON))
		}

	case err := <-errChan:
		http.Error(w, err.Error(), http.StatusBadGateway)

		// Handling potential errors in provider / server.
	case <-time.After(config.RequestTimeout):
		http.Error(w, "Timeout calling provider", http.StatusGatewayTimeout)
	}
}
