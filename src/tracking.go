package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

const UsageFile = "usage.json"

func initUsageFile() error {
	emptyUsage := make(map[string]*Usage)
	bytes, err := json.MarshalIndent(emptyUsage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal empty usage map: %w", err)
	}
	if err := os.WriteFile(UsageFile, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write usage file: %w", err)
	}
	return nil
}

func trackUsageFile(virtualKey string) error {
	usageMutex.Lock()
	defer usageMutex.Unlock()

	// Step 1: Read the file
	usageMap := make(map[string]*Usage)
	bytes, err := os.ReadFile(UsageFile)
	if err == nil {
		if err := json.Unmarshal(bytes, &usageMap); err != nil {
			return fmt.Errorf("failed to parse usage file: %w", err)
		}
	}

	now := time.Now().UTC()
	u, exists := usageMap[virtualKey]
	if !exists {
		usageMap[virtualKey] = &Usage{
			VirtualKey:   virtualKey,
			RequestCount: 1,
			LastReset:    now,
		}
	} else {
		// Reset quota if an hour has passed
		if now.Sub(u.LastReset) > time.Hour {
			u.RequestCount = 1
			u.LastReset = now
		} else {
			if u.RequestCount >= MaxRequestsPerHour {
				return fmt.Errorf("quota exceeded: max %d requests per hour", MaxRequestsPerHour)
			}
			u.RequestCount++
		}
	}

	// Step 2: Write back to the file
	updatedBytes, err := json.MarshalIndent(usageMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal usage map: %w", err)
	}
	if err := os.WriteFile(UsageFile, updatedBytes, 0644); err != nil {
		return fmt.Errorf("failed to write usage file: %w", err)
	}
	log.Printf(string(updatedBytes))

	return nil
}
