package config

import "time"

const (
	PORT               = ":8080"
	KeysJson           = "keys.json"
	UsageFile          = "usage.json"
	RequestTimeout     = 20 * time.Second
	MaxRequestsPerHour = 100
	Authorization      = "Authorization"
	TimeFormat         = "2006-01-02T15:04:05.000Z"
	Bearer             = "Bearer "
	ApplicationJson    = "application/json"
	ContentType        = "Content-Type"
)

var ChatProviders = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/complete",
	"openai":    "https://api.openai.com/v1/chat/completions",
}

var HealthProviders = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/models",
	"openai":    "https://status.openai.com/api/v2/summary.json",
}
