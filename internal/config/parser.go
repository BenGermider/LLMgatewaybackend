package config

import (
	"encoding/json"
	"llmgatewaybackend/internal/models"
	"log"
	"os"
)

// Reads a json file, parses it according to the structs above and returns the desired provider and api-key.
// filename - name of file to open
// key - virtual key to get data with

func GetKeyDataAsync(filename string, key string) <-chan models.KeyData {
	resultChan := make(chan models.KeyData)

	go func() {
		defer close(resultChan)

		// Error handling while opening and reading the file.
		bytes, err := os.ReadFile(filename)
		if err != nil {
			log.Println("Error reading file:", err)
			return
		}

		// Parse the json.
		var data models.KeysFile
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			log.Println("Error parsing JSON:", err)
			return
		}

		// Get the data searched.
		if val, ok := data.VirtualKeys[key]; ok {
			resultChan <- val
		} else {
			log.Println("Key not found:", key)
		}
	}()

	return resultChan
}
