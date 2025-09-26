package models

import (
	"encoding/json"
	"time"
)

type Log struct {
	Timestamp  string          `json:"timestamp"`
	VirtualKey string          `json:"virtual_key"`
	Provider   string          `json:"provider"`
	Method     string          `json:"method"`
	Status     int             `json:"status"`
	DurationMs int64           `json:"duration_ms"`
	Request    json.RawMessage `json:"request"`
	Response   json.RawMessage `json:"response"`
}

type KeyData struct {
	Provider string `json:"provider"`
	ApiKey   string `json:"api_key"`
}

type KeyDataVirtualKey struct {
	KeyData    KeyData
	VirtualKey string
}

type KeysFile struct {
	VirtualKeys map[string]KeyData `json:"virtual_keys"`
}

type Metrics struct {
	TotalRequests       int64            `json:"total_requests"`
	RequestsPerProvider map[string]int64 `json:"requests_per_provider"`
	AverageResponseTime float64          `json:"average_response_time_ms"`
}

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

type HealthLog struct {
	Timestamp string `json:"timestamp"`
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Status    int    `json:"status"`
}
