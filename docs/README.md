# LLM Gateway Backend


Lightweight LLM gateway service routes requests to OpenAI and Anthropic APIs, manages virtual API keys, and logs all interactions.

This application allows you to send an http request to different LLMs (currently supporting anthropic and openAI).

Available methods:

POST /chat/completion: Sends the request to the LLM

* curl https://localhost:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {your_virtual_key}" \
  -d '{
  "model": "gpt-3.5-turbo",
  "messages": [
  {"role": "user", "content": "Hello, how are you?"}
  ]
  }'

GET /health: Ping the LLM to see if available

* curl http://localhost:8080/health?provider={your_provider}

GET /metrics: See metrics that include total requests, requests per provider and average response time.

* curl http://localhost:8080/metrics


### Setup

1) Make sure keys.json includes your virtual-key, provider and api-key (example at the bottom of the page).
2) Type in terminal / cmd:

Windows:
```bash
docker-compose build
```
Linux:
```bash
docker compose build
```

### Run

Type in terminal / cmd:

Windows:
```bash
docker-compose up
```
Linux:
```bash
docker compose up
```

For your convenience, type -d right after "up" to hide logs.


keys.json example:

```json
{
    "virtual_keys": {
        "vk_user1_openai": {
            "provider": "openai",
            "api_key": "sk-real-openai-key-123"
        },
        "vk_user2_anthropic": {
            "provider": "anthropic",
            "api_key": "sk-ant-real-anthropic-key-456"
        }
}
```