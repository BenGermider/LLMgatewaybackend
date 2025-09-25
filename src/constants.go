package main

const (
	PORT      = ":8080"
	KEYS_JSON = "keys.json"
)

var providers map[string]string = map[string]string{
	"anthropic": "https://api.anthropic.com/v1/complete",
	"openai":    "https://api.openai.com/v1/chat/completions",
}
