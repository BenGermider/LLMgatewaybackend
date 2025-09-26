package main

import (
	"llmgatewaybackend/internal/config"
	"llmgatewaybackend/internal/handlers"
	"llmgatewaybackend/internal/services"
	"log"
	"net/http"
)

func main() {
	err := services.InitUsageFile()
	if err != nil {
		log.Fatal("Failed to initialize usage file.")
	}
	http.HandleFunc("/chat/completion", handlers.ChatCompletion)
	http.HandleFunc("/health", handlers.HealthCheck)
	http.HandleFunc("/metrics", handlers.MetricsHandler)
	log.Fatal(http.ListenAndServe(config.PORT, nil))
}
