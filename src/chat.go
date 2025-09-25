package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	RequestTimeout = 20 * time.Second
)

type RequestBody struct {
	Prompt string `json:"prompt"`
}

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

func extractProviderAndKey(r *http.Request, keysFile string) <-chan KeyData {
	resultChan := make(chan KeyData)

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

		resultChan <- keyData

	}()
	return resultChan
}

// extractKeyData wraps the existing extractProviderAndKey call
func extractKeyData(r *http.Request) (KeyData, error) {
	ch := extractProviderAndKey(r, KEYS_JSON)
	keyData, ok := <-ch
	if !ok {
		return KeyData{}, fmt.Errorf("unauthorized: invalid or missing API key")
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

		req, err := http.NewRequest("POST", providers[keyData.Provider], bytes.NewBuffer(body))
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
func forwardResponse(w http.ResponseWriter, resp *http.Response) error {
	defer func() {
		if closeError := resp.Body.Close(); closeError != nil {
			log.Println("Error closing response body:", closeError)
		}
	}()
	body, readError := io.ReadAll(resp.Body)
	if readError != nil {
		return readError
	}

	for name, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	if _, writeError := w.Write(body); writeError != nil {
		return fmt.Errorf("failed to write response body to client: %w", writeError)
	}
	return nil
}

// chatCompletion is now very concise
func chatCompletion(w http.ResponseWriter, r *http.Request) {
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

	keyData, err := extractKeyData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, _ := json.Marshal(reqBody)
	respChan, errChan := sendToProvider(r, keyData, body)

	select {
	case resp := <-respChan:
		if resp == nil || forwardResponse(w, resp) != nil {
			http.Error(w, "Failed to forward provider response", http.StatusBadGateway)
		}

	case err := <-errChan:
		http.Error(w, err.Error(), http.StatusBadGateway)

	case <-time.After(RequestTimeout):
		http.Error(w, "Timeout calling provider", http.StatusGatewayTimeout)
	}
}
