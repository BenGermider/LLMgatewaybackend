package main

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

const (
	RequestTimeout = 20 * time.Second
)

type RequestBody struct {
	Prompt string `json:"prompt"`
}

type Usage struct {
	Provider           string
	VirtualKey         string
	TotalRequestTimeMs int64
	RequestCount       int
	TokensUsed         int
	LastReset          time.Time
}

var usageMap = make(map[string]*Usage)
var usageMutex = &sync.Mutex{}

const MaxRequestsPerHour = 100

// validateRequest reads and parses the request body
func validateRequest(r *http.Request) (*RequestBody, int, error) {
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
	var req RequestBody
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err)
	}

	return &req, http.StatusOK, nil
}

func getBearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):]), true
	}
	return "", false
}

func extractProviderAndKey(r *http.Request, keysFile string) <-chan KeyDataVirtualKey {
	resultChan := make(chan KeyDataVirtualKey)

	go func() {
		defer close(resultChan)

		authHeader := r.Header.Get("Authorization")
		const bearerPrefix = "Bearer "
		if len(authHeader) <= len(bearerPrefix) || !strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
			// Invalid header, just exit
			return
		}

		virtualKey, _ := getBearerToken(r)
		ch := GetKeyDataAsync(keysFile, virtualKey)
		keyData, found := <-ch
		if !found {
			return
		}

		resultChan <- KeyDataVirtualKey{
			KeyData:    keyData,
			VirtualKey: virtualKey,
		}

	}()
	return resultChan
}

// extractKeyData wraps the existing extractProviderAndKey call
func extractKeyData(r *http.Request) (KeyDataVirtualKey, error) {
	ch := extractProviderAndKey(r, KEYS_JSON)
	keyData, ok := <-ch
	if !ok {
		return KeyDataVirtualKey{}, fmt.Errorf("unauthorized: invalid or missing API key")
	}
	return keyData, nil
}

// sendToProvider sends the request body to the LLM provider concurrently
func sendToProvider(r *http.Request, keyData KeyData, body []byte) (<-chan *http.Response, <-chan error) {
	respChan := make(chan *http.Response, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		req, err := http.NewRequest("POST", chatProviders[keyData.Provider], bytes.NewBuffer(body))
		if err != nil {
			errChan <- err
			return
		}

		// Copy headers except Authorization
		for name, values := range r.Header {
			if strings.EqualFold(name, "Authorization") {
				continue
			}
			for _, v := range values {
				req.Header.Add(name, v)
			}
		}

		req.Header.Set("Authorization", "Bearer "+keyData.ApiKey)
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

// forwardResponse writes the provider response to the client
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

	w.WriteHeader(resp.StatusCode)
	if _, writeError := w.Write(body); writeError != nil {
		return nil, fmt.Errorf("failed to write response body to client: %w", writeError)
	}
	return body, nil
}

func chatCompletion(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqBody, code, err := validateRequest(r)
	if code != http.StatusOK {
		w.WriteHeader(code)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			encodeErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			if encodeErr != nil {
				log.Println("Failed to encode JSON error:", encodeErr)
			}
		}
		return
	}

	keyDataVirtualKey, err := extractKeyData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if canSend, sendErr := canSendMessage(keyDataVirtualKey.VirtualKey, keyDataVirtualKey.KeyData.Provider); canSend == false {
		errorMsg := "Rate limit exceeded"
		if sendErr != nil {
			errorMsg = sendErr.Error()
		}
		http.Error(w, errorMsg, http.StatusTooManyRequests)
		return
	}

	// FIX: Add nil check and initialization
	usageMutex.Lock()
	usage, exists := usageMap[keyDataVirtualKey.VirtualKey]
	if !exists || usage == nil {
		usageMap[keyDataVirtualKey.VirtualKey] = &Usage{
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

	respChan, errChan := sendToProvider(r, keyDataVirtualKey.KeyData, body)

	totalTime := time.Since(start).Milliseconds()

	select {

	case resp := <-respChan:

		if resp == nil {
			http.Error(w, "Failed to forward provider response", http.StatusBadGateway)
		}

		respBody, err := forwardResponse(w, resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if trackErr := trackUsageFile(keyDataVirtualKey.VirtualKey, keyDataVirtualKey.KeyData.Provider, totalTime); trackErr != nil {
			log.Println("Failed to track usage file:", trackErr)
		}

		outputLog := Log{
			Timestamp:  start.UTC().Format("2006-01-02T15:04:05.000Z"),
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

	case <-time.After(RequestTimeout):
		http.Error(w, "Timeout calling provider", http.StatusGatewayTimeout)
	}
}
