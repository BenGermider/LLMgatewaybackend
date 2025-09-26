package services

import (
	"encoding/json"
	"fmt"
	"llmgatewaybackend/internal/config"
	"llmgatewaybackend/internal/models"
	"log"
	"os"
	"sync"
	"time"
)

var (
	usageMap   map[string]*models.Usage
	usageMutex sync.Mutex
)

// InitUsageFile creates a new database for metrics.
func InitUsageFile() error {
	emptyUsage := make(map[string]*models.Usage)
	bytes, err := json.MarshalIndent(emptyUsage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal empty usage map: %w", err)
	}
	if err := os.WriteFile(config.UsageFile, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write usage file: %w", err)
	}

	usageMap = make(map[string]*models.Usage)
	return nil
}

// CanSendMessage makes sure user is not rate limited and can send the request.
func CanSendMessage(virtualKey, provider string) (bool, error) {
	usageMutex.Lock()
	defer usageMutex.Unlock()

	usageMap = make(map[string]*models.Usage)
	bytes, err := os.ReadFile(config.UsageFile)
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

	currentUse, exists := usageMap[virtualKey]
	if !exists || currentUse.Provider != provider {
		// No usage yet for this key/provider
		return true, nil
	}

	if time.Since(currentUse.LastReset) > time.Hour {
		return true, nil
	}

	if currentUse.RequestCount >= config.MaxRequestsPerHour {
		return false, nil
	}

	return true, nil
}

// TrackUsageFile saves interaction data in file.
func TrackUsageFile(virtualKey string, provider string, requestTimeMs int64) error {
	usageMutex.Lock()
	defer usageMutex.Unlock()

	// Read current interaction database.
	usageMap = make(map[string]*models.Usage)
	bytes, err := os.ReadFile(config.UsageFile)
	if err == nil {
		if err := json.Unmarshal(bytes, &usageMap); err != nil {
			return fmt.Errorf("failed to parse usage file: %w", err)
		}
	}

	// Update the database in correct form of data saved.
	now := time.Now().UTC()
	currentUse, exists := usageMap[virtualKey]
	if !exists {
		usageMap[virtualKey] = &models.Usage{
			Provider:           provider,
			VirtualKey:         virtualKey,
			RequestCount:       1,
			TotalRequestTimeMs: requestTimeMs,
			LastReset:          now,
		}
	} else {
		if now.Sub(currentUse.LastReset) > time.Hour {
			currentUse.RequestCount = 1
			currentUse.TotalRequestTimeMs = requestTimeMs
			currentUse.LastReset = now
		} else {
			currentUse.RequestCount++
			currentUse.TotalRequestTimeMs += requestTimeMs
		}
		currentUse.Provider = provider
		currentUse.LastReset = now
	}

	// Update database
	updatedBytes, err := json.MarshalIndent(usageMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal usage map: %w", err)
	}
	if err := os.WriteFile(config.UsageFile, updatedBytes, 0644); err != nil {
		return fmt.Errorf("failed to write usage file: %w", err)
	}

	log.Printf("Updated usage: %s\n", string(updatedBytes))
	return nil
}
