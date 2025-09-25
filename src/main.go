package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/chat/completion", chatCompletion)
	log.Fatal(http.ListenAndServe(PORT, nil))
}
