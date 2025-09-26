package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func initUsageFile() error {
	emptyUsage := make(map[string]*Usage)
	bytes, err := json.MarshalIndent(emptyUsage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal empty usage map: %w", err)
	}
	if err := os.WriteFile(USAGE_FILE, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write usage file: %w", err)
	}
	return nil
}

func canSendMessage(virtualKey, provider string) (bool, error) {
	usageMutex.Lock()
	defer usageMutex.Unlock()

	// Step 1: Read usage file
	usageMap := make(map[string]*Usage)
	bytes, err := os.ReadFile(USAGE_FILE)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, assume no usage yet
			return true, nil
		}
		return false, fmt.Errorf("failed to read usage file: %w", err)
	}

	if err := json.Unmarshal(bytes, &usageMap); err != nil {
		return false, fmt.Errorf("failed to parse usage file: %w", err)
	}

	// Step 2: Lookup usage
	u, exists := usageMap[virtualKey]
	if !exists || u.Provider != provider {
		// No usage yet for this key/provider
		return true, nil
	}

	// Step 3: Reset if more than an hour has passed
	if time.Since(u.LastReset) > time.Hour {
		return true, nil
	}

	// Step 4: Check if max requests reached
	if u.RequestCount >= MaxRequestsPerHour {
		return false, nil
	}

	return true, nil
}

func trackUsageFile(virtualKey string, provider string, requestTimeMs int64) error {
	usageMutex.Lock()
	defer usageMutex.Unlock()

	// Step 1: Read the usage file
	usageMap := make(map[string]*Usage)
	bytes, err := os.ReadFile(USAGE_FILE)
	if err == nil {
		if err := json.Unmarshal(bytes, &usageMap); err != nil {
			return fmt.Errorf("failed to parse usage file: %w", err)
		}
	}

	now := time.Now().UTC()
	u, exists := usageMap[virtualKey]
	if !exists {
		// Create new usage entry
		usageMap[virtualKey] = &Usage{
			Provider:           provider,
			VirtualKey:         virtualKey,
			RequestCount:       1,
			TotalRequestTimeMs: requestTimeMs,
			LastReset:          now,
		}
	} else {
		// Reset quota if an hour has passed
		if now.Sub(u.LastReset) > time.Hour {
			u.RequestCount = 1
			u.TotalRequestTimeMs = requestTimeMs
			u.LastReset = now
		} else {
			// Increment request count and sum total request time
			u.RequestCount++
			u.TotalRequestTimeMs += requestTimeMs
		}
		// Always update provider in case it changed
		u.Provider = provider
		u.LastReset = now
	}

	// Step 2: Write back to the file
	updatedBytes, err := json.MarshalIndent(usageMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal usage map: %w", err)
	}
	if err := os.WriteFile(USAGE_FILE, updatedBytes, 0644); err != nil {
		return fmt.Errorf("failed to write usage file: %w", err)
	}

	log.Printf("Updated usage: %s\n", string(updatedBytes))
	return nil
}
