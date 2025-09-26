package config

const (
	PORT      = ":8080"
	KeysJson  = "keys.json"
	UsageFile = "usage.json"

	MaxRequestsPerHour = 100
)

var ChatProviders = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/complete",
	"openai":    "https://api.openai.com/v1/chat/completions",
}

var HealthProviders = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/models",
	"openai":    "https://status.openai.com/api/v2/summary.json",
}
