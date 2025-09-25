package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"net/http"
)

type RequestBody struct {
	Prompt string `json:"prompt"`
}

// ResponseBody represents the JSON response
type ResponseBody struct {
	Reply string `json:"reply"`
}

// validateRequest reads and parses the request body
func validateRequest(r *http.Request) (*RequestBody, int, error) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

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
		if keyData, ok := <-ch; ok {
			resultChan <- keyData
		} else {
			log.Println("Failed to make the request, check your Virtual Key.")
		}
	}()

	return resultChan
}

// chatCompletion sends the request to the desired LLM.
func chatCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract virtual key
	ch := extractProviderAndKey(r, KEYS_JSON)
	keyData, ok := <-ch
	if !ok {
		http.Error(w, "Invalid or missing API key", http.StatusUnauthorized)
		return
	}

	// Read the original request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Concurrently send request to the provider
	respChan := make(chan *http.Response)
	errChan := make(chan error)

	go func() {
		req, err := http.NewRequest("POST", providers[keyData.Provider], bytes.NewBuffer(body))
		if err != nil {
			errChan <- err
			return
		}

		// Copy headers from original request
		for name, values := range r.Header {
			for _, v := range values {
				req.Header.Add(name, v)
			}
		}

		// Replace Authorization with actual API key
		req.Header.Set("Authorization", "Bearer "+keyData.ApiKey)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			errChan <- err
			return
		}
		respChan <- resp
	}()

	// Wait for response or error
	select {
	case resp := <-respChan:
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		// Copy status code and body to client
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)

	case err := <-errChan:
		http.Error(w, err.Error(), http.StatusBadGateway)

	case <-time.After(20 * time.Second):
		http.Error(w, "Timeout calling provider", http.StatusGatewayTimeout)
	}
}
