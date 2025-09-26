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

	u, exists := usageMap[virtualKey]
	if !exists || u.Provider != provider {
		// No usage yet for this key/provider
		return true, nil
	}

	if time.Since(u.LastReset) > time.Hour {
		return true, nil
	}

	if u.RequestCount >= config.MaxRequestsPerHour {
		return false, nil
	}

	return true, nil
}

func TrackUsageFile(virtualKey string, provider string, requestTimeMs int64) error {
	usageMutex.Lock()
	defer usageMutex.Unlock()

	usageMap = make(map[string]*models.Usage)
	bytes, err := os.ReadFile(config.UsageFile)
	if err == nil {
		if err := json.Unmarshal(bytes, &usageMap); err != nil {
			return fmt.Errorf("failed to parse usage file: %w", err)
		}
	}

	now := time.Now().UTC()
	u, exists := usageMap[virtualKey]
	if !exists {
		usageMap[virtualKey] = &models.Usage{
			Provider:           provider,
			VirtualKey:         virtualKey,
			RequestCount:       1,
			TotalRequestTimeMs: requestTimeMs,
			LastReset:          now,
		}
	} else {
		if now.Sub(u.LastReset) > time.Hour {
			u.RequestCount = 1
			u.TotalRequestTimeMs = requestTimeMs
			u.LastReset = now
		} else {
			u.RequestCount++
			u.TotalRequestTimeMs += requestTimeMs
		}
		u.Provider = provider
		u.LastReset = now
	}

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
