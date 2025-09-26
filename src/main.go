package main

import (
	"log"
	"net/http"
)

func main() {
	err := initUsageFile()
	if err != nil {
		log.Fatal("Failed to initialize usage file.")
	}
	http.HandleFunc("/chat/completion", chatCompletion)
	http.HandleFunc("/health", healthCheck)
	log.Fatal(http.ListenAndServe(PORT, nil))
}
