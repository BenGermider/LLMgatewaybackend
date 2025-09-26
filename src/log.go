package main

import "encoding/json"

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
