package main

const (
	PORT      = ":8080"
	KEYS_JSON = "keys.json"
)

var chatProviders map[string]string = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/complete",
	"openai":    "https://api.openai.com/v1/chat/completions",
}

var healthProviders map[string]string = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/models",
	"openai":    "https://status.openai.com/api/v2/summary.json",
}
