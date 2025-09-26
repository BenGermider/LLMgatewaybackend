package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/chat/completion", chatCompletion)
	http.HandleFunc("/health", healthCheck)
	log.Fatal(http.ListenAndServe(PORT, nil))
}
